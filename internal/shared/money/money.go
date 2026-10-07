// Package money provê o objeto de valor Money, em centavos inteiros (ADR-009).
package money

import (
	"fmt"
	"math"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

// Money representa um valor monetário não negativo em centavos de real.
type Money int64

// Zero é o valor monetário nulo.
const Zero Money = 0

// New cria um valor a partir de centavos; rejeita valores negativos (RN15).
func New(cents int64) (Money, error) {
	if cents < 0 {
		return 0, apperr.Validation("VALOR_NEGATIVO", "o valor monetário não pode ser negativo")
	}
	return Money(cents), nil
}

// Cents devolve o valor em centavos.
func (m Money) Cents() int64 { return int64(m) }

// Add soma dois valores, detectando estouro.
func (m Money) Add(other Money) (Money, error) {
	if int64(m) > math.MaxInt64-int64(other) {
		return 0, apperr.Validation("VALOR_EXCEDE_LIMITE", "o valor monetário excede o limite permitido")
	}
	return m + other, nil
}

// Multiply multiplica por uma quantidade não negativa, detectando estouro.
func (m Money) Multiply(qty int) (Money, error) {
	if qty < 0 {
		return 0, apperr.Validation("QUANTIDADE_INVALIDA", "a quantidade não pode ser negativa")
	}
	if qty != 0 && int64(m) > math.MaxInt64/int64(qty) {
		return 0, apperr.Validation("VALOR_EXCEDE_LIMITE", "o valor monetário excede o limite permitido")
	}
	return m * Money(qty), nil
}

// String formata em reais, ex.: "R$ 485,00".
func (m Money) String() string {
	return fmt.Sprintf("R$ %d,%02d", int64(m)/100, int64(m)%100)
}
