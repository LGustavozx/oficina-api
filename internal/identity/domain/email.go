// Package domain do contexto identity: usuários que acessam a API administrativa.
package domain

import (
	"net/mail"
	"strings"

	"oficina-api/internal/shared/apperr"
)

const (
	CodeInvalidEmail = "INVALID_EMAIL"
	maxEmailLength   = 254
)

type Email struct {
	value string
}

func NewEmail(raw string) (Email, error) {
	value := strings.ToLower(strings.TrimSpace(raw))

	// ParseAddress aceita "Nome <a@b.com>"; comparar com o original rejeita esse formato.
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || len(value) > maxEmailLength {
		return Email{}, apperr.Validation(CodeInvalidEmail, "e-mail inválido")
	}
	return Email{value: value}, nil
}

func (e Email) String() string { return e.value }
