package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestCategories(t *testing.T) {
	cases := []struct {
		name     string
		err      *Error
		category error
	}{
		{"validação", Validation("X", "m"), ErrValidation},
		{"não encontrado", NotFound("X", "m"), ErrNotFound},
		{"conflito", Conflict("X", "m"), ErrConflict},
		{"não autorizado", Unauthorized("X", "m"), ErrUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !errors.Is(c.err, c.category) {
				t.Errorf("errors.Is deveria reconhecer %v", c.category)
			}
			other := ErrConflict
			if errors.Is(c.category, ErrConflict) {
				other = ErrValidation
			}
			if errors.Is(c.err, other) {
				t.Errorf("não deveria casar com %v", other)
			}
		})
	}
}

func TestMessageAndCode(t *testing.T) {
	err := Validation("CPF_INVALIDO", "CPF inválido")
	if got, want := err.Error(), "CPF_INVALIDO: CPF inválido"; got != want {
		t.Errorf("Error() = %q, esperado %q", got, want)
	}
	wrapped := fmt.Errorf("contexto: %w", err)
	if got := CodeOf(wrapped); got != "CPF_INVALIDO" {
		t.Errorf("CodeOf() = %q", got)
	}
	if got := CodeOf(errors.New("comum")); got != "" {
		t.Errorf("CodeOf() de erro comum = %q, esperado vazio", got)
	}
}
