// Package application contém os casos de uso do contexto Identidade e Acesso.
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

// dummyPassword alimenta o hash usado para equalizar o tempo de resposta
// quando o usuário não existe (mitiga enumeração de contas por tempo).
//
// Valor fictício e público: não é uma credencial real (falso positivo do gosec G101).
const dummyPassword = "senha-ficticia-para-equalizar-tempo" //nolint:gosec

// Authenticate valida credenciais e emite o token de acesso.
type Authenticate struct {
	repo      domain.UserRepository
	hasher    domain.PasswordHasher
	tokens    domain.TokenIssuer
	dummyHash string
}

// NewAuthenticate cria o caso de uso.
func NewAuthenticate(repo domain.UserRepository, hasher domain.PasswordHasher, tokens domain.TokenIssuer) (*Authenticate, error) {
	h, err := hasher.Hash(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("gerar hash fictício: %w", err)
	}
	return &Authenticate{repo: repo, hasher: hasher, tokens: tokens, dummyHash: h}, nil
}

func invalidCredentials() error {
	return apperr.Unauthorized("CREDENCIAIS_INVALIDAS", "credenciais inválidas")
}

// Execute autentica por e-mail e senha. Qualquer falha de credencial devolve o
// mesmo erro, sem indicar se o e-mail existe.
func (a *Authenticate) Execute(ctx context.Context, email, password string) (domain.Token, error) {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		a.hasher.Compare(a.dummyHash, password)
		return domain.Token{}, invalidCredentials()
	}

	user, err := a.repo.FindByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			a.hasher.Compare(a.dummyHash, password)
			return domain.Token{}, invalidCredentials()
		}
		return domain.Token{}, fmt.Errorf("buscar usuário: %w", err)
	}

	if !a.hasher.Compare(user.PasswordHash, password) || !user.Active {
		return domain.Token{}, invalidCredentials()
	}

	token, err := a.tokens.Issue(user)
	if err != nil {
		return domain.Token{}, fmt.Errorf("emitir token: %w", err)
	}
	return token, nil
}
