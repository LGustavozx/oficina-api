package infrastructure

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/testdb"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/workorder/domain"
)

// scenario cria no banco o que uma ordem de serviço referencia (cliente,
// veículo, serviço e peça). As limpezas rodam na ordem inversa: as ordens de
// serviço criadas depois saem antes dos registros de que dependem.
type scenario struct {
	db                         *sql.DB
	customerID, vehicleID      uuid.UUID
	serviceID, partID, otherID uuid.UUID
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func newScenario(t *testing.T) scenario {
	t.Helper()
	db := testdb.Open(t)
	now := time.Now().UTC()
	s := scenario{db: db, customerID: uuid.New(), vehicleID: uuid.New(), serviceID: uuid.New(), partID: uuid.New(), otherID: uuid.New()}
	tag := uuid.NewString()[:8]

	exec(t, db, `INSERT INTO clientes (id, nome, documento, tipo_documento, criado_em, atualizado_em) VALUES ($1, 'Cliente Teste', $2, 'CPF', $3, $3)`, s.customerID, "OS"+tag, now)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM clientes WHERE id = $1`, s.customerID) })

	exec(t, db, `INSERT INTO veiculos (id, cliente_id, placa, marca, modelo, ano, criado_em, atualizado_em) VALUES ($1, $2, $3, 'Fiat', 'Uno', 2010, $4, $4)`, s.vehicleID, s.customerID, "OS"+tag, now)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM veiculos WHERE id = $1`, s.vehicleID) })

	exec(t, db, `INSERT INTO servicos (id, nome, preco_centavos, duracao_estimada_min, criado_em, atualizado_em) VALUES ($1, $2, 12000, 45, $3, $3)`, s.serviceID, "Serviço "+tag, now)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM servicos WHERE id = $1`, s.serviceID) })

	for i, id := range []uuid.UUID{s.partID, s.otherID} {
		exec(t, db, `INSERT INTO pecas (id, sku, nome, preco_unitario_centavos, criado_em, atualizado_em) VALUES ($1, $2, 'Peça', 3590, $3, $3)`, id, "OS"+tag+string(rune('A'+i)), now)
		t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM pecas WHERE id = $1`, id) })
	}
	return s
}

func (s scenario) items() []domain.Item {
	return []domain.Item{
		{Kind: domain.ItemService, RefID: s.serviceID, Name: "Troca de óleo", UnitPrice: money.FromCents(12000), Quantity: 1},
		{Kind: domain.ItemPart, RefID: s.partID, Name: "Filtro de óleo", UnitPrice: money.FromCents(3590), Quantity: 2},
	}
}

func (s scenario) create(t *testing.T, repo *WorkOrderRepository, items []domain.Item) domain.WorkOrder {
	t.Helper()
	order, err := domain.NewWorkOrder(uuid.New(), s.customerID, s.vehicleID, "Barulho no motor", items, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.Create(context.Background(), order)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _, _ = s.db.Exec(`DELETE FROM ordens_servico WHERE id = $1`, created.ID) })
	return created
}

func wantAppErr(t *testing.T, err error, kind apperr.Kind, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("erro = %v, esperado %s/%s", err, kind, code)
	}
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func sameOrder(a, b domain.WorkOrder) bool {
	if a.ID != b.ID || a.Number != b.Number || a.CustomerID != b.CustomerID || a.VehicleID != b.VehicleID ||
		a.Description != b.Description || a.Status != b.Status || len(a.Items) != len(b.Items) ||
		!a.CreatedAt.Equal(b.CreatedAt) || !a.UpdatedAt.Equal(b.UpdatedAt) ||
		!sameTime(a.StartedAt, b.StartedAt) || !sameTime(a.CompletedAt, b.CompletedAt) || !sameTime(a.DeliveredAt, b.DeliveredAt) {
		return false
	}
	for i := range a.Items {
		if a.Items[i] != b.Items[i] {
			return false
		}
	}
	return true
}

func TestCreateAndFindByID(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)

	created := s.create(t, repo, s.items())

	if created.Number < 1 {
		t.Errorf("o banco deveria gerar o número: %d", created.Number)
	}
	got, err := repo.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !sameOrder(got, created) {
		t.Errorf("lida = %+v\nesperada = %+v", got, created)
	}
	if got.Items[0].Kind != domain.ItemService || got.Items[1].Kind != domain.ItemPart || got.Items[1].Quantity != 2 {
		t.Errorf("itens fora de ordem ou errados: %+v", got.Items)
	}
	if got.Total().Cents() != 19180 {
		t.Errorf("total = %d, esperado 19180", got.Total().Cents())
	}
	if got.StartedAt != nil || got.CompletedAt != nil || got.DeliveredAt != nil {
		t.Error("marcos de execução deveriam estar vazios")
	}
}

func TestNumbersAreSequential(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)

	first := s.create(t, repo, nil)
	second := s.create(t, repo, nil)

	if second.Number <= first.Number {
		t.Errorf("números: %d depois %d", first.Number, second.Number)
	}
}

func TestFindByIDNotFound(t *testing.T) {
	s := newScenario(t)

	_, err := NewWorkOrderRepository(s.db).FindByID(context.Background(), uuid.New())

	wantAppErr(t, err, apperr.KindNotFound, domain.CodeWorkOrderNotFound)
}

func TestSaveWithoutItems(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)
	order := s.create(t, repo, nil)

	got, err := repo.FindByID(context.Background(), order.ID)

	if err != nil || len(got.Items) != 0 {
		t.Errorf("ordem sem itens: %d itens (%v)", len(got.Items), err)
	}
}

func TestSaveStoresStatusMilestonesAndItems(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)
	ctx := context.Background()
	order := s.create(t, repo, s.items())
	at := order.CreatedAt.Add(time.Hour)

	next, err := order.ReplaceItems([]domain.Item{s.items()[1]}, at)
	if err != nil {
		t.Fatal(err)
	}
	next, _ = next.StartDiagnosis(at)
	next, _ = next.SendQuote(at)
	next, err = next.Approve(at)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Save(ctx, next, domain.StatusReceived); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, order.ID)
	if err != nil || !sameOrder(got, next) {
		t.Fatalf("lida = %+v (%v)\nesperada = %+v", got, err, next)
	}
	if got.Status != domain.StatusInExecution || got.StartedAt == nil || len(got.Items) != 1 {
		t.Errorf("status/início/itens errados: %+v", got)
	}
}

func TestSaveRejectsStaleStatusAndChangesNothing(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)
	ctx := context.Background()
	order := s.create(t, repo, s.items())

	changed, _ := order.ReplaceItems(nil, order.CreatedAt.Add(time.Minute))
	changed, _ = changed.StartDiagnosis(order.CreatedAt.Add(time.Minute))

	// O chamador acha que a ordem está em diagnóstico, mas no banco ela é RECEBIDA.
	err := repo.Save(ctx, changed, domain.StatusInDiagnosis)

	wantAppErr(t, err, apperr.KindConflict, domain.CodeWorkOrderChanged)
	got, _ := repo.FindByID(ctx, order.ID)
	if got.Status != domain.StatusReceived || len(got.Items) != 2 {
		t.Errorf("nada deveria ter mudado: status %s, %d itens", got.Status, len(got.Items))
	}
}

func TestSaveUnknownOrderConflicts(t *testing.T) {
	s := newScenario(t)
	ghost, _ := domain.NewWorkOrder(uuid.New(), s.customerID, s.vehicleID, "x", nil, time.Now().UTC())

	err := NewWorkOrderRepository(s.db).Save(context.Background(), ghost, domain.StatusReceived)

	wantAppErr(t, err, apperr.KindConflict, domain.CodeWorkOrderChanged)
}

// Várias aprovações simultâneas da mesma ordem: só uma pode vencer.
func TestConcurrentSavesOnlyOneWins(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)
	ctx := context.Background()
	order := s.create(t, repo, s.items())
	at := order.CreatedAt.Add(time.Minute)

	order, _ = order.StartDiagnosis(at)
	order, _ = order.SendQuote(at)
	if err := repo.Save(ctx, order, domain.StatusReceived); err != nil {
		t.Fatal(err)
	}
	// o Save acima foi de RECEBIDA para AGUARDANDO_APROVACAO de uma vez só (teste de infraestrutura)
	approved, err := order.Approve(at)
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 10
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		wins     int
		conflict int
		others   []error
	)
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := repo.Save(ctx, approved, domain.StatusAwaitingApproval)

			mu.Lock()
			defer mu.Unlock()
			switch appErr, ok := apperr.As(err); {
			case err == nil:
				wins++
			case ok && appErr.Code == domain.CodeWorkOrderChanged:
				conflict++
			default:
				others = append(others, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(others) > 0 {
		t.Fatalf("erros inesperados: %v", others)
	}
	if wins != 1 || conflict != attempts-1 {
		t.Errorf("vitórias = %d, conflitos = %d; esperado 1 e %d", wins, conflict, attempts-1)
	}
}

func TestListFiltersOrdersAndLoadsItems(t *testing.T) {
	s := newScenario(t)
	repo := NewWorkOrderRepository(s.db)
	ctx := context.Background()

	older := s.create(t, repo, s.items())
	newer := s.create(t, repo, []domain.Item{s.items()[1]})
	diagnosed, _ := newer.StartDiagnosis(newer.CreatedAt.Add(time.Minute))
	if err := repo.Save(ctx, diagnosed, domain.StatusReceived); err != nil {
		t.Fatal(err)
	}

	all, err := repo.List(ctx, "", 200, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	positions := map[uuid.UUID]int{}
	for i, o := range all {
		positions[o.ID] = i
	}
	if positions[newer.ID] >= positions[older.ID] {
		t.Errorf("deveria vir da mais recente para a mais antiga: %v", positions)
	}
	if got := all[positions[older.ID]]; len(got.Items) != 2 || got.Items[1].Quantity != 2 {
		t.Errorf("os itens deveriam vir carregados: %+v", got.Items)
	}

	onlyDiagnosis, err := repo.List(ctx, domain.StatusInDiagnosis, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range onlyDiagnosis {
		if o.Status != domain.StatusInDiagnosis {
			t.Errorf("o filtro deixou passar %s", o.Status)
		}
		if o.ID == newer.ID {
			found = true
		}
	}
	if !found {
		t.Error("a ordem em diagnóstico deveria aparecer no filtro")
	}

	page, err := repo.List(ctx, "", 1, 0)
	if err != nil || len(page) != 1 {
		t.Errorf("limit=1 devolveu %d (%v)", len(page), err)
	}
}

func TestListEmptyResultIsEmptySliceNotNil(t *testing.T) {
	s := newScenario(t)

	got, err := NewWorkOrderRepository(s.db).List(context.Background(), domain.StatusDelivered, 1, 1_000_000)

	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("esperado slice vazio, veio %#v (%v)", got, err)
	}
}
