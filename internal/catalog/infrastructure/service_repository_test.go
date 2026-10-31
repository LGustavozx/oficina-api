package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/catalog/domain"
	"oficina-api/internal/platform/testdb"
	"oficina-api/internal/shared/apperr"
)

func newService(t *testing.T, db *sql.DB, name string) domain.Service {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)
	service, err := domain.NewService(uuid.New(), domain.ServiceData{
		Name: name, Description: "Descrição de teste", PriceCents: 12050, EstimatedMinutes: 45,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM servicos WHERE id = $1`, service.ID) })
	return service
}

func uniqueName(label string) string { return label + " " + uuid.NewString()[:8] }

func sameService(a, b domain.Service) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Description == b.Description && a.Price == b.Price &&
		a.EstimatedMinutes == b.EstimatedMinutes && a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}

func wantAppErr(t *testing.T, err error, kind apperr.Kind, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("erro = %v, esperado %s/%s", err, kind, code)
	}
}

func TestServiceRepositoryCreateAndFind(t *testing.T) {
	db := testdb.Open(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()
	service := newService(t, db, uniqueName("Troca de óleo"))

	if err := repo.Create(ctx, service); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByID(ctx, service.ID)
	if err != nil || !sameService(got, service) {
		t.Fatalf("serviço lido = %+v (%v), esperado %+v", got, err, service)
	}
	if got.Price.Cents() != 12050 {
		t.Errorf("preço = %d centavos", got.Price.Cents())
	}
}

func TestServiceRepositoryNotFound(t *testing.T) {
	db := testdb.Open(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()
	ghost := newService(t, db, uniqueName("Fantasma"))

	_, err := repo.FindByID(ctx, uuid.New())
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeServiceNotFound)
	wantAppErr(t, repo.Update(ctx, ghost), apperr.KindNotFound, domain.CodeServiceNotFound)
	wantAppErr(t, repo.Delete(ctx, ghost.ID), apperr.KindNotFound, domain.CodeServiceNotFound)
}

func TestServiceRepositoryDuplicateNameIgnoresCase(t *testing.T) {
	db := testdb.Open(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()
	name := uniqueName("Alinhamento")
	first := newService(t, db, name)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}

	wantAppErr(t, repo.Create(ctx, newService(t, db, strings.ToUpper(name))), apperr.KindConflict, domain.CodeServiceNameTaken)

	other := newService(t, db, uniqueName("Balanceamento"))
	if err := repo.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	renamed, _ := other.Update(domain.ServiceData{Name: name, PriceCents: 100, EstimatedMinutes: 10}, other.UpdatedAt)
	wantAppErr(t, repo.Update(ctx, renamed), apperr.KindConflict, domain.CodeServiceNameTaken)
}

func TestServiceRepositoryUpdateAndDelete(t *testing.T) {
	db := testdb.Open(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()
	service := newService(t, db, uniqueName("Revisão"))
	if err := repo.Create(ctx, service); err != nil {
		t.Fatal(err)
	}

	updated, err := service.Update(domain.ServiceData{Name: service.Name, PriceCents: 99900, EstimatedMinutes: 120}, service.UpdatedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.FindByID(ctx, service.ID)
	if err != nil || !sameService(got, updated) {
		t.Fatalf("serviço lido = %+v (%v), esperado %+v", got, err, updated)
	}

	if err := repo.Delete(ctx, service.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.FindByID(ctx, service.ID)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeServiceNotFound)
}

func TestServiceRepositoryListOrdersByNameAndPaginates(t *testing.T) {
	db := testdb.Open(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()
	prefix := "Zz-" + uuid.NewString()[:8] + "-"

	for _, suffix := range []string{"C", "A", "B"} {
		if err := repo.Create(ctx, newService(t, db, prefix+suffix)); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repo.List(ctx, 200, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var ours []string
	for _, s := range all {
		if strings.HasPrefix(s.Name, prefix) {
			ours = append(ours, strings.TrimPrefix(s.Name, prefix))
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
