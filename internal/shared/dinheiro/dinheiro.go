// Package dinheiro provê o objeto de valor Dinheiro, em centavos inteiros (ADR-009).
package dinheiro

import (
	"fmt"
	"math"

	"github.com/LGustavozx/oficina-api/internal/shared/erros"
)

// Dinheiro representa um valor monetário não negativo em centavos de real.
type Dinheiro int64

// Zero é o valor monetário nulo.
const Zero Dinheiro = 0

// Novo cria um valor a partir de centavos; rejeita valores negativos (RN15).
func Novo(centavos int64) (Dinheiro, error) {
	if centavos < 0 {
		return 0, erros.Validacao("VALOR_NEGATIVO", "o valor monetário não pode ser negativo")
	}
	return Dinheiro(centavos), nil
}

// Centavos devolve o valor em centavos.
func (d Dinheiro) Centavos() int64 { return int64(d) }

// Somar soma dois valores, detectando estouro.
func (d Dinheiro) Somar(outro Dinheiro) (Dinheiro, error) {
	if int64(d) > math.MaxInt64-int64(outro) {
		return 0, erros.Validacao("VALOR_EXCEDE_LIMITE", "o valor monetário excede o limite permitido")
	}
	return d + outro, nil
}

// Multiplicar multiplica por uma quantidade não negativa, detectando estouro.
func (d Dinheiro) Multiplicar(qtd int) (Dinheiro, error) {
	if qtd < 0 {
		return 0, erros.Validacao("QUANTIDADE_INVALIDA", "a quantidade não pode ser negativa")
	}
	if qtd != 0 && int64(d) > math.MaxInt64/int64(qtd) {
		return 0, erros.Validacao("VALOR_EXCEDE_LIMITE", "o valor monetário excede o limite permitido")
	}
	return d * Dinheiro(qtd), nil
}

// String formata em reais, ex.: "R$ 485,00".
func (d Dinheiro) String() string {
	return fmt.Sprintf("R$ %d,%02d", int64(d)/100, int64(d)%100)
}
