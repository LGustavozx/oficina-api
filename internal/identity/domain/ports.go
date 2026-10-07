package domain

import (
	"context"
	"time"
)

// UserRepository persiste usuários.
// FindByEmail devolve erro de categoria "não encontrado" quando não existe;
// Save devolve erro de categoria "conflito" quando o e-mail já está em uso.
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*User, error)
	Save(ctx context.Context, u *User) error
}

// PasswordHasher abstrai o algoritmo de hash de senhas.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// Token é um token de acesso emitido.
type Token struct {
	Value     string
	ExpiresAt time.Time
	TTL       time.Duration
}

// Claims são as informações extraídas de um token válido.
type Claims struct {
	UserID    string
	Role      Role
	ExpiresAt time.Time
}

// TokenIssuer emite e valida tokens de acesso.
// Validate devolve erro de categoria "não autorizado" para tokens inválidos ou expirados.
type TokenIssuer interface {
	Issue(u *User) (Token, error)
	Validate(token string) (Claims, error)
}
