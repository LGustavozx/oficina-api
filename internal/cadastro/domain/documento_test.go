package domain

import (
	"errors"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/erros"
)

func TestNovoDocumento_Validos(t *testing.T) {
	casos := []struct {
		nome, entrada, valor, formatado, mascarado string
		tipo                                       TipoPessoa
	}{
		{"cpf sem mascara", "52998224725", "52998224725", "529.982.247-25", "***.982.247-**", PessoaFisica},
		{"cpf com mascara", "529.982.247-25", "52998224725", "529.982.247-25", "***.982.247-**", PessoaFisica},
		{"cpf da documentacao", "123.456.789-09", "12345678909", "123.456.789-09", "***.456.789-**", PessoaFisica},
		{"cpf com resto 10", "111.444.777-35", "11144477735", "111.444.777-35", "***.444.777-**", PessoaFisica},
		{"cnpj sem mascara", "11222333000181", "11222333000181", "11.222.333/0001-81", "**.***.333/0001-**", PessoaJuridica},
		{"cnpj com mascara", "11.444.777/0001-61", "11444777000161", "11.444.777/0001-61", "**.***.777/0001-**", PessoaJuridica},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := NovoDocumento(c.entrada)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if d.String() != c.valor || d.Tipo() != c.tipo || d.Formatado() != c.formatado || d.Mascarado() != c.mascarado {
				t.Errorf("obtido (%s, %s, %s, %s)", d.String(), d.Tipo(), d.Formatado(), d.Mascarado())
			}
		})
	}
}

func TestNovoDocumento_Invalidos(t *testing.T) {
	casos := []struct {
		nome, entrada, codigo string
	}{
		{"vazio", "", "DOCUMENTO_INVALIDO"},
		{"tamanho 10", "1234567890", "DOCUMENTO_INVALIDO"},
		{"tamanho 12", "123456789012", "DOCUMENTO_INVALIDO"},
		{"letras", "abc", "DOCUMENTO_INVALIDO"},
		{"cpf digitos repetidos", "111.111.111-11", "CPF_INVALIDO"},
		{"cpf digito errado", "52998224724", "CPF_INVALIDO"},
		{"cpf primeiro digito errado", "52998224735", "CPF_INVALIDO"},
		{"cnpj digitos repetidos", "00000000000000", "CNPJ_INVALIDO"},
		{"cnpj digito errado", "11222333000182", "CNPJ_INVALIDO"},
		{"cnpj primeiro digito errado", "11222333000191", "CNPJ_INVALIDO"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := NovoDocumento(c.entrada)
			if !errors.Is(err, erros.ErrValidacao) {
				t.Fatalf("esperava erro de validação, obtido %v", err)
			}
			if got := erros.Codigo(err); got != c.codigo {
				t.Errorf("código = %q, esperado %q", got, c.codigo)
			}
		})
	}
}

func TestDocumentoZero_NaoPanica(t *testing.T) {
	var d Documento
	if d.Formatado() != "" || d.Mascarado() != "" || d.String() != "" {
		t.Error("documento zero deve produzir strings vazias")
	}
}
