package domain

import (
	"errors"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

func TestNewDocument_Valid(t *testing.T) {
	cases := []struct {
		name, input, value, formatted, masked string
		personType                            PersonType
	}{
		{"cpf sem máscara", "52998224725", "52998224725", "529.982.247-25", "***.982.247-**", Individual},
		{"cpf com máscara", "529.982.247-25", "52998224725", "529.982.247-25", "***.982.247-**", Individual},
		{"cpf da documentação", "123.456.789-09", "12345678909", "123.456.789-09", "***.456.789-**", Individual},
		{"cpf com resto 10", "111.444.777-35", "11144477735", "111.444.777-35", "***.444.777-**", Individual},
		{"cnpj sem máscara", "11222333000181", "11222333000181", "11.222.333/0001-81", "**.***.333/0001-**", Company},
		{"cnpj com máscara", "11.444.777/0001-61", "11444777000161", "11.444.777/0001-61", "**.***.777/0001-**", Company},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := NewDocument(c.input)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if d.String() != c.value || d.Type() != c.personType || d.Formatted() != c.formatted || d.Masked() != c.masked {
				t.Errorf("obtido (%s, %s, %s, %s)", d.String(), d.Type(), d.Formatted(), d.Masked())
			}
		})
	}
}

func TestNewDocument_Invalid(t *testing.T) {
	cases := []struct {
		name, input, code string
	}{
		{"vazio", "", "DOCUMENTO_INVALIDO"},
		{"tamanho 10", "1234567890", "DOCUMENTO_INVALIDO"},
		{"tamanho 12", "123456789012", "DOCUMENTO_INVALIDO"},
		{"letras", "abc", "DOCUMENTO_INVALIDO"},
		{"cpf dígitos repetidos", "111.111.111-11", "CPF_INVALIDO"},
		{"cpf dígito errado", "52998224724", "CPF_INVALIDO"},
		{"cpf primeiro dígito errado", "52998224735", "CPF_INVALIDO"},
		{"cnpj dígitos repetidos", "00000000000000", "CNPJ_INVALIDO"},
		{"cnpj dígito errado", "11222333000182", "CNPJ_INVALIDO"},
		{"cnpj primeiro dígito errado", "11222333000191", "CNPJ_INVALIDO"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewDocument(c.input)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("esperava erro de validação, obtido %v", err)
			}
			if got := apperr.CodeOf(err); got != c.code {
				t.Errorf("código = %q, esperado %q", got, c.code)
			}
		})
	}
}

func TestZeroDocument_DoesNotPanic(t *testing.T) {
	var d Document
	if d.Formatted() != "" || d.Masked() != "" || d.String() != "" {
		t.Error("documento zero deve produzir strings vazias")
	}
}
