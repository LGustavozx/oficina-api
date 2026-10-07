package domain

import (
	"errors"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

func TestNewPlate_Valid(t *testing.T) {
	cases := []struct {
		name, input, value, formatted string
		mercosur                      bool
	}{
		{"antiga com hífen", "ABC-1234", "ABC1234", "ABC-1234", false},
		{"antiga sem hífen", "ABC1234", "ABC1234", "ABC-1234", false},
		{"antiga minúscula", "abc-1234", "ABC1234", "ABC-1234", false},
		{"mercosul", "ABC1D23", "ABC1D23", "ABC1D23", true},
		{"mercosul minúscula com espaços", "  abc1d23 ", "ABC1D23", "ABC1D23", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlate(c.input)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if p.String() != c.value || p.Formatted() != c.formatted || p.IsMercosur() != c.mercosur {
				t.Errorf("obtido (%s, %s, %v)", p.String(), p.Formatted(), p.IsMercosur())
			}
		})
	}
}

func TestNewPlate_Invalid(t *testing.T) {
	for _, input := range []string{"", "ABC123", "AB1-2345", "ABCD123", "1BC1D23", "ABC1DD3", "ABC-12345", "ABC 12 34", "ÁBC1234"} {
		t.Run(input, func(t *testing.T) {
			_, err := NewPlate(input)
			if !errors.Is(err, apperr.ErrValidation) || apperr.CodeOf(err) != "PLACA_INVALIDA" {
				t.Errorf("esperava PLACA_INVALIDA, obtido %v", err)
			}
		})
	}
}

func TestZeroPlate(t *testing.T) {
	var p Plate
	if p.Formatted() != "" || p.String() != "" {
		t.Error("placa zero deve produzir strings vazias")
	}
}
