// Package infrastructure do contexto workorder: persistência em PostgreSQL.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/database"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/workorder/domain"
)

const orderColumns = `id, numero, cliente_id, veiculo_id, descricao, status, criado_em, atualizado_em, iniciada_em, finalizada_em, entregue_em`

type WorkOrderRepository struct {
	db *sql.DB
}

func NewWorkOrderRepository(db *sql.DB) *WorkOrderRepository {
	return &WorkOrderRepository{db: db}
}

func (r *WorkOrderRepository) Create(ctx context.Context, o domain.WorkOrder) (domain.WorkOrder, error) {
	err := database.RunInTx(ctx, r.db, func(ctx context.Context) error {
		conn := database.Conn(ctx, r.db)

		err := conn.QueryRowContext(ctx,
			`INSERT INTO ordens_servico (id, cliente_id, veiculo_id, descricao, status, criado_em, atualizado_em, iniciada_em, finalizada_em, entregue_em)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING numero`,
			o.ID, o.CustomerID, o.VehicleID, o.Description, statusToDB[o.Status], o.CreatedAt, o.UpdatedAt,
			nullable(o.StartedAt), nullable(o.CompletedAt), nullable(o.DeliveredAt)).Scan(&o.Number)
		if err != nil {
			return fmt.Errorf("workorder: criar ordem de serviço: %w", err)
		}
		return insertItems(ctx, conn, o)
	})
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return o, nil
}

// Save só grava se o status ainda for o esperado; o UPDATE condicional é o que
// impede duas operações simultâneas de sobrescreverem uma à outra.
func (r *WorkOrderRepository) Save(ctx context.Context, o domain.WorkOrder, expected domain.Status) error {
	return database.RunInTx(ctx, r.db, func(ctx context.Context) error {
		conn := database.Conn(ctx, r.db)

		result, err := conn.ExecContext(ctx,
			`UPDATE ordens_servico
			 SET status = $2, atualizado_em = $3, iniciada_em = $4, finalizada_em = $5, entregue_em = $6
			 WHERE id = $1 AND status = $7`,
			o.ID, statusToDB[o.Status], o.UpdatedAt, nullable(o.StartedAt), nullable(o.CompletedAt), nullable(o.DeliveredAt), statusToDB[expected])
		if err != nil {
			return fmt.Errorf("workorder: atualizar ordem de serviço: %w", err)
		}

		changed := apperr.Conflict(domain.CodeWorkOrderChanged, "a ordem de serviço foi alterada por outra operação; tente novamente")
		if err := database.RequireAffected(result, changed); err != nil {
			return err
		}

		if _, err := conn.ExecContext(ctx, `DELETE FROM itens_ordem_servico WHERE ordem_id = $1`, o.ID); err != nil {
			return fmt.Errorf("workorder: limpar itens: %w", err)
		}
		return insertItems(ctx, conn, o)
	})
}

func insertItems(ctx context.Context, conn database.DBTX, o domain.WorkOrder) error {
	for position, item := range o.Items {
		var serviceID, partID any
		if item.Kind == domain.ItemService {
			serviceID = item.RefID
		} else {
			partID = item.RefID
		}

		_, err := conn.ExecContext(ctx,
			`INSERT INTO itens_ordem_servico (ordem_id, posicao, tipo, servico_id, peca_id, nome, preco_unitario_centavos, quantidade)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			o.ID, position, kindToDB[item.Kind], serviceID, partID, item.Name, item.UnitPrice.Cents(), item.Quantity)
		if err != nil {
			return fmt.Errorf("workorder: gravar item: %w", err)
		}
	}
	return nil
}

func (r *WorkOrderRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	order, err := scanOrder(r.db.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM ordens_servico WHERE id = $1`, id))
	if err != nil {
		return domain.WorkOrder{}, err
	}

	itemsByOrder, err := r.loadItems(ctx, []uuid.UUID{id})
	if err != nil {
		return domain.WorkOrder{}, err
	}
	order.Items = itemsByOrder[id]
	return order, nil
}

// List devolve da mais recente para a mais antiga. Os itens de todas as
// ordens da página vêm numa única consulta, para não consultar uma a uma.
func (r *WorkOrderRepository) List(ctx context.Context, status domain.Status, limit, offset int) ([]domain.WorkOrder, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+orderColumns+` FROM ordens_servico
		 WHERE $1 = '' OR status = $1
		 ORDER BY numero DESC LIMIT $2 OFFSET $3`,
		statusToDB[status], limit, offset)
	if err != nil {
		return nil, fmt.Errorf("workorder: listar ordens de serviço: %w", err)
	}
	defer func() { _ = rows.Close() }()

	orders := []domain.WorkOrder{}
	ids := []uuid.UUID{}
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
		ids = append(ids, order.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workorder: listar ordens de serviço: %w", err)
	}
	if len(orders) == 0 {
		return orders, nil
	}

	itemsByOrder, err := r.loadItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		orders[i].Items = itemsByOrder[orders[i].ID]
	}
	return orders, nil
}

func (r *WorkOrderRepository) loadItems(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]domain.Item, error) {
	text := make([]string, len(orderIDs))
	for i, id := range orderIDs {
		text[i] = id.String()
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT ordem_id, tipo, COALESCE(servico_id, peca_id), nome, preco_unitario_centavos, quantidade
		 FROM itens_ordem_servico WHERE ordem_id = ANY($1::uuid[]) ORDER BY ordem_id, posicao`, text)
	if err != nil {
		return nil, fmt.Errorf("workorder: carregar itens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := map[uuid.UUID][]domain.Item{}
	for rows.Next() {
		var (
			orderID uuid.UUID
			kind    string
			cents   int64
			item    domain.Item
		)
		if err := rows.Scan(&orderID, &kind, &item.RefID, &item.Name, &cents, &item.Quantity); err != nil {
			return nil, fmt.Errorf("workorder: ler item: %w", err)
		}
		item.Kind, item.UnitPrice = kindFromDB[kind], money.FromCents(cents)
		result[orderID] = append(result[orderID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workorder: carregar itens: %w", err)
	}
	return result, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrder(row rowScanner) (domain.WorkOrder, error) {
	var (
		o                        domain.WorkOrder
		status                   string
		started, completed, done sql.NullTime
	)

	err := row.Scan(&o.ID, &o.Number, &o.CustomerID, &o.VehicleID, &o.Description, &status,
		&o.CreatedAt, &o.UpdatedAt, &started, &completed, &done)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkOrder{}, apperr.NotFound(domain.CodeWorkOrderNotFound, "ordem de serviço não encontrada")
	}
	if err != nil {
		return domain.WorkOrder{}, fmt.Errorf("workorder: ler ordem de serviço: %w", err)
	}

	o.Status = statusFromDB[status]
	o.CreatedAt, o.UpdatedAt = o.CreatedAt.UTC(), o.UpdatedAt.UTC()
	o.StartedAt, o.CompletedAt, o.DeliveredAt = utcPtr(started), utcPtr(completed), utcPtr(done)
	return o, nil
}

func nullable(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func utcPtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	utc := t.Time.UTC()
	return &utc
}

// O banco guarda os valores em português; o domínio e a API usam o código em inglês.
var statusToDB = map[domain.Status]string{
	"":                            "",
	domain.StatusReceived:         "RECEBIDA",
	domain.StatusInDiagnosis:      "EM_DIAGNOSTICO",
	domain.StatusAwaitingApproval: "AGUARDANDO_APROVACAO",
	domain.StatusInExecution:      "EM_EXECUCAO",
	domain.StatusCompleted:        "FINALIZADA",
	domain.StatusDelivered:        "ENTREGUE",
	domain.StatusCancelled:        "CANCELADA",
}

var statusFromDB = invert(statusToDB)

var kindToDB = map[domain.ItemKind]string{
	domain.ItemService: "SERVICO",
	domain.ItemPart:    "PECA",
}

var kindFromDB = invert(kindToDB)

func invert[K, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}
