package application

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/catalog/domain"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

type fakeRepo struct {
	services map[uuid.UUID]domain.Service
	err      error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{services: map[uuid.UUID]domain.Service{}} }

func notFound() error {
	return apperr.NotFound(domain.CodeServiceNotFound, "serviço não encontrado")
}

func (r *fakeRepo) Create(_ context.Context, s domain.Service) error {
	if r.err != nil {
		return r.err
	}
	r.services[s.ID] = s
	return nil
}

func (r *fakeRepo) Update(_ context.Context, s domain.Service) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.services[s.ID]; !ok {
		return notFound()
	}
	r.services[s.ID] = s
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.services[id]; !ok {
		return notFound()
	}
	delete(r.services, id)
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.Service, error) {
	s, ok := r.services[id]
	if !ok {
		return domain.Service{}, notFound()
	}
	return s, nil
}

func (r *fakeRepo) List(_ context.Context, limit, offset int) ([]domain.Service, error) {
	all := make([]domain.Service, 0, len(r.services))
	for _, s := range r.services {
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })

	if offset >= len(all) {
		return nil, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

var now = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

func newCatalog(repo *fakeRepo) *ServiceCatalog {
	return NewServiceCatalog(repo, clock.Fixed{Time: now})
}

func oilChange() domain.ServiceData {
	return domain.ServiceData{Name: "Troca de óleo", PriceCents: 12000, EstimatedMinutes: 45}
}

func TestCreate(t *testing.T) {
	repo := newFakeRepo()

	s, err := newCatalog(repo).Create(context.Background(), oilChange())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if s.ID == uuid.Nil || !s.CreatedAt.Equal(now) || s.Price.Cents() != 12000 {
		t.Errorf("serviço errado: %+v", s)
	}
	if _, ok := repo.services[s.ID]; !ok {
		t.Error("serviço não foi gravado")
	}
}

func TestCreateErrors(t *testing.T) {
	repo := newFakeRepo()
	catalog := newCatalog(repo)

	if _, err := catalog.Create(context.Background(), domain.ServiceData{}); err == nil {
		t.Error("esperado erro de validação")
	}
	if len(repo.services) != 0 {
		t.Error("dados inválidos não devem chegar ao repositório")
	}

	repo.err = apperr.Conflict(domain.CodeServiceNameTaken, "nome já cadastrado")
	_, err := catalog.Create(context.Background(), oilChange())
	if appErr, ok := apperr.As(err); !ok || appErr.Code != domain.CodeServiceNameTaken {
		t.Errorf("erro do repositório não propagado: %v", err)
	}
}

func TestGetAndList(t *testing.T) {
	catalog := newCatalog(newFakeRepo())
	ctx := context.Background()
	created, _ := catalog.Create(ctx, oilChange())
	_, _ = catalog.Create(ctx, domain.ServiceData{Name: "Alinhamento", PriceCents: 8000, EstimatedMinutes: 30})

	if got, err := catalog.Get(ctx, created.ID); err != nil || got.ID != created.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := catalog.Get(ctx, uuid.New()); err == nil {
		t.Error("esperado not found")
	}

	list, err := catalog.List(ctx, 10, 0)
	if err != nil || len(list) != 2 || list[0].Name != "Alinhamento" {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

func TestUpdate(t *testing.T) {
	repo := newFakeRepo()
	catalog := newCatalog(repo)
	ctx := context.Background()
	created, _ := catalog.Create(ctx, oilChange())

	data := oilChange()
	data.PriceCents = 15000
	updated, err := catalog.Update(ctx, created.ID, data)
	if err != nil || updated.Price.Cents() != 15000 || updated.ID != created.ID {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	if repo.services[created.ID].Price.Cents() != 15000 {
		t.Error("alteração não foi gravada")
	}

	if _, err := catalog.Update(ctx, uuid.New(), data); err == nil {
		t.Error("esperado not found")
	}
	if _, err := catalog.Update(ctx, created.ID, domain.ServiceData{}); err == nil {
		t.Error("esperado erro de validação")
	}

	repo.err = errors.New("banco fora do ar")
	if _, err := catalog.Update(ctx, created.ID, data); err == nil {
		t.Error("esperado erro do repositório")
	}
}

func TestDelete(t *testing.T) {
	catalog := newCatalog(newFakeRepo())
	ctx := context.Background()
	created, _ := catalog.Create(ctx, oilChange())

	if err := catalog.Delete(ctx, created.ID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := catalog.Delete(ctx, created.ID); err == nil {
		t.Error("esperado not found ao apagar de novo")
	}
}
