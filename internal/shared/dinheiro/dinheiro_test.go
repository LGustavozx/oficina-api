package dinheiro

import (
	"errors"
	"math"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/erros"
)

func TestNovo(t *testing.T) {
	if d, err := Novo(1500); err != nil || d.Centavos() != 1500 {
		t.Fatalf("Novo(1500) = %v, %v", d, err)
	}
	if _, err := Novo(-1); !errors.Is(err, erros.ErrValidacao) {
		t.Errorf("negativo deveria ser erro de validação, obtido %v", err)
	}
	if d, err := Novo(0); err != nil || d != Zero {
		t.Errorf("zero deveria ser válido")
	}
}

func TestSomar(t *testing.T) {
	d, err := Dinheiro(1050).Somar(Dinheiro(250))
	if err != nil || d != 1300 {
		t.Fatalf("Somar = %v, %v", d, err)
	}
	if _, err := Dinheiro(math.MaxInt64).Somar(1); err == nil {
		t.Error("esperava erro de estouro")
	}
}

func TestMultiplicar(t *testing.T) {
	casos := []struct {
		nome    string
		d       Dinheiro
		qtd     int
		want    Dinheiro
		wantErr bool
	}{
		{"normal", 2500, 4, 10000, false},
		{"zero unidades", 2500, 0, 0, false},
		{"negativa", 2500, -1, 0, true},
		{"estouro", Dinheiro(math.MaxInt64), 2, 0, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := c.d.Multiplicar(c.qtd)
			if (err != nil) != c.wantErr {
				t.Fatalf("erro = %v, esperado erro? %v", err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	casos := map[Dinheiro]string{0: "R$ 0,00", 5: "R$ 0,05", 48500: "R$ 485,00", 100099: "R$ 1000,99"}
	for d, want := range casos {
		if got := d.String(); got != want {
			t.Errorf("String(%d) = %q, esperado %q", int64(d), got, want)
		}
	}
}
