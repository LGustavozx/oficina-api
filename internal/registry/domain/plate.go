package domain

import (
	"regexp"
	"strings"

	"oficina-api/internal/shared/apperr"
)

const CodeInvalidPlate = "INVALID_PLATE"

// Cobre o formato antigo (ABC1234) e o Mercosul (ABC1D23).
var platePattern = regexp.MustCompile(`^[A-Z]{3}[0-9][0-9A-Z][0-9]{2}$`)

// Plate é uma placa válida, em maiúsculas e sem hífen.
type Plate struct {
	value string
}

func NewPlate(raw string) (Plate, error) {
	value := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(raw))

	if !platePattern.MatchString(value) {
		return Plate{}, apperr.Validation(CodeInvalidPlate, "placa inválida")
	}
	return Plate{value: value}, nil
}

func (p Plate) String() string { return p.value }
