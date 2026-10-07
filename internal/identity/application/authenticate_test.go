package application

import (
	"context"
	"errors"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

func seededUser(t *testing.T, repo *fakeRepo, active bool) {
	t.Helper()
	u, err := domain.NewUser("u1", "admin@oficina.local", "hash:segredo123", domain.RoleAdmin, testTime)
	if err != nil {
		t.Fatal(err)
	}
	u.Active = active
	repo.users[u.Email] = u
}

func TestAuthenticate_Success(t *testing.T) {
	repo, hasher := newFakeRepo(), &fakeHasher{}
	seededUser(t, repo, true)
	uc, err := NewAuthenticate(repo, hasher, fakeIssuer{})
	if err != nil {
		t.Fatal(err)
	}

	token, err := uc.Execute(context.Background(), "  ADMIN@oficina.local ", "segredo123")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if token.Value != "token-u1" {
		t.Errorf("token = %q", token.Value)
	}
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	cases := []struct {
		name     string
		email    string
		password string
		active   bool
	}{
		{"senha errada", "admin@oficina.local", "errada123", true},
		{"usuário inexistente", "outro@oficina.local", "segredo123", true},
		{"e-mail malformado", "isso-nao-e-email", "segredo123", true},
		{"usuário inativo", "admin@oficina.local", "segredo123", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, hasher := newFakeRepo(), &fakeHasher{}
			seededUser(t, repo, c.active)
			uc, err := NewAuthenticate(repo, hasher, fakeIssuer{})
			if err != nil {
				t.Fatal(err)
			}

			_, err = uc.Execute(context.Background(), c.email, c.password)
			if !errors.Is(err, apperr.ErrUnauthorized) || apperr.CodeOf(err) != "CREDENCIAIS_INVALIDAS" {
				t.Fatalf("esperava CREDENCIAIS_INVALIDAS, obtido %v", err)
			}
		})
	}
}

// Garante a equalização de tempo: usuário inexistente também executa uma comparação de hash.
func TestAuthenticate_UnknownUserStillComparesHash(t *testing.T) {
	repo, hasher := newFakeRepo(), &fakeHasher{}
	uc, err := NewAuthenticate(repo, hasher, fakeIssuer{})
	if err != nil {
		t.Fatal(err)
	}
	before := hasher.compare

	_, _ = uc.Execute(context.Background(), "ninguem@oficina.local", "qualquer123")
	_, _ = uc.Execute(context.Background(), "email-invalido", "qualquer123")

	if got := hasher.compare - before; got != 2 {
		t.Errorf("comparações de hash = %d, esperado 2", got)
	}
}

func TestAuthenticate_InfrastructureFailures(t *testing.T) {
	t.Run("falha no hash fictício", func(t *testing.T) {
		_, err := NewAuthenticate(newFakeRepo(), &fakeHasher{hashErr: errors.New("boom")}, fakeIssuer{})
		if err == nil {
			t.Fatal("esperava erro")
		}
	})
	t.Run("falha no repositório não vira credencial inválida", func(t *testing.T) {
		repo := newFakeRepo()
		repo.findErr = errors.New("banco fora")
		uc, _ := NewAuthenticate(repo, &fakeHasher{}, fakeIssuer{})
		_, err := uc.Execute(context.Background(), "admin@oficina.local", "segredo123")
		if err == nil || errors.Is(err, apperr.ErrUnauthorized) {
			t.Fatalf("esperava erro de infraestrutura, obtido %v", err)
		}
	})
	t.Run("falha ao emitir token", func(t *testing.T) {
		repo := newFakeRepo()
		seededUser(t, repo, true)
		uc, _ := NewAuthenticate(repo, &fakeHasher{}, fakeIssuer{err: errors.New("sem chave")})
		_, err := uc.Execute(context.Background(), "admin@oficina.local", "segredo123")
		if err == nil || errors.Is(err, apperr.ErrUnauthorized) {
			t.Fatalf("esperava erro de infraestrutura, obtido %v", err)
		}
	})
}
