package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
	"github.com/LGustavozx/oficina-api/internal/shared/clock"
)

var testTime = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func newEnsureAdmin(repo *fakeRepo, hasher *fakeHasher) *EnsureAdmin {
	return NewEnsureAdmin(repo, hasher, &clock.Fixed{Instant: testTime}, func() string { return "id-fixo" })
}

func TestEnsureAdmin_CreatesWhenMissing(t *testing.T) {
	repo := newFakeRepo()
	created, err := newEnsureAdmin(repo, &fakeHasher{}).Execute(context.Background(), " Admin@Oficina.Local ", "segredo123")
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	u := repo.users["admin@oficina.local"]
	if u == nil || u.ID != "id-fixo" || u.PasswordHash != "hash:segredo123" || !u.CreatedAt.Equal(testTime) {
		t.Errorf("usuário inesperado: %+v", u)
	}
}

func TestEnsureAdmin_IsIdempotentAndKeepsPassword(t *testing.T) {
	repo := newFakeRepo()
	seededUser(t, repo, true)
	created, err := newEnsureAdmin(repo, &fakeHasher{}).Execute(context.Background(), "admin@oficina.local", "outra-senha-123")
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if repo.users["admin@oficina.local"].PasswordHash != "hash:segredo123" {
		t.Error("a senha existente não pode ser alterada pelo seed")
	}
	if repo.saved != 0 {
		t.Error("não deveria salvar novamente")
	}
}

func TestEnsureAdmin_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("e-mail inválido", func(t *testing.T) {
		_, err := newEnsureAdmin(newFakeRepo(), &fakeHasher{}).Execute(ctx, "x", "segredo123")
		if apperr.CodeOf(err) != "EMAIL_INVALIDO" {
			t.Errorf("código = %q", apperr.CodeOf(err))
		}
	})
	t.Run("senha fraca", func(t *testing.T) {
		repo := newFakeRepo()
		_, err := newEnsureAdmin(repo, &fakeHasher{}).Execute(ctx, "a@b.com", "curta")
		if apperr.CodeOf(err) != "SENHA_INVALIDA" || repo.saved != 0 {
			t.Errorf("código = %q, salvos = %d", apperr.CodeOf(err), repo.saved)
		}
	})
	t.Run("falha ao buscar", func(t *testing.T) {
		repo := newFakeRepo()
		repo.findErr = errors.New("banco fora")
		if _, err := newEnsureAdmin(repo, &fakeHasher{}).Execute(ctx, "a@b.com", "segredo123"); err == nil {
			t.Error("esperava erro")
		}
	})
	t.Run("falha no hash", func(t *testing.T) {
		if _, err := newEnsureAdmin(newFakeRepo(), &fakeHasher{hashErr: errors.New("boom")}).Execute(ctx, "a@b.com", "segredo123"); err == nil {
			t.Error("esperava erro")
		}
	})
	t.Run("falha ao salvar", func(t *testing.T) {
		repo := newFakeRepo()
		repo.saveErr = apperr.Conflict("EMAIL_JA_CADASTRADO", "e-mail já cadastrado")
		_, err := newEnsureAdmin(repo, &fakeHasher{}).Execute(ctx, "a@b.com", "segredo123")
		if !errors.Is(err, apperr.ErrConflict) {
			t.Errorf("esperava conflito, obtido %v", err)
		}
	})
}
