package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
	"github.com/LGustavozx/oficina-api/internal/shared/clock"
)

// EnsureAdmin cria o usuário administrador inicial (seed) caso não exista.
type EnsureAdmin struct {
	repo   domain.UserRepository
	hasher domain.PasswordHasher
	clock  clock.Clock
	newID  func() string
}

// NewEnsureAdmin cria o caso de uso.
func NewEnsureAdmin(repo domain.UserRepository, hasher domain.PasswordHasher, c clock.Clock, newID func() string) *EnsureAdmin {
	return &EnsureAdmin{repo: repo, hasher: hasher, clock: c, newID: newID}
}

// Execute cria o administrador se o e-mail ainda não existe. É idempotente:
// devolve created=false quando o usuário já existe, sem alterar a senha.
func (e *EnsureAdmin) Execute(ctx context.Context, email, password string) (created bool, err error) {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return false, err
	}

	_, err = e.repo.FindByEmail(ctx, normalized)
	switch {
	case err == nil:
		return false, nil
	case !errors.Is(err, apperr.ErrNotFound):
		return false, fmt.Errorf("buscar administrador: %w", err)
	}

	if err := domain.ValidatePassword(password); err != nil {
		return false, err
	}
	hash, err := e.hasher.Hash(password)
	if err != nil {
		return false, fmt.Errorf("gerar hash: %w", err)
	}
	user, err := domain.NewUser(e.newID(), normalized, hash, domain.RoleAdmin, e.clock.Now())
	if err != nil {
		return false, err
	}
	if err := e.repo.Save(ctx, user); err != nil {
		return false, fmt.Errorf("salvar administrador: %w", err)
	}
	return true, nil
}
