package application

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/inventory/domain"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

type fakeRepo struct {
	parts     map[uuid.UUID]domain.Part
	movements []domain.StockMovement
	err       error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{parts: map[uuid.UUID]domain.Part{}} }

func notFound() error { return apperr.NotFound(domain.CodePartNotFound, "peça não encontrada") }

func (r *fakeRepo) Create(_ context.Context, p domain.Part) error {
	if r.err != nil {
		return r.err
	}
	r.parts[p.ID] = p
	return nil
}

func (r *fakeRepo) Update(_ context.Context, p domain.Part) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.parts[p.ID]; !ok {
		return notFound()
	}
	r.parts[p.ID] = p
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.parts[id]; !ok {
		return notFound()
	}
	delete(r.parts, id)
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.Part, error) {
	p, ok := r.parts[id]
	if !ok {
		return domain.Part{}, notFound()
	}
	return p, nil
}

func (r *fakeRepo) FindBySKU(_ context.Context, sku string) (domain.Part, error) {
	for _, p := range r.parts {
		if p.SKU == sku {
			return p, nil
		}
	}
	return domain.Part{}, notFound()
}

func (r *fakeRepo) List(_ context.Context, limit, offset int) ([]domain.Part, error) {
	all := make([]domain.Part, 0, len(r.parts))
	for _, p := range r.parts {
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })

	if offset >= len(all) {
		return nil, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

func (r *fakeRepo) RecordMovement(_ context.Context, m domain.StockMovement) (domain.Part, error) {
	if r.err != nil {
		return domain.Part{}, r.err
	}
	p, ok := r.parts[m.PartID]
	if !ok {
		return domain.Part{}, notFound()
	}
	if m.Type == domain.MovementOut {
		p.Stock -= m.Quantity
	} else {
		p.Stock += m.Quantity
	}
	r.parts[p.ID] = p
	r.movements = append(r.movements, m)
	return p, nil
}

func (r *fakeRepo) ListMovements(_ context.Context, partID uuid.UUID, limit, offset int) ([]domain.StockMovement, error) {
	var out []domain.StockMovement
	for _, m := range r.movements {
		if m.PartID == partID {
			out = append(out, m)
		}
	}
	return out, nil
}

var now = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

func newService(repo *fakeRepo) *InventoryService {
	return NewInventoryService(repo, clock.Fixed{Time: now})
}

func filter() domain.PartData {
	return domain.PartData{SKU: "flt-01", Name: "Filtro de óleo", UnitPriceCents: 3590}
}

func TestCreate(t *testing.T) {
	repo := newFakeRepo()

	p, err := newService(repo).Create(context.Background(), filter())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if p.ID == uuid.Nil || p.SKU != "FLT-01" || p.Stock != 0 || !p.CreatedAt.Equal(now) {
		t.Errorf("peça errada: %+v", p)
	}
	if _, ok := repo.parts[p.ID]; !ok {
		t.Error("peça não foi gravada")
	}
}

func TestCreateErrors(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)

	if _, err := svc.Create(context.Background(), domain.PartData{}); err == nil {
		t.Error("esperado erro de validação")
	}
	if len(repo.parts) != 0 {
		t.Error("dados inválidos não devem chegar ao repositório")
	}

	repo.err = apperr.Conflict(domain.CodeSKUTaken, "código já cadastrado")
	_, err := svc.Create(context.Background(), filter())
	if appErr, ok := apperr.As(err); !ok || appErr.Code != domain.CodeSKUTaken {
		t.Errorf("erro do repositório não propagado: %v", err)
	}
}

func TestGetFindBySKUAndList(t *testing.T) {
	svc := newService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, filter())
	_, _ = svc.Create(ctx, domain.PartData{SKU: "VELA-01", Name: "Vela", UnitPriceCents: 2500})

	if got, err := svc.Get(ctx, created.ID); err != nil || got.ID != created.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := svc.Get(ctx, uuid.New()); err == nil {
		t.Error("esperado not found")
	}

	for _, raw := range []string{"FLT-01", "flt-01", "  Flt-01 "} {
		if got, err := svc.FindBySKU(ctx, raw); err != nil || got.ID != created.ID {
			t.Errorf("FindBySKU(%q) = %+v, %v", raw, got, err)
		}
	}
	if _, err := svc.FindBySKU(ctx, "NAO-EXISTE"); err == nil {
		t.Error("esperado not found")
	}

	list, err := svc.List(ctx, 10, 0)
	if err != nil || len(list) != 2 || list[0].Name != "Filtro de óleo" {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

func TestUpdateKeepsStock(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	created, _ := svc.Create(ctx, filter())
	_, _, _ = svc.RecordMovement(ctx, created.ID, domain.MovementData{Type: "IN", Quantity: 7, Reason: "compra"})

	data := filter()
	data.Name = "Filtro novo"
	updated, err := svc.Update(ctx, created.ID, data)
	if err != nil || updated.Name != "Filtro novo" || updated.Stock != 7 {
		t.Fatalf("Update = %+v, %v", updated, err)
	}

	if _, err := svc.Update(ctx, uuid.New(), data); err == nil {
		t.Error("esperado not found")
	}
	if _, err := svc.Update(ctx, created.ID, domain.PartData{}); err == nil {
		t.Error("esperado erro de validação")
	}

	repo.err = errors.New("banco fora do ar")
	if _, err := svc.Update(ctx, created.ID, data); err == nil {
		t.Error("esperado erro do repositório")
	}
}

func TestDelete(t *testing.T) {
	svc := newService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, filter())

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := svc.Delete(ctx, created.ID); err == nil {
		t.Error("esperado not found ao apagar de novo")
	}
}

func TestRecordMovement(t *testing.T) {
	svc := newService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, filter())

	part, movement, err := svc.RecordMovement(ctx, created.ID, domain.MovementData{Type: "IN", Quantity: 10, Reason: "  Compra  "})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if part.Stock != 10 || movement.PartID != created.ID || movement.Reason != "Compra" || !movement.CreatedAt.Equal(now) || movement.ID == uuid.Nil {
		t.Errorf("resultado errado: %+v / %+v", part, movement)
	}

	part, _, err = svc.RecordMovement(ctx, created.ID, domain.MovementData{Type: "OUT", Quantity: 4, Reason: "Uso"})
	if err != nil || part.Stock != 6 {
		t.Errorf("saldo = %d (%v), esperado 6", part.Stock, err)
	}
}

func TestRecordMovementErrors(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	created, _ := svc.Create(ctx, filter())

	if _, _, err := svc.RecordMovement(ctx, created.ID, domain.MovementData{Type: "X", Quantity: 1, Reason: "x"}); err == nil {
		t.Error("esperado erro de validação")
	}
	if len(repo.movements) != 0 {
		t.Error("movimentação inválida não deve chegar ao repositório")
	}

	if _, _, err := svc.RecordMovement(ctx, uuid.New(), domain.MovementData{Type: "IN", Quantity: 1, Reason: "x"}); err == nil {
		t.Error("esperado not found")
	}

	repo.err = apperr.Conflict(domain.CodeInsufficientStock, "estoque insuficiente")
	_, _, err := svc.RecordMovement(ctx, created.ID, domain.MovementData{Type: "OUT", Quantity: 1, Reason: "x"})
	if appErr, ok := apperr.As(err); !ok || appErr.Code != domain.CodeInsufficientStock {
		t.Errorf("erro do repositório não propagado: %v", err)
	}
}

func TestListMovements(t *testing.T) {
	svc := newService(newFakeRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, filter())
	_, _, _ = svc.RecordMovement(ctx, created.ID, domain.MovementData{Type: "IN", Quantity: 3, Reason: "compra"})

	got, err := svc.ListMovements(ctx, created.ID, 10, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListMovements = %+v, %v", got, err)
	}

	_, err = svc.ListMovements(ctx, uuid.New(), 10, 0)
	if appErr, ok := apperr.As(err); !ok || appErr.Kind != apperr.KindNotFound {
		t.Errorf("peça inexistente deveria dar not found, não lista vazia: %v", err)
	}
}
