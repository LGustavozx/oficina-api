package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/inventory/domain"
	"oficina-api/internal/platform/testdb"
	"oficina-api/internal/shared/apperr"
)

func uniqueSKU() string { return "T-" + strings.ToUpper(uuid.NewString()[:12]) }

func newPart(t *testing.T, db *sql.DB, name string) domain.Part {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)
	part, err := domain.NewPart(uuid.New(), domain.PartData{
		SKU: uniqueSKU(), Name: name, Description: "Peça de teste", UnitPriceCents: 3590,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	// O histórico precisa sair antes da peça por causa da chave estrangeira.
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM movimentacoes_estoque WHERE peca_id = $1`, part.ID)
		_, _ = db.Exec(`DELETE FROM pecas WHERE id = $1`, part.ID)
	})
	return part
}

func savedPart(t *testing.T, db *sql.DB) domain.Part {
	t.Helper()
	part := newPart(t, db, "Filtro de óleo")
	if err := NewPartRepository(db).Create(context.Background(), part); err != nil {
		t.Fatal(err)
	}
	return part
}

func movement(t *testing.T, partID uuid.UUID, kind string, qty int) domain.StockMovement {
	t.Helper()
	m, err := domain.NewStockMovement(uuid.New(), partID, domain.MovementData{Type: kind, Quantity: qty, Reason: "teste"}, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func samePart(a, b domain.Part) bool {
	return a.ID == b.ID && a.SKU == b.SKU && a.Name == b.Name && a.Description == b.Description &&
		a.UnitPrice == b.UnitPrice && a.Stock == b.Stock &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}

func wantAppErr(t *testing.T, err error, kind apperr.Kind, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("erro = %v, esperado %s/%s", err, kind, code)
	}
}

func TestPartRepositoryCreateAndFind(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := newPart(t, db, "Filtro de óleo")

	if err := repo.Create(ctx, part); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byID, err := repo.FindByID(ctx, part.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	bySKU, err := repo.FindBySKU(ctx, part.SKU)
	if err != nil {
		t.Fatalf("FindBySKU: %v", err)
	}

	for _, got := range []domain.Part{byID, bySKU} {
		if !samePart(got, part) {
			t.Errorf("peça lida = %+v, esperado %+v", got, part)
		}
	}
}

func TestPartRepositoryNotFound(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	ghost := newPart(t, db, "Fantasma")

	_, err := repo.FindByID(ctx, uuid.New())
	wantAppErr(t, err, apperr.KindNotFound, domain.CodePartNotFound)
	_, err = repo.FindBySKU(ctx, ghost.SKU)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodePartNotFound)
	wantAppErr(t, repo.Update(ctx, ghost), apperr.KindNotFound, domain.CodePartNotFound)
	wantAppErr(t, repo.Delete(ctx, ghost.ID), apperr.KindNotFound, domain.CodePartNotFound)
}

func TestPartRepositoryDuplicateSKU(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	first := savedPart(t, db)

	clone := newPart(t, db, "Clone")
	clone.SKU = first.SKU
	wantAppErr(t, repo.Create(ctx, clone), apperr.KindConflict, domain.CodeSKUTaken)

	other := savedPart(t, db)
	other.SKU = first.SKU
	wantAppErr(t, repo.Update(ctx, other), apperr.KindConflict, domain.CodeSKUTaken)
}

func TestPartRepositoryUpdateNeverChangesStock(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := savedPart(t, db)
	if _, err := repo.RecordMovement(ctx, movement(t, part.ID, "IN", 10)); err != nil {
		t.Fatal(err)
	}

	// O objeto em memória diz saldo 999; o banco não pode aceitar isso.
	edited, err := part.Update(domain.PartData{SKU: part.SKU, Name: "Filtro novo", UnitPriceCents: 5000}, part.UpdatedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	edited.Stock = 999
	if err := repo.Update(ctx, edited); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.FindByID(ctx, part.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Filtro novo" || got.UnitPrice.Cents() != 5000 {
		t.Errorf("dados cadastrais não foram atualizados: %+v", got)
	}
	if got.Stock != 10 {
		t.Errorf("saldo = %d, esperado 10: editar a peça não pode mexer no estoque", got.Stock)
	}
}

func TestPartRepositoryDelete(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()

	clean := savedPart(t, db)
	if err := repo.Delete(ctx, clean.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.FindByID(ctx, clean.ID)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodePartNotFound)

	withHistory := savedPart(t, db)
	if _, err := repo.RecordMovement(ctx, movement(t, withHistory.ID, "IN", 1)); err != nil {
		t.Fatal(err)
	}
	wantAppErr(t, repo.Delete(ctx, withHistory.ID), apperr.KindConflict, domain.CodePartInUse)
}

func TestRecordMovementUpdatesBalance(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := savedPart(t, db)

	steps := []struct {
		kind string
		qty  int
		want int
	}{
		{"IN", 10, 10},
		{"OUT", 3, 7},
		{"IN", 5, 12},
		{"OUT", 12, 0},
	}
	for _, s := range steps {
		got, err := repo.RecordMovement(ctx, movement(t, part.ID, s.kind, s.qty))
		if err != nil {
			t.Fatalf("%s %d: %v", s.kind, s.qty, err)
		}
		if got.Stock != s.want {
			t.Fatalf("%s %d: saldo = %d, esperado %d", s.kind, s.qty, got.Stock, s.want)
		}
	}

	stored, err := repo.FindByID(ctx, part.ID)
	if err != nil || stored.Stock != 0 {
		t.Errorf("saldo gravado = %d (%v), esperado 0", stored.Stock, err)
	}
}

func TestRecordMovementRejectsInsufficientStock(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := savedPart(t, db)
	if _, err := repo.RecordMovement(ctx, movement(t, part.ID, "IN", 5)); err != nil {
		t.Fatal(err)
	}

	_, err := repo.RecordMovement(ctx, movement(t, part.ID, "OUT", 6))
	wantAppErr(t, err, apperr.KindConflict, domain.CodeInsufficientStock)

	stored, _ := repo.FindByID(ctx, part.ID)
	if stored.Stock != 5 {
		t.Errorf("saldo = %d, esperado 5: saída recusada não pode alterar o estoque", stored.Stock)
	}
	history, _ := repo.ListMovements(ctx, part.ID, 10, 0)
	if len(history) != 1 {
		t.Errorf("histórico tem %d registros, esperado 1: saída recusada não pode deixar rastro", len(history))
	}
}

func TestRecordMovementUnknownPart(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()

	for _, kind := range []string{"IN", "OUT"} {
		_, err := repo.RecordMovement(ctx, movement(t, uuid.New(), kind, 1))
		wantAppErr(t, err, apperr.KindNotFound, domain.CodePartNotFound)
	}
}

func TestRecordMovementStockLimit(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := savedPart(t, db)

	if _, err := repo.RecordMovement(ctx, movement(t, part.ID, "IN", domain.MaxStock)); err != nil {
		t.Fatalf("entrada até o teto deveria passar: %v", err)
	}
	_, err := repo.RecordMovement(ctx, movement(t, part.ID, "IN", 1))
	wantAppErr(t, err, apperr.KindConflict, domain.CodeStockLimitExceeded)

	stored, _ := repo.FindByID(ctx, part.ID)
	if stored.Stock != domain.MaxStock {
		t.Errorf("saldo = %d, esperado %d", stored.Stock, domain.MaxStock)
	}
}

// O teste que justifica a decisão de design: 20 baixas simultâneas de 1
// unidade sobre um estoque de 10 precisam dar exatamente 10 sucessos e 10
// recusas, com saldo final zero e nunca negativo.
func TestRecordMovementConcurrentOutNeverGoesNegative(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := savedPart(t, db)
	if _, err := repo.RecordMovement(ctx, movement(t, part.ID, "IN", 10)); err != nil {
		t.Fatal(err)
	}

	const attempts = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		rejected  int
		other     []error
	)
	start := make(chan struct{})

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := movement(t, part.ID, "OUT", 1)
			<-start
			_, err := repo.RecordMovement(ctx, m)

			mu.Lock()
			defer mu.Unlock()
			switch appErr, ok := apperr.As(err); {
			case err == nil:
				succeeded++
			case ok && appErr.Code == domain.CodeInsufficientStock:
				rejected++
			default:
				other = append(other, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(other) > 0 {
		t.Fatalf("erros inesperados: %v", other)
	}
	if succeeded != 10 || rejected != 10 {
		t.Errorf("sucessos = %d, recusas = %d; esperado 10 e 10", succeeded, rejected)
	}

	stored, err := repo.FindByID(ctx, part.ID)
	if err != nil || stored.Stock != 0 {
		t.Errorf("saldo final = %d (%v), esperado 0", stored.Stock, err)
	}

	history, err := repo.ListMovements(ctx, part.ID, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 11 { // 1 entrada + 10 saídas que passaram
		t.Errorf("histórico tem %d registros, esperado 11", len(history))
	}
}

func TestListMovementsNewestFirstAndPaginates(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	part := savedPart(t, db)

	base := time.Now().UTC().Truncate(time.Microsecond)
	for i, qty := range []int{1, 2, 3} {
		m := movement(t, part.ID, "IN", qty)
		m.CreatedAt = base.Add(time.Duration(i) * time.Second)
		if _, err := repo.RecordMovement(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repo.ListMovements(ctx, part.ID, 10, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("%d movimentações (%v), esperado 3", len(all), err)
	}
	if all[0].Quantity != 3 || all[2].Quantity != 1 {
		t.Errorf("deveria vir da mais nova para a mais antiga: %d, %d, %d", all[0].Quantity, all[1].Quantity, all[2].Quantity)
	}
	if all[0].Type != domain.MovementIn || all[0].Reason != "teste" || all[0].PartID != part.ID {
		t.Errorf("campos errados: %+v", all[0])
	}

	page, err := repo.ListMovements(ctx, part.ID, 1, 1)
	if err != nil || len(page) != 1 || page[0].Quantity != 2 {
		t.Errorf("página 2 = %+v (%v)", page, err)
	}
}

func TestListPartsOrdersByNameAndPaginates(t *testing.T) {
	db := testdb.Open(t)
	repo := NewPartRepository(db)
	ctx := context.Background()
	prefix := "Zz-" + uuid.NewString()[:8] + "-"

	for _, suffix := range []string{"C", "A", "B"} {
		part := newPart(t, db, prefix+suffix)
		if err := repo.Create(ctx, part); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repo.List(ctx, 200, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var ours []string
	for _, p := range all {
		if strings.HasPrefix(p.Name, prefix) {
			ours = append(ours, strings.TrimPrefix(p.Name, prefix))
		}
	}
	if strings.Join(ours, "") != "ABC" {
		t.Errorf("ordem = %v, esperado A, B, C", ours)
	}

	page, err := repo.List(ctx, 1, 0)
	if err != nil || len(page) != 1 {
		t.Errorf("limit=1 devolveu %d itens (%v)", len(page), err)
	}
}
