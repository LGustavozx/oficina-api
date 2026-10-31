package testdb

import (
	"regexp"
	"testing"
)

// cpfIsValid refaz a conta dos dígitos verificadores de forma independente.
func cpfIsValid(cpf string) bool {
	if len(cpf) != 11 {
		return false
	}
	digit := func(n int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(cpf[i]-'0') * (n + 1 - i)
		}
		if rest := sum % 11; rest >= 2 {
			return 11 - rest
		}
		return 0
	}
	return digit(9) == int(cpf[9]-'0') && digit(10) == int(cpf[10]-'0')
}

func TestRandomCPFIsValid(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		cpf := RandomCPF()

		if !cpfIsValid(cpf) {
			t.Fatalf("CPF gerado é inválido: %s", cpf)
		}
		if cpf == "00000000000" || cpf == "11111111111" || cpf == "99999999999" {
			t.Fatalf("o domínio recusa sequências repetidas: %s", cpf)
		}
		seen[cpf] = true
	}
	if len(seen) < 1900 {
		t.Errorf("só %d CPFs diferentes em 2000: o gerador repete demais", len(seen))
	}
}

func TestRandomPlateIsValid(t *testing.T) {
	mercosul := regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z][0-9]{2}$`)

	for i := 0; i < 2000; i++ {
		if plate := RandomPlate(); !mercosul.MatchString(plate) {
			t.Fatalf("placa gerada fora do formato Mercosul: %s", plate)
		}
	}
}
