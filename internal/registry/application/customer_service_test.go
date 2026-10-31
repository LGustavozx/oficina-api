package application

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

type fakeCustomerRepo struct {
	customers map[uuid.UUID]domain.Customer
	err       error
}

func newFakeRepo() *fakeCustomerRepo {
	return &fakeCustomerRepo{customers: map[uuid.UUID]domain.Customer{}}
}

func notFound() error {
	return apperr.NotFound(domain.CodeCustomerNotFound, "cliente não encontrado")
}

func (r *fakeCustomerRepo) Create(_ context.Context, c domain.Customer) error {
	if r.err != nil {
		return r.err
	}
	r.customers[c.ID] = c
	return nil
}

func (r *fakeCustomerRepo) Update(_ context.Context, c domain.Customer) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.customers[c.ID]; !ok {
		return notFound()
	}
	r.customers[c.ID] = c
	return nil
}

func (r *fakeCustomerRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.customers[id]; !ok {
		return notFound()
	}
	delete(r.customers, id)
	return nil
}

func (r *fakeCustomerRepo) FindByID(_ context.Context, id uuid.UUID) (domain.Customer, error) {
	c, ok := r.customers[id]
	if !ok {
		return domain.Customer{}, notFound()
	}
	return c, nil
}

func (r *fakeCustomerRepo) FindByDocument(_ context.Context, d domain.Document) (domain.Customer, error) {
	for _, c := range r.customers {
		if c.Document == d {
			return c, nil
		}
	}
	return domain.Customer{}, notFound()
}

func (r *fakeCustomerRepo) List(_ context.Context, limit, offset int) ([]domain.Customer, error) {
	all := make([]domain.Customer, 0, len(r.customers))
	for _, c := range r.customers {
		all = append(all, c)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })

	if offset >= len(all) {
		return nil, nil
	}
	end := min(offset+limit, len(all))
	return all[offset:end], nil
}

var now = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

func newService(repo *fakeCustomerRepo) *CustomerService {
	return NewCustomerService(repo, clock.Fixed{Time: now})
}

func maria() domain.CustomerData {
	return domain.CustomerData{Name: "Maria Souza", Document: "529.982.247-25"}
}

func TestCreate(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)

	c, err := svc.Create(context.Background(), maria())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if c.ID == uuid.Nil || !c.CreatedAt.Equal(now) {
		t.Errorf("cliente sem ID ou data: %+v", c)
	}
	if _, ok := repo.customers[c.ID]; !ok {
		t.Error("cliente não foi gravado")
	}
}

func TestCreateValidationAndRepoErrors(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)

	if _, err := svc.Create(context.Background(), domain.CustomerData{Name: "x"}); err == nil {
		t.Error("esperado erro de validação")
	}
	if len(repo.customers) != 0 {
		t.Error("dados inválidos não devem chegar ao repositório")
	}

	repo.err = apperr.Conflict(domain.CodeDocumentTaken, "documento já cadastrado")
	_, err := svc.Create(context.Background(), maria())
	if appErr, ok := apperr.As(err); !ok || appErr.Code != domain.CodeDocumentTaken {
		t.Errorf("erro do repositório não propagado: %v", err)
	}
}

func TestGetAndFindByDocument(t *testing.T) {
	svc := newService(newFakeRepo())
	created, _ := svc.Create(context.Background(), maria())

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := svc.Get(context.Background(), uuid.New()); err == nil {
		t.Error("esperado not found")
	}

	byDoc, err := svc.FindByDocument(context.Background(), "529.982.247-25")
	if err != nil || byDoc.ID != created.ID {
		t.Fatalf("FindByDocument = %+v, %v", byDoc, err)
	}
	if _, err := svc.FindByDocument(context.Background(), "123"); err == nil {
		t.Error("documento inválido deveria falhar na validação")
	}
	if _, err := svc.FindByDocument(context.Background(), "11.222.333/0001-81"); err == nil {
		t.Error("esperado not found para documento não cadastrado")
	}
}

func TestList(t *testing.T) {
	svc := newService(newFakeRepo())
	ctx := context.Background()
	_, _ = svc.Create(ctx, domain.CustomerData{Name: "Bruno", Document: "529.982.247-25"})
	_, _ = svc.Create(ctx, domain.CustomerData{Name: "Ana", Document: "11.222.333/0001-81"})

	got, err := svc.List(ctx, 10, 0)
	if err != nil || len(got) != 2 || got[0].Name != "Ana" {
		t.Fatalf("List = %+v, %v", got, err)
	}
}

func TestUpdate(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	created, _ := svc.Create(ctx, maria())

	updated, err := svc.Update(ctx, created.ID, domain.CustomerData{Name: "Maria Silva", Document: "529.982.247-25"})
	if err != nil || updated.Name != "Maria Silva" || updated.ID != created.ID {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	if repo.customers[created.ID].Name != "Maria Silva" {
		t.Error("alteração não foi gravada")
	}

	if _, err := svc.Update(ctx, uuid.New(), maria()); err == nil {
		t.Error("esperado not found")
	}
	if _, err := svc.Update(ctx, created.ID, domain.CustomerData{Name: "x"}); err == nil {
		t.Error("esperado erro de validação")
	}

	repo.err = errors.New("banco fora do ar")
	if _, err := svc.Update(ctx, created.ID, maria()); err == nil {
		t.Error("esperado erro do repositório")
	}
}

func TestDelete(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	created, _ := svc.Create(ctx, maria())

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := svc.Delete(ctx, created.ID); err == nil {
		t.Error("esperado not found ao apagar de novo")
	}
}
