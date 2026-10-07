// Package domain contém as regras do contexto Cadastro (clientes e veículos).
package domain

import (
	"strings"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

// PersonType distingue pessoa física de jurídica.
type PersonType string

const (
	// Individual é pessoa física (CPF).
	Individual PersonType = "PF"
	// Company é pessoa jurídica (CNPJ).
	Company PersonType = "PJ"
)

// Document é o objeto de valor CPF/CNPJ, sempre válido e armazenado sem máscara (RN01).
type Document struct {
	value      string
	personType PersonType
}

// NewDocument normaliza (remove máscara) e valida um CPF ou CNPJ.
func NewDocument(input string) (Document, error) {
	digits := onlyDigits(input)
	switch len(digits) {
	case 11:
		if !validCPF(digits) {
			return Document{}, apperr.Validation("CPF_INVALIDO", "CPF inválido")
		}
		return Document{value: digits, personType: Individual}, nil
	case 14:
		if !validCNPJ(digits) {
			return Document{}, apperr.Validation("CNPJ_INVALIDO", "CNPJ inválido")
		}
		return Document{value: digits, personType: Company}, nil
	default:
		return Document{}, apperr.Validation("DOCUMENTO_INVALIDO", "o documento deve ter 11 dígitos (CPF) ou 14 dígitos (CNPJ)")
	}
}

// String devolve o documento sem máscara (formato de persistência).
func (d Document) String() string { return d.value }

// Type informa se é pessoa física ou jurídica.
func (d Document) Type() PersonType { return d.personType }

// Formatted devolve o documento com máscara de exibição.
func (d Document) Formatted() string {
	v := d.value
	switch d.personType {
	case Individual:
		return v[0:3] + "." + v[3:6] + "." + v[6:9] + "-" + v[9:11]
	case Company:
		return v[0:2] + "." + v[2:5] + "." + v[5:8] + "/" + v[8:12] + "-" + v[12:14]
	}
	return v
}

// Masked oculta a maior parte do documento, para uso em logs (LGPD).
func (d Document) Masked() string {
	v := d.value
	switch d.personType {
	case Individual:
		return "***." + v[3:6] + "." + v[6:9] + "-**"
	case Company:
		return "**.***." + v[5:8] + "/" + v[8:12] + "-**"
	}
	return ""
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func allEqual(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func validCPF(c string) bool {
	if allEqual(c) {
		return false
	}
	return cpfDigit(c[:9], 10) == int(c[9]-'0') && cpfDigit(c[:10], 11) == int(c[10]-'0')
}

// cpfDigit calcula um dígito verificador com pesos decrescentes a partir de startWeight.
func cpfDigit(base string, startWeight int) int {
	sum := 0
	for i := 0; i < len(base); i++ {
		sum += int(base[i]-'0') * (startWeight - i)
	}
	rest := (sum * 10) % 11
	if rest == 10 {
		return 0
	}
	return rest
}

func validCNPJ(c string) bool {
	if allEqual(c) {
		return false
	}
	weights1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	weights2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	return cnpjDigit(c[:12], weights1) == int(c[12]-'0') && cnpjDigit(c[:13], weights2) == int(c[13]-'0')
}

func cnpjDigit(base string, weights []int) int {
	sum := 0
	for i := 0; i < len(base); i++ {
		sum += int(base[i]-'0') * weights[i]
	}
	rest := sum % 11
	if rest < 2 {
		return 0
	}
	return 11 - rest
}
