package domain

import (
	"strings"
	"testing"

	"oficina-api/internal/shared/apperr"
)

func TestNewEmailValid(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"admin@oficina.com", "admin@oficina.com"},
		{"  Admin@Oficina.COM ", "admin@oficina.com"},
		{"a.b+tag@sub.dominio.com.br", "a.b+tag@sub.dominio.com.br"},
	}

	for _, tt := range tests {
		email, err := NewEmail(tt.raw)
		if err != nil {
			t.Errorf("NewEmail(%q): erro inesperado: %v", tt.raw, err)
			continue
		}
		if email.String() != tt.want {
			t.Errorf("NewEmail(%q) = %q, esperado %q", tt.raw, email.String(), tt.want)
		}
	}
}

func TestNewEmailInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"vazio", ""},
		{"sem arroba", "admin.oficina.com"},
		{"sem domínio", "admin@"},
		{"com nome de exibição", "Admin <admin@oficina.com>"},
		{"dois endereços", "a@b.com, c@d.com"},
		{"longo demais", strings.Repeat("a", 250) + "@b.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEmail(tt.raw)
			appErr, ok := apperr.As(err)
			if !ok || appErr.Code != CodeInvalidEmail {
				t.Errorf("erro = %v, esperado %s", err, CodeInvalidEmail)
			}
		})
	}
}
