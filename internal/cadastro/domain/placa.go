package domain

import (
	"regexp"
	"strings"

	"github.com/LGustavozx/oficina-api/internal/shared/erros"
)

var (
	placaAntiga   = regexp.MustCompile(`^[A-Z]{3}[0-9]{4}$`)
	placaMercosul = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z][0-9]{2}$`)
)

// Placa é o objeto de valor de placa veicular (padrão antigo ou Mercosul), normalizada (RN02).
type Placa struct{ valor string }

// NovaPlaca normaliza (maiúsculas, sem hífen, sem espaços nas bordas) e valida a placa.
func NovaPlaca(entrada string) (Placa, error) {
	v := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(entrada), "-", ""))
	if !placaAntiga.MatchString(v) && !placaMercosul.MatchString(v) {
		return Placa{}, erros.Validacao("PLACA_INVALIDA", "placa inválida: use AAA-9999 ou AAA9A99")
	}
	return Placa{valor: v}, nil
}

// String devolve a placa normalizada (formato de persistência).
func (p Placa) String() string { return p.valor }

// Mercosul informa se a placa segue o padrão Mercosul.
func (p Placa) Mercosul() bool { return placaMercosul.MatchString(p.valor) }

// Formatado devolve a placa para exibição: AAA-9999 (antiga) ou AAA9A99 (Mercosul).
func (p Placa) Formatado() string {
	if p.valor == "" || p.Mercosul() {
		return p.valor
	}
	return p.valor[:3] + "-" + p.valor[3:]
}
