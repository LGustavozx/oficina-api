package testdb

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// Os testes de integração compartilham um banco e, por isso, leem linhas
// gravadas por outros testes. Qualquer dado que um teste grave direto por SQL
// precisa ter o formato que o domínio aceita ao ler; estes geradores garantem
// isso (um documento ou uma placa inventados à mão derrubam a leitura de toda
// a lista nos outros pacotes).

// RandomCPF devolve um CPF válido, só com dígitos.
func RandomCPF() string {
	for {
		digits := make([]int, 11)
		for i := 0; i < 9; i++ {
			digits[i] = rand.IntN(10)
		}
		digits[9] = cpfCheckDigit(digits[:9], 10)
		digits[10] = cpfCheckDigit(digits[:10], 11)

		var b strings.Builder
		same := true
		for _, d := range digits {
			fmt.Fprint(&b, d)
			same = same && d == digits[0]
		}
		if !same { // sequências como 111.111.111-11 passam na conta, mas o domínio as recusa
			return b.String()
		}
	}
}

func cpfCheckDigit(base []int, firstWeight int) int {
	sum := 0
	for i, d := range base {
		sum += d * (firstWeight - i)
	}
	if rest := sum % 11; rest >= 2 {
		return 11 - rest
	}
	return 0
}

// RandomPlate devolve uma placa válida no formato Mercosul (ABC1D23).
func RandomPlate() string {
	letter := func() byte { return byte('A' + rand.IntN(26)) }
	digit := func() byte { return byte('0' + rand.IntN(10)) }

	return string([]byte{letter(), letter(), letter(), digit(), letter(), digit(), digit()})
}
