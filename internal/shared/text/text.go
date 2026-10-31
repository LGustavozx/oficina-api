// Package text valida textos de entrada.
package text

import (
	"strings"
	"unicode/utf8"
)

// Clean apara o texto e confere o tamanho em caracteres. Recusa U+FFFD, que o
// decoder de JSON coloca no lugar de bytes que não são UTF-8.
func Clean(raw string, minLength, maxLength int) (string, bool) {
	value := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(value)
	if n < minLength || n > maxLength || strings.ContainsRune(value, utf8.RuneError) {
		return "", false
	}
	return value, true
}
