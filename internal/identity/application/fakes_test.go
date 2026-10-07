package application

import (
	"context"
	"time"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

// fakeRepo é um repositório em memória com injeção de falhas.
type fakeRepo struct {
	users   map[string]*domain.User
	findErr error
	saveErr error
	saved   int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{users: map[string]*domain.User{}} }

func (r *fakeRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	u, ok := r.users[email]
	if !ok {
		return nil, apperr.NotFound("USUARIO_NAO_ENCONTRADO", "usuário não encontrado")
	}
	return u, nil
}

func (r *fakeRepo) Save(_ context.Context, u *domain.User) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.users[u.Email] = u
	r.saved++
	return nil
}

// fakeHasher usa um "hash" previsível e conta as comparações.
type fakeHasher struct {
	hashErr error
	compare int
}

func (h *fakeHasher) Hash(password string) (string, error) {
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return "hash:" + password, nil
}

func (h *fakeHasher) Compare(hash, password string) bool {
	h.compare++
	return hash == "hash:"+password
}

// fakeIssuer devolve um token fixo ou falha sob demanda.
type fakeIssuer struct{ err error }

func (i fakeIssuer) Issue(u *domain.User) (domain.Token, error) {
	if i.err != nil {
		return domain.Token{}, i.err
	}
	return domain.Token{Value: "token-" + u.ID, TTL: time.Hour}, nil
}

func (fakeIssuer) Validate(string) (domain.Claims, error) { return domain.Claims{}, nil }
