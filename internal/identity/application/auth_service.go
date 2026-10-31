// Package application do contexto identity: casos de uso de autenticação.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/identity/domain"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

const (
	CodeInvalidCredentials = "INVALID_CREDENTIALS" //nolint:gosec // código de erro, não é uma credencial
	CodeInvalidToken       = "INVALID_TOKEN"
)

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}

type Token struct {
	Value     string
	ExpiresAt time.Time
}

type TokenManager interface {
	Issue(userID uuid.UUID) (Token, error)
	Verify(token string) (uuid.UUID, error)
}

type AuthService struct {
	users  domain.UserRepository
	hasher PasswordHasher
	tokens TokenManager
	clock  clock.Clock

	// Hash descartável comparado quando o e-mail não existe, para o tempo de
	// resposta não revelar quais e-mails estão cadastrados.
	dummyHash string
}

func NewAuthService(users domain.UserRepository, hasher PasswordHasher, tokens TokenManager, clk clock.Clock) (*AuthService, error) {
	dummyHash, err := hasher.Hash("senha-descartavel")
	if err != nil {
		return nil, err
	}
	return &AuthService{users: users, hasher: hasher, tokens: tokens, clock: clk, dummyHash: dummyHash}, nil
}

// Login devolve o mesmo erro para e-mail desconhecido e senha errada.
func (s *AuthService) Login(ctx context.Context, rawEmail, password string) (Token, error) {
	user, found, err := s.findByEmail(ctx, rawEmail)
	if err != nil {
		return Token{}, err
	}

	if !found {
		_ = s.hasher.Compare(s.dummyHash, password)
		return Token{}, invalidCredentials()
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return Token{}, invalidCredentials()
	}

	return s.tokens.Issue(user.ID)
}

// Authenticate valida o token e confirma que o usuário ainda existe.
func (s *AuthService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	id, err := s.tokens.Verify(token)
	if err != nil {
		return domain.User{}, err
	}

	user, err := s.users.FindByID(ctx, id)
	if appErr, ok := apperr.As(err); ok && appErr.Kind == apperr.KindNotFound {
		return domain.User{}, invalidToken()
	}
	return user, err
}

// EnsureAdmin cria o administrador inicial se ainda não houver nenhum usuário.
// Devolve true quando criou.
func (s *AuthService) EnsureAdmin(ctx context.Context, rawEmail, password string) (bool, error) {
	count, err := s.users.Count(ctx)
	if err != nil || count > 0 {
		return false, err
	}

	email, err := domain.NewEmail(rawEmail)
	if err != nil {
		return false, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return false, err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return false, err
	}

	user := domain.User{ID: uuid.New(), Email: email, PasswordHash: hash, CreatedAt: s.clock.Now()}
	if err := s.users.Create(ctx, user); err != nil {
		// Outra instância criou o mesmo admin ao mesmo tempo.
		if appErr, ok := apperr.As(err); ok && appErr.Kind == apperr.KindConflict {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *AuthService) findByEmail(ctx context.Context, rawEmail string) (domain.User, bool, error) {
	email, err := domain.NewEmail(rawEmail)
	if err != nil {
		return domain.User{}, false, nil
	}

	user, err := s.users.FindByEmail(ctx, email)
	if appErr, ok := apperr.As(err); ok && appErr.Kind == apperr.KindNotFound {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, err
	}
	return user, true, nil
}

func invalidCredentials() error {
	return apperr.Unauthorized(CodeInvalidCredentials, "e-mail ou senha inválidos")
}

func invalidToken() error {
	return apperr.Unauthorized(CodeInvalidToken, "token de acesso ausente ou inválido")
}
