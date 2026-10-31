package application

import "strings"

// normalizeSKU deixa a busca por código igual ao formato gravado: maiúsculas, sem espaços nas pontas.
func normalizeSKU(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}
