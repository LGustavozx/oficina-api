package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/testdb"
	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

func newCustomer(t *testing.T, db *sql.DB, name string) domain.Customer {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)
	customer, err := domain.NewCustomer(uuid.New(), domain.CustomerData{
		Name:     name,
		Document: testdb.RandomCPF(),
		Email:    "cliente@oficina.com",
		Phone:    "11987654321",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM clientes WHERE id = $1`, customer.ID) })
	return customer
}

func wantAppErr(t *testing.T, err error, kind apperr.Kind, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("erro = %v, esperado %s/%s", err, kind, code)
	}
}

func TestCustomerRepositoryCreateAndFind(t *testing.T) {
	db := testdb.Open(t)
	repo := NewCustomerRepository(db)
	ctx := context.Background()
	customer := newCustomer(t, db, "Maria Souza")

	if err := repo.Create(ctx, customer); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byID, err := repo.FindByID(ctx, customer.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	byDocument, err := repo.FindByDocument(ctx, customer.Document)
	if err != nil {
		t.Fatalf("FindByDocument: %v", err)
	}

	for _, got := range []domain.Customer{byID, byDocument} {
		if !sameCustomer(got, customer) {
			t.Errorf("cliente lido = %+v, esperado %+v", got, customer)
		}
	}
}

func TestCustomerRepositoryNotFound(t *testing.T) {
	db := testdb.Open(t)
	repo := NewCustomerRepository(db)
	ctx := context.Background()
	other := newCustomer(t, db, "Nunca Gravado")

	_, err := repo.FindByID(ctx, uuid.New())
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeCustomerNotFound)

	_, err = repo.FindByDocument(ctx, other.Document)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeCustomerNotFound)

	wantAppErr(t, repo.Update(ctx, other), apperr.KindNotFound, domain.CodeCustomerNotFound)
	wantAppErr(t, repo.Delete(ctx, other.ID), apperr.KindNotFound, domain.CodeCustomerNotFound)
}

func TestCustomerRepositoryUpdate(t *testing.T) {
	db := testdb.Open(t)
	repo := NewCustomerRepository(db)
	ctx := context.Background()
	customer := newCustomer(t, db, "Maria Souza")
	if err := repo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}

	updated, err := customer.Update(domain.CustomerData{Name: "Maria Silva", Document: customer.Document.String()}, customer.UpdatedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.FindByID(ctx, customer.ID)
	if err != nil || !sameCustomer(got, updated) {
		t.Fatalf("cliente lido = %+v (%v), esperado %+v", got, err, updated)
	}
}

func TestCustomerRepositoryDuplicateDocument(t *testing.T) {
	db := testdb.Open(t)
	repo := NewCustomerRepository(db)
	ctx := context.Background()
	first := newCustomer(t, db, "Primeiro")
	second := newCustomer(t, db, "Segundo")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}

	wantAppErr(t, repo.Create(ctx, domain.Customer{
		ID: uuid.New(), Name: "Clone", Document: first.Document, CreatedAt: first.CreatedAt, UpdatedAt: first.UpdatedAt,
	}), apperr.KindConflict, domain.CodeDocumentTaken)

	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	moved, _ := second.Update(domain.CustomerData{Name: "Segundo", Document: first.Document.String()}, second.UpdatedAt)
	wantAppErr(t, repo.Update(ctx, moved), apperr.KindConflict, domain.CodeDocumentTaken)
}

func TestCustomerRepositoryDelete(t *testing.T) {
	db := testdb.Open(t)
	repo := NewCustomerRepository(db)
	ctx := context.Background()
	customer := newCustomer(t, db, "Maria Souza")
	if err := repo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, customer.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.FindByID(ctx, customer.ID)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeCustomerNotFound)
}

func TestCustomerRepositoryListOrdersByNameAndPaginates(t *testing.T) {
	db := testdb.Open(t)
	repo := NewCustomerRepository(db)
	ctx := context.Background()
	prefix := "Zz-" + uuid.NewString()[:8] + "-"

	for _, suffix := range []string{"C", "A", "B"} {
		if err := repo.Create(ctx, newCustomer(t, db, prefix+suffix)); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repo.List(ctx, 200, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var ours []string
	for _, c := range all {
		if strings.HasPrefix(c.Name, prefix) {
			ours = append(ours, strings.TrimPrefix(c.Name, prefix))
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

func sameCustomer(a, b domain.Customer) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Document == b.Document &&
		a.Email == b.Email && a.Phone == b.Phone &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}
