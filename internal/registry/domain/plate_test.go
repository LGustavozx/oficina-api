package domain

import (
	"testing"

	"oficina-api/internal/shared/apperr"
)

func TestNewPlateValid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"formato antigo com hífen", "ABC-1234", "ABC1234"},
		{"formato antigo sem hífen", "ABC1234", "ABC1234"},
		{"Mercosul", "ABC1D23", "ABC1D23"},
		{"minúsculas", "abc1d23", "ABC1D23"},
		{"com espaços", " abc-1234 ", "ABC1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plate, err := NewPlate(tt.raw)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if plate.String() != tt.want {
				t.Errorf("String() = %q, esperado %q", plate.String(), tt.want)
			}
		})
	}
}

func TestNewPlateInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"vazia", ""},
		{"curta", "AB1234"},
		{"longa", "ABCD1234"},
		{"começa com número", "1BC1234"},
		{"só letras", "ABCDEFG"},
		{"só números", "1234567"},
		{"caractere especial", "ABC#234"},
		{"letra onde deve ser número", "ABC12A4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPlate(tt.raw)
			if err == nil {
				t.Fatal("esperado erro")
			}
			appErr, ok := apperr.As(err)
			if !ok || appErr.Code != CodeInvalidPlate || appErr.Kind != apperr.KindValidation {
				t.Errorf("erro = %v, esperado validação %s", err, CodeInvalidPlate)
			}
		})
	}
}
