package domain

import (
	"errors"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/erros"
)

func TestNovaPlaca_Validas(t *testing.T) {
	casos := []struct {
		nome, entrada, valor, formatado string
		mercosul                        bool
	}{
		{"antiga com hifen", "ABC-1234", "ABC1234", "ABC-1234", false},
		{"antiga sem hifen", "ABC1234", "ABC1234", "ABC-1234", false},
		{"antiga minuscula", "abc-1234", "ABC1234", "ABC-1234", false},
		{"mercosul", "ABC1D23", "ABC1D23", "ABC1D23", true},
		{"mercosul minuscula com espacos", "  abc1d23 ", "ABC1D23", "ABC1D23", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, err := NovaPlaca(c.entrada)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if p.String() != c.valor || p.Formatado() != c.formatado || p.Mercosul() != c.mercosul {
				t.Errorf("obtido (%s, %s, %v)", p.String(), p.Formatado(), p.Mercosul())
			}
		})
	}
}

func TestNovaPlaca_Invalidas(t *testing.T) {
	for _, entrada := range []string{"", "ABC123", "AB1-2345", "ABCD123", "1BC1D23", "ABC1DD3", "ABC-12345", "ABC 12 34", "ÁBC1234"} {
		t.Run(entrada, func(t *testing.T) {
			_, err := NovaPlaca(entrada)
			if !errors.Is(err, erros.ErrValidacao) || erros.Codigo(err) != "PLACA_INVALIDA" {
				t.Errorf("esperava PLACA_INVALIDA, obtido %v", err)
			}
		})
	}
}

func TestPlacaZero(t *testing.T) {
	var p Placa
	if p.Formatado() != "" || p.String() != "" {
		t.Error("placa zero deve produzir strings vazias")
	}
}
