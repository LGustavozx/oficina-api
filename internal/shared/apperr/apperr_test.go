package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestConstructorsSetKind(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want Kind
	}{
		{"Validation", Validation("C", "m"), KindValidation},
		{"NotFound", NotFound("C", "m"), KindNotFound},
		{"Conflict", Conflict("C", "m"), KindConflict},
		{"Unauthorized", Unauthorized("C", "m"), KindUnauthorized},
		{"New", New(KindInternal, "C", "m"), KindInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Kind != tt.want {
				t.Errorf("Kind = %q, esperado %q", tt.err.Kind, tt.want)
			}
			if tt.err.Code != "C" || tt.err.Message != "m" {
				t.Errorf("Code/Message errados: %+v", tt.err)
			}
		})
	}
}

func TestErrorString(t *testing.T) {
	semCausa := Validation("INVALID_X", "x inválido")
	if got := semCausa.Error(); got != "INVALID_X: x inválido" {
		t.Errorf("Error() = %q", got)
	}

	comCausa := Wrap(KindInternal, "DB_FAILURE", "falha no banco", errors.New("timeout"))
	if got := comCausa.Error(); got != "DB_FAILURE: falha no banco: timeout" {
		t.Errorf("Error() = %q", got)
	}
}

func TestWrapKeepsCause(t *testing.T) {
	cause := errors.New("causa raiz")
	err := Wrap(KindInternal, "C", "m", cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is deveria encontrar a causa original")
	}
}

func TestAs(t *testing.T) {
	t.Run("encontra erro de aplicação embrulhado", func(t *testing.T) {
		original := NotFound("CUSTOMER_NOT_FOUND", "cliente não encontrado")
		embrulhado := fmt.Errorf("buscar cliente: %w", original)

		got, ok := As(embrulhado)
		if !ok {
			t.Fatal("As deveria encontrar o erro")
		}
		if got.Kind != KindNotFound || got.Code != "CUSTOMER_NOT_FOUND" {
			t.Errorf("erro errado: %+v", got)
		}
	})

	t.Run("não encontra em erro comum", func(t *testing.T) {
		if _, ok := As(errors.New("erro qualquer")); ok {
			t.Error("As não deveria encontrar erro de aplicação")
		}
	})
}
