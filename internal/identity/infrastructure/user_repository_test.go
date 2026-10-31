package infrastructure

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/identity/domain"
	"oficina-api/internal/platform/testdb"
	"oficina-api/internal/shared/apperr"
)

func newTestUser(t *testing.T, db *sql.DB) domain.User {
	t.Helper()

	email, err := domain.NewEmail("Teste-" + uuid.NewString() + "@Oficina.com")
	if err != nil {
		t.Fatal(err)
	}
	user := domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM usuarios WHERE id = $1`, user.ID) })
	return user
}

func TestUserRepositoryCreateAndFind(t *testing.T) {
	db := testdb.Open(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	user := newTestUser(t, db)

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byEmail, err := repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	byID, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	for _, got := range []domain.User{byEmail, byID} {
		if got.ID != user.ID || got.Email != user.Email || got.PasswordHash != "hash" || !got.CreatedAt.Equal(user.CreatedAt) {
			t.Errorf("usuário lido = %+v, esperado %+v", got, user)
		}
	}
}

func TestUserRepositoryDuplicateEmail(t *testing.T) {
	db := testdb.Open(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	user := newTestUser(t, db)

	if err := repo.Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	again := user
	again.ID = uuid.New()
	err := repo.Create(ctx, again)

	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind != apperr.KindConflict || appErr.Code != domain.CodeEmailTaken {
		t.Fatalf("erro = %v, esperado conflito %s", err, domain.CodeEmailTaken)
	}
}

func TestUserRepositoryNotFound(t *testing.T) {
	db := testdb.Open(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	email, _ := domain.NewEmail("nao-existe-" + uuid.NewString() + "@oficina.com")

	for name, err := range map[string]error{
		"FindByEmail": func() error { _, err := repo.FindByEmail(ctx, email); return err }(),
		"FindByID":    func() error { _, err := repo.FindByID(ctx, uuid.New()); return err }(),
	} {
		appErr, ok := apperr.As(err)
		if !ok || appErr.Kind != apperr.KindNotFound {
			t.Errorf("%s: erro = %v, esperado not_found", name, err)
		}
	}
}

func TestUserRepositoryCount(t *testing.T) {
	db := testdb.Open(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	before, err := repo.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, newTestUser(t, db)); err != nil {
		t.Fatal(err)
	}

	after, err := repo.Count(ctx)
	if err != nil || after != before+1 {
		t.Fatalf("Count = %d, %v; esperado %d", after, err, before+1)
	}
}
