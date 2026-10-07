package money

import (
	"errors"
	"math"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

func TestNew(t *testing.T) {
	if m, err := New(1500); err != nil || m.Cents() != 1500 {
		t.Fatalf("New(1500) = %v, %v", m, err)
	}
	if _, err := New(-1); !errors.Is(err, apperr.ErrValidation) {
		t.Errorf("negativo deveria ser erro de validação, obtido %v", err)
	}
	if m, err := New(0); err != nil || m != Zero {
		t.Errorf("zero deveria ser válido")
	}
}

func TestAdd(t *testing.T) {
	m, err := Money(1050).Add(Money(250))
	if err != nil || m != 1300 {
		t.Fatalf("Add = %v, %v", m, err)
	}
	if _, err := Money(math.MaxInt64).Add(1); err == nil {
		t.Error("esperava erro de estouro")
	}
}

func TestMultiply(t *testing.T) {
	cases := []struct {
		name    string
		m       Money
		qty     int
		want    Money
		wantErr bool
	}{
		{"normal", 2500, 4, 10000, false},
		{"zero unidades", 2500, 0, 0, false},
		{"negativa", 2500, -1, 0, true},
		{"estouro", Money(math.MaxInt64), 2, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.m.Multiply(c.qty)
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
	cases := map[Money]string{0: "R$ 0,00", 5: "R$ 0,05", 48500: "R$ 485,00", 100099: "R$ 1000,99"}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("String(%d) = %q, esperado %q", int64(m), got, want)
		}
	}
}
