package money

import "testing"

func TestArithmetic(t *testing.T) {
	a := FromCents(1050)
	b := FromCents(275)

	if got := a.Add(b).Cents(); got != 1325 {
		t.Errorf("Add = %d, esperado 1325", got)
	}
	if got := a.Sub(b).Cents(); got != 775 {
		t.Errorf("Sub = %d, esperado 775", got)
	}
	if got := b.Mul(4).Cents(); got != 1100 {
		t.Errorf("Mul = %d, esperado 1100", got)
	}
}

func TestExactSum(t *testing.T) {
	if got := FromCents(10).Add(FromCents(20)); got != FromCents(30) {
		t.Errorf("0,10 + 0,20 = %v, esperado 0,30", got)
	}
}

func TestImmutability(t *testing.T) {
	a := FromCents(100)
	_ = a.Add(FromCents(50))

	if a.Cents() != 100 {
		t.Errorf("a foi alterado: %d", a.Cents())
	}
}

func TestSignAndZero(t *testing.T) {
	if !Zero().IsZero() {
		t.Error("Zero() deveria ser zero")
	}
	if FromCents(1).IsZero() {
		t.Error("1 centavo não é zero")
	}
	if !FromCents(-1).IsNegative() {
		t.Error("-1 deveria ser negativo")
	}
	if FromCents(1).IsNegative() || Zero().IsNegative() {
		t.Error("positivo e zero não são negativos")
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "R$ 0,00"},
		{5, "R$ 0,05"},
		{99, "R$ 0,99"},
		{100, "R$ 1,00"},
		{12345, "R$ 123,45"},
		{123456, "R$ 1.234,56"},
		{100000000, "R$ 1.000.000,00"},
		{-500, "-R$ 5,00"},
	}

	for _, tt := range tests {
		if got := FromCents(tt.cents).String(); got != tt.want {
			t.Errorf("FromCents(%d).String() = %q, esperado %q", tt.cents, got, tt.want)
		}
	}
}
