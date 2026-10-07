package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

func TestNormalizeEmail(t *testing.T) {
	valid := map[string]string{
		"admin@oficina.local":     "admin@oficina.local",
		"  Admin@Oficina.Local  ": "admin@oficina.local",
	}
	for input, want := range valid {
		got, err := NormalizeEmail(input)
		if err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; esperado %q", input, got, err, want)
		}
	}

	invalid := []string{"", "   ", "sem-arroba", "Nome <a@b.com>", "a@", "@b.com", strings.Repeat("a", 250) + "@b.com"}
	for _, input := range invalid {
		_, err := NormalizeEmail(input)
		if !errors.Is(err, apperr.ErrValidation) || apperr.CodeOf(err) != "EMAIL_INVALIDO" {
			t.Errorf("NormalizeEmail(%q) deveria falhar com EMAIL_INVALIDO, obtido %v", input, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"mínimo", "12345678", false},
		{"máximo", strings.Repeat("a", 72), false},
		{"curta", "1234567", true},
		{"longa demais", strings.Repeat("a", 73), true},
		{"vazia", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.password)
			if (err != nil) != c.wantErr {
				t.Fatalf("erro = %v, esperado erro? %v", err, c.wantErr)
			}
			if c.wantErr && apperr.CodeOf(err) != "SENHA_INVALIDA" {
				t.Errorf("código = %q", apperr.CodeOf(err))
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	u, err := NewUser("id-1", " Admin@Oficina.Local ", "hash", RoleAdmin, now)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if u.Email != "admin@oficina.local" || !u.Active || u.Role != RoleAdmin || !u.CreatedAt.Equal(now) {
		t.Errorf("usuário inesperado: %+v", u)
	}

	invalid := []struct {
		name                  string
		id, email, hash, code string
		role                  Role
	}{
		{"e-mail inválido", "id", "x", "hash", "EMAIL_INVALIDO", RoleAdmin},
		{"sem id", "", "a@b.com", "hash", "USUARIO_INVALIDO", RoleAdmin},
		{"sem hash", "id", "a@b.com", "", "USUARIO_INVALIDO", RoleAdmin},
		{"perfil inválido", "id", "a@b.com", "hash", "PERFIL_INVALIDO", Role("ROOT")},
	}
	for _, c := range invalid {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewUser(c.id, c.email, c.hash, c.role, now)
			if apperr.CodeOf(err) != c.code {
				t.Errorf("código = %q, esperado %q (erro: %v)", apperr.CodeOf(err), c.code, err)
			}
		})
	}
}
