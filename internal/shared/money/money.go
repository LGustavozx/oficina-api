// Package money representa valores em reais como inteiros em centavos.
// Float não serve para dinheiro (0.1 + 0.2 != 0.3); inteiros dão conta exata.
package money

import (
	"strconv"
	"strings"
)

type Money struct {
	cents int64
}

func FromCents(cents int64) Money { return Money{cents: cents} }

func Zero() Money { return Money{} }

func (m Money) Cents() int64 { return m.cents }

func (m Money) Add(other Money) Money { return Money{cents: m.cents + other.cents} }

// Sub pode resultar em valor negativo.
func (m Money) Sub(other Money) Money { return Money{cents: m.cents - other.cents} }

func (m Money) Mul(quantity int64) Money { return Money{cents: m.cents * quantity} }

func (m Money) IsNegative() bool { return m.cents < 0 }

func (m Money) IsZero() bool { return m.cents == 0 }

// String formata no padrão brasileiro: "R$ 1.234,56".
func (m Money) String() string {
	cents := m.cents
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	reais := groupThousands(strconv.FormatInt(cents/100, 10))
	return sign + "R$ " + reais + "," + twoDigits(cents%100)
}

func groupThousands(digits string) string {
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}
	return b.String()
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
