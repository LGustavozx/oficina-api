package domain

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
)

const (
	CodeInvalidPassword = "INVALID_PASSWORD"
	CodeUserNotFound    = "USER_NOT_FOUND"
	CodeEmailTaken      = "EMAIL_ALREADY_REGISTERED"

	MinPasswordLength = 8
	// O bcrypt só considera os primeiros 72 bytes da senha.
	MaxPasswordBytes = 72
)

type User struct {
	ID           uuid.UUID
	Email        Email
	PasswordHash string
	CreatedAt    time.Time
}

func ValidatePassword(plain string) error {
	if utf8.RuneCountInString(plain) < MinPasswordLength || len(plain) > MaxPasswordBytes {
		return apperr.Validation(CodeInvalidPassword, "a senha deve ter entre 8 e 72 caracteres")
	}
	return nil
}

type UserRepository interface {
	Create(ctx context.Context, user User) error
	FindByEmail(ctx context.Context, email Email) (User, error)
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	Count(ctx context.Context) (int, error)
}
