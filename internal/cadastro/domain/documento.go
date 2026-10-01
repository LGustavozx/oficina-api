// Package domain contém as regras do contexto Cadastro (clientes e veículos).
package domain

import (
	"strings"

	"github.com/LGustavozx/oficina-api/internal/shared/erros"
)

// TipoPessoa distingue pessoa física de jurídica.
type TipoPessoa string

const (
	PessoaFisica   TipoPessoa = "PF"
	PessoaJuridica TipoPessoa = "PJ"
)

// Documento é o objeto de valor CPF/CNPJ, sempre válido e armazenado sem máscara (RN01).
type Documento struct {
	valor string
	tipo  TipoPessoa
}

// NovoDocumento normaliza (remove máscara) e valida um CPF ou CNPJ.
func NovoDocumento(entrada string) (Documento, error) {
	digitos := somenteDigitos(entrada)
	switch len(digitos) {
	case 11:
		if !cpfValido(digitos) {
			return Documento{}, erros.Validacao("CPF_INVALIDO", "CPF inválido")
		}
		return Documento{valor: digitos, tipo: PessoaFisica}, nil
	case 14:
		if !cnpjValido(digitos) {
			return Documento{}, erros.Validacao("CNPJ_INVALIDO", "CNPJ inválido")
		}
		return Documento{valor: digitos, tipo: PessoaJuridica}, nil
	default:
		return Documento{}, erros.Validacao("DOCUMENTO_INVALIDO", "o documento deve ter 11 dígitos (CPF) ou 14 dígitos (CNPJ)")
	}
}

// String devolve o documento sem máscara (formato de persistência).
func (d Documento) String() string { return d.valor }

// Tipo informa se é PF ou PJ.
func (d Documento) Tipo() TipoPessoa { return d.tipo }

// Formatado devolve o documento com máscara de exibição.
func (d Documento) Formatado() string {
	v := d.valor
	switch d.tipo {
	case PessoaFisica:
		return v[0:3] + "." + v[3:6] + "." + v[6:9] + "-" + v[9:11]
	case PessoaJuridica:
		return v[0:2] + "." + v[2:5] + "." + v[5:8] + "/" + v[8:12] + "-" + v[12:14]
	}
	return v
}

// Mascarado oculta a maior parte do documento, para uso em logs (LGPD).
func (d Documento) Mascarado() string {
	v := d.valor
	switch d.tipo {
	case PessoaFisica:
		return "***." + v[3:6] + "." + v[6:9] + "-**"
	case PessoaJuridica:
		return "**.***." + v[5:8] + "/" + v[8:12] + "-**"
	}
	return ""
}

func somenteDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func todosIguais(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func cpfValido(c string) bool {
	if todosIguais(c) {
		return false
	}
	return digitoCPF(c[:9], 10) == int(c[9]-'0') && digitoCPF(c[:10], 11) == int(c[10]-'0')
}

// digitoCPF calcula um dígito verificador com pesos decrescentes a partir de pesoInicial.
func digitoCPF(base string, pesoInicial int) int {
	soma := 0
	for i := 0; i < len(base); i++ {
		soma += int(base[i]-'0') * (pesoInicial - i)
	}
	resto := (soma * 10) % 11
	if resto == 10 {
		return 0
	}
	return resto
}

func cnpjValido(c string) bool {
	if todosIguais(c) {
		return false
	}
	pesos1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	pesos2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	return digitoCNPJ(c[:12], pesos1) == int(c[12]-'0') && digitoCNPJ(c[:13], pesos2) == int(c[13]-'0')
}

func digitoCNPJ(base string, pesos []int) int {
	soma := 0
	for i := 0; i < len(base); i++ {
		soma += int(base[i]-'0') * pesos[i]
	}
	resto := soma % 11
	if resto < 2 {
		return 0
	}
	return 11 - resto
}
