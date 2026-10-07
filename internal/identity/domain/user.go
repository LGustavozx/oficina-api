// Package domain contém as regras do contexto Identidade e Acesso.
package domain

import (
	"net/mail"
	"strings"
	"time"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

// Role define o nível de acesso de um usuário.
type Role string

// RoleAdmin é o único perfil do MVP (atendente, mecânico e administrador).
const RoleAdmin Role = "ADMIN"

const (
	minPasswordLen = 8
	// maxPasswordLen respeita o limite de 72 bytes do bcrypt.
	maxPasswordLen = 72
	maxEmailLen    = 254
)

// User é o agregado raiz do contexto de identidade.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
	Active       bool
	CreatedAt    time.Time
}

// NewUser cria um usuário ativo, normalizando e validando o e-mail.
func NewUser(id, email, passwordHash string, role Role, createdAt time.Time) (*User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if id == "" || passwordHash == "" {
		return nil, apperr.Validation("USUARIO_INVALIDO", "identificador e hash de senha são obrigatórios")
	}
	if role != RoleAdmin {
		return nil, apperr.Validation("PERFIL_INVALIDO", "perfil de usuário inválido")
	}
	return &User{
		ID:           id,
		Email:        normalized,
		PasswordHash: passwordHash,
		Role:         role,
		Active:       true,
		CreatedAt:    createdAt,
	}, nil
}

// NormalizeEmail remove espaços, converte para minúsculas e valida o formato.
func NormalizeEmail(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" || len(e) > maxEmailLen {
		return "", apperr.Validation("EMAIL_INVALIDO", "e-mail inválido")
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", apperr.Validation("EMAIL_INVALIDO", "e-mail inválido")
	}
	return e, nil
}

// ValidatePassword aplica a política de senha (8 a 72 bytes).
func ValidatePassword(password string) error {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return apperr.Validation("SENHA_INVALIDA", "a senha deve ter entre 8 e 72 caracteres")
	}
	return nil
}
