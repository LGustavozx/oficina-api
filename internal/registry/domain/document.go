package domain

import (
	"strings"

	"oficina-api/internal/shared/apperr"
)

const CodeInvalidDocument = "INVALID_DOCUMENT"

type DocumentType string

const (
	DocumentCPF  DocumentType = "CPF"
	DocumentCNPJ DocumentType = "CNPJ"
)

// Document é um CPF ou CNPJ válido, guardado sem pontuação.
type Document struct {
	value string
	kind  DocumentType
}

func NewDocument(raw string) (Document, error) {
	value := normalizeDocument(raw)

	switch {
	case len(value) == 11 && validCPF(value):
		return Document{value: value, kind: DocumentCPF}, nil
	case len(value) == 14 && validCNPJ(value):
		return Document{value: value, kind: DocumentCNPJ}, nil
	}
	return Document{}, apperr.Validation(CodeInvalidDocument, "CPF/CNPJ inválido")
}

func (d Document) String() string { return d.value }

func (d Document) Type() DocumentType { return d.kind }

func (d Document) Formatted() string {
	v := d.value
	if d.kind == DocumentCPF {
		return v[0:3] + "." + v[3:6] + "." + v[6:9] + "-" + v[9:11]
	}
	return v[0:2] + "." + v[2:5] + "." + v[5:8] + "/" + v[8:12] + "-" + v[12:14]
}

func normalizeDocument(raw string) string {
	cleaned := strings.NewReplacer(".", "", "-", "", "/", "", " ", "").Replace(raw)
	return strings.ToUpper(cleaned)
}

var (
	cpfWeights1  = []int{10, 9, 8, 7, 6, 5, 4, 3, 2}
	cpfWeights2  = []int{11, 10, 9, 8, 7, 6, 5, 4, 3, 2}
	cnpjWeights1 = []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	cnpjWeights2 = []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
)

func validCPF(v string) bool {
	if !allDigits(v) || allSame(v) {
		return false
	}
	return checkDigit(v[:9], cpfWeights1) == int(v[9]-'0') &&
		checkDigit(v[:10], cpfWeights2) == int(v[10]-'0')
}

// O CNPJ alfanumérico (a partir de jul/2026) usa letras nas 12 primeiras
// posições; os dois dígitos verificadores continuam numéricos.
func validCNPJ(v string) bool {
	if !allDigits(v[12:]) || allSame(v) {
		return false
	}
	for _, c := range v[:12] {
		if !isDigit(c) && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return checkDigit(v[:12], cnpjWeights1) == int(v[12]-'0') &&
		checkDigit(v[:13], cnpjWeights2) == int(v[13]-'0')
}

// checkDigit calcula o dígito verificador (módulo 11). O valor de cada
// caractere é seu código ASCII menos 48: '0'..'9' valem 0..9 e 'A'..'Z'
// valem 17..42, regra oficial do CNPJ alfanumérico (e igual para dígitos).
func checkDigit(base string, weights []int) int {
	sum := 0
	for i, c := range base {
		sum += int(c-'0') * weights[i]
	}
	if rest := sum % 11; rest >= 2 {
		return 11 - rest
	}
	return 0
}

func isDigit(c rune) bool { return c >= '0' && c <= '9' }

func allDigits(s string) bool {
	for _, c := range s {
		if !isDigit(c) {
			return false
		}
	}
	return true
}

func allSame(s string) bool {
	return strings.Count(s, s[:1]) == len(s)
}
