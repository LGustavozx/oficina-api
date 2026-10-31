// Package infrastructure do contexto inventory: persistência em PostgreSQL.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"oficina-api/internal/inventory/domain"
	"oficina-api/internal/platform/database"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
)

const partColumns = `id, sku, nome, descricao, preco_unitario_centavos, quantidade, criado_em, atualizado_em`

type PartRepository struct {
	db *sql.DB
}

func NewPartRepository(db *sql.DB) *PartRepository {
	return &PartRepository{db: db}
}

// Create grava a peça com o saldo que ela já traz (zero, para peças novas).
func (r *PartRepository) Create(ctx context.Context, p domain.Part) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pecas (id, sku, nome, descricao, preco_unitario_centavos, quantidade, criado_em, atualizado_em)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.ID, p.SKU, p.Name, p.Description, p.UnitPrice.Cents(), p.Stock, p.CreatedAt, p.UpdatedAt)
	return wrapWrite(err, "criar peça")
}

// Update não toca na coluna quantidade: o saldo só muda por movimentação.
func (r *PartRepository) Update(ctx context.Context, p domain.Part) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE pecas
		 SET sku = $2, nome = $3, descricao = $4, preco_unitario_centavos = $5, atualizado_em = $6
		 WHERE id = $1`,
		p.ID, p.SKU, p.Name, p.Description, p.UnitPrice.Cents(), p.UpdatedAt)
	if err != nil {
		return wrapWrite(err, "atualizar peça")
	}
	return database.RequireAffected(result, partNotFound())
}

func (r *PartRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM pecas WHERE id = $1`, id)
	if err != nil {
		return wrapWrite(err, "apagar peça")
	}
	return database.RequireAffected(result, partNotFound())
}

func (r *PartRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Part, error) {
	return scanPart(r.db.QueryRowContext(ctx,
		`SELECT `+partColumns+` FROM pecas WHERE id = $1`, id))
}

func (r *PartRepository) FindBySKU(ctx context.Context, sku string) (domain.Part, error) {
	return scanPart(r.db.QueryRowContext(ctx,
		`SELECT `+partColumns+` FROM pecas WHERE sku = $1`, sku))
}

func (r *PartRepository) List(ctx context.Context, limit, offset int) ([]domain.Part, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+partColumns+` FROM pecas ORDER BY nome, id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("inventory: listar peças: %w", err)
	}
	defer func() { _ = rows.Close() }()

	parts := []domain.Part{}
	for rows.Next() {
		part, err := scanPart(rows)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inventory: listar peças: %w", err)
	}
	return parts, nil
}

// Entrada e saída são um único UPDATE, sem ler o saldo antes. Na saída, a
// condição quantidade >= N faz parte do próprio comando: o Postgres trava a
// linha, e uma segunda baixa simultânea reavalia a condição com o saldo já
// atualizado. Por isso o saldo nunca fica negativo, sem precisar de lock manual.
const (
	stockInSQL = `UPDATE pecas SET quantidade = quantidade + $2, atualizado_em = $3
	              WHERE id = $1 RETURNING ` + partColumns
	stockOutSQL = `UPDATE pecas SET quantidade = quantidade - $2, atualizado_em = $3
	               WHERE id = $1 AND quantidade >= $2 RETURNING ` + partColumns
)

// RecordMovement roda numa transação própria ou participa da que vier no
// contexto (ver database.RunInTx), como na aprovação de uma ordem de serviço.
func (r *PartRepository) RecordMovement(ctx context.Context, m domain.StockMovement) (domain.Part, error) {
	var part domain.Part

	err := database.RunInTx(ctx, r.db, func(ctx context.Context) error {
		conn := database.Conn(ctx, r.db)

		query := stockInSQL
		if m.Type == domain.MovementOut {
			query = stockOutSQL
		}

		var err error
		part, err = scanPart(conn.QueryRowContext(ctx, query, m.PartID, m.Quantity, m.CreatedAt))
		if err != nil {
			return r.explainStockFailure(ctx, conn, m, err)
		}

		_, err = conn.ExecContext(ctx,
			`INSERT INTO movimentacoes_estoque (id, peca_id, tipo, quantidade, motivo, criado_em)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			m.ID, m.PartID, movementTypeToDB(m.Type), m.Quantity, m.Reason, m.CreatedAt)
		if err != nil {
			return fmt.Errorf("inventory: gravar movimentação: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Part{}, err
	}
	return part, nil
}

// explainStockFailure traduz a falha do UPDATE. Numa saída, "nenhuma linha"
// pode ser peça inexistente ou saldo insuficiente; uma consulta decide qual.
func (r *PartRepository) explainStockFailure(ctx context.Context, conn database.DBTX, m domain.StockMovement, err error) error {
	if database.IsCheckViolation(err) {
		return apperr.Conflict(domain.CodeStockLimitExceeded, fmt.Sprintf("o saldo não pode passar de %d unidades", domain.MaxStock))
	}

	if appErr, ok := apperr.As(err); ok && appErr.Code == domain.CodePartNotFound && m.Type == domain.MovementOut {
		var exists bool
		if qerr := conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pecas WHERE id = $1)`, m.PartID).Scan(&exists); qerr != nil {
			return fmt.Errorf("inventory: conferir peça: %w", qerr)
		}
		if exists {
			return apperr.Conflict(domain.CodeInsufficientStock, "estoque insuficiente para a saída")
		}
	}
	return err
}

func (r *PartRepository) ListMovements(ctx context.Context, partID uuid.UUID, limit, offset int) ([]domain.StockMovement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, peca_id, tipo, quantidade, motivo, criado_em
		 FROM movimentacoes_estoque WHERE peca_id = $1
		 ORDER BY criado_em DESC, id LIMIT $2 OFFSET $3`, partID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("inventory: listar movimentações: %w", err)
	}
	defer func() { _ = rows.Close() }()

	movements := []domain.StockMovement{}
	for rows.Next() {
		var (
			m      domain.StockMovement
			dbType string
		)
		if err := rows.Scan(&m.ID, &m.PartID, &dbType, &m.Quantity, &m.Reason, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("inventory: ler movimentação: %w", err)
		}
		m.Type = movementTypeFromDB(dbType)
		m.CreatedAt = m.CreatedAt.UTC()
		movements = append(movements, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inventory: listar movimentações: %w", err)
	}
	return movements, nil
}

// O banco guarda os valores em português; o domínio e a API usam IN/OUT.
func movementTypeToDB(t domain.MovementType) string {
	if t == domain.MovementIn {
		return "ENTRADA"
	}
	return "SAIDA"
}

func movementTypeFromDB(s string) domain.MovementType {
	if s == "ENTRADA" {
		return domain.MovementIn
	}
	return domain.MovementOut
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPart(row rowScanner) (domain.Part, error) {
	var (
		p     domain.Part
		cents int64
	)

	err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &cents, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Part{}, partNotFound()
	}
	if err != nil {
		return domain.Part{}, fmt.Errorf("inventory: ler peça: %w", err)
	}

	p.UnitPrice = money.FromCents(cents)
	p.CreatedAt, p.UpdatedAt = p.CreatedAt.UTC(), p.UpdatedAt.UTC()
	return p, nil
}

func partNotFound() error {
	return apperr.NotFound(domain.CodePartNotFound, "peça não encontrada")
}

func wrapWrite(err error, action string) error {
	switch {
	case err == nil:
		return nil
	case database.IsUniqueViolation(err):
		return apperr.Conflict(domain.CodeSKUTaken, "já existe uma peça com esse código")
	case database.IsForeignKeyViolation(err):
		return apperr.Conflict(domain.CodePartInUse, "a peça possui movimentações de estoque ou está em uso")
	}
	return fmt.Errorf("inventory: %s: %w", action, err)
}
