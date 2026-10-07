package domain

import (
	"regexp"
	"strings"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

var (
	oldPlate      = regexp.MustCompile(`^[A-Z]{3}[0-9]{4}$`)
	mercosurPlate = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z][0-9]{2}$`)
)

// Plate é o objeto de valor de placa veicular (padrão antigo ou Mercosul), normalizada (RN02).
type Plate struct{ value string }

// NewPlate normaliza (maiúsculas, sem hífen, sem espaços nas bordas) e valida a placa.
func NewPlate(input string) (Plate, error) {
	v := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input), "-", ""))
	if !oldPlate.MatchString(v) && !mercosurPlate.MatchString(v) {
		return Plate{}, apperr.Validation("PLACA_INVALIDA", "placa inválida: use AAA-9999 ou AAA9A99")
	}
	return Plate{value: v}, nil
}

// String devolve a placa normalizada (formato de persistência).
func (p Plate) String() string { return p.value }

// IsMercosur informa se a placa segue o padrão Mercosul.
func (p Plate) IsMercosur() bool { return mercosurPlate.MatchString(p.value) }

// Formatted devolve a placa para exibição: AAA-9999 (antiga) ou AAA9A99 (Mercosul).
func (p Plate) Formatted() string {
	if p.value == "" || p.IsMercosur() {
		return p.value
	}
	return p.value[:3] + "-" + p.value[3:]
}
