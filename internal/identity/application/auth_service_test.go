package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/identity/domain"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

type fakeRepo struct {
	users     map[string]domain.User
	createErr error
	countErr  error
	findErr   error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{users: map[string]domain.User{}} }

func (r *fakeRepo) Create(_ context.Context, u domain.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.users[u.Email.String()] = u
	return nil
}

func (r *fakeRepo) FindByEmail(_ context.Context, e domain.Email) (domain.User, error) {
	if r.findErr != nil {
		return domain.User{}, r.findErr
	}
	u, ok := r.users[e.String()]
	if !ok {
		return domain.User{}, apperr.NotFound(domain.CodeUserNotFound, "usuário não encontrado")
	}
	return u, nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.User{}, apperr.NotFound(domain.CodeUserNotFound, "usuário não encontrado")
}

func (r *fakeRepo) Count(context.Context) (int, error) { return len(r.users), r.countErr }

type fakeHasher struct{ hashErr error }

func (h fakeHasher) Hash(plain string) (string, error) { return "hash:" + plain, h.hashErr }

func (fakeHasher) Compare(hash, plain string) error {
	if hash != "hash:"+plain {
		return errors.New("senha não confere")
	}
	return nil
}

type fakeTokens struct{}

func (fakeTokens) Issue(id uuid.UUID) (Token, error) {
	return Token{Value: "token:" + id.String(), ExpiresAt: time.Unix(1000, 0)}, nil
}

func (fakeTokens) Verify(token string) (uuid.UUID, error) {
	const prefix = "token:"
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return uuid.Nil, apperr.Unauthorized(CodeInvalidToken, "token inválido")
	}
	return uuid.Parse(token[len(prefix):])
}

func newService(t *testing.T, repo *fakeRepo) *AuthService {
	t.Helper()
	svc, err := NewAuthService(repo, fakeHasher{}, fakeTokens{}, clock.Fixed{Time: time.Unix(0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func seedUser(t *testing.T, repo *fakeRepo, email, password string) domain.User {
	t.Helper()
	e, err := domain.NewEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	u := domain.User{ID: uuid.New(), Email: e, PasswordHash: "hash:" + password}
	repo.users[e.String()] = u
	return u
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != code {
		t.Fatalf("erro = %v, esperado código %s", err, code)
	}
}

func TestLogin(t *testing.T) {
	repo := newFakeRepo()
	user := seedUser(t, repo, "admin@oficina.com", "senha-forte-123")
	svc := newService(t, repo)
	ctx := context.Background()

	t.Run("credenciais corretas", func(t *testing.T) {
		token, err := svc.Login(ctx, "Admin@Oficina.com", "senha-forte-123")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if token.Value != "token:"+user.ID.String() {
			t.Errorf("token = %q", token.Value)
		}
	})

	t.Run("senha errada", func(t *testing.T) {
		_, err := svc.Login(ctx, "admin@oficina.com", "outra-senha")
		wantCode(t, err, CodeInvalidCredentials)
	})

	t.Run("e-mail desconhecido tem o mesmo erro da senha errada", func(t *testing.T) {
		_, err := svc.Login(ctx, "ninguem@oficina.com", "senha-forte-123")
		wantCode(t, err, CodeInvalidCredentials)
	})

	t.Run("e-mail malformado", func(t *testing.T) {
		_, err := svc.Login(ctx, "isso-nao-e-email", "senha-forte-123")
		wantCode(t, err, CodeInvalidCredentials)
	})

	t.Run("falha do repositório não vira credencial inválida", func(t *testing.T) {
		quebrado := newFakeRepo()
		quebrado.findErr = errors.New("banco fora do ar")

		_, err := newService(t, quebrado).Login(ctx, "admin@oficina.com", "senha-forte-123")
		if err == nil {
			t.Fatal("esperado erro")
		}
		if _, ok := apperr.As(err); ok {
			t.Errorf("falha de infraestrutura não deveria virar erro de aplicação: %v", err)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	repo := newFakeRepo()
	user := seedUser(t, repo, "admin@oficina.com", "senha-forte-123")
	svc := newService(t, repo)
	ctx := context.Background()

	t.Run("token válido", func(t *testing.T) {
		got, err := svc.Authenticate(ctx, "token:"+user.ID.String())
		if err != nil || got.ID != user.ID {
			t.Fatalf("got %+v, err %v", got, err)
		}
	})

	t.Run("token inválido", func(t *testing.T) {
		_, err := svc.Authenticate(ctx, "lixo")
		wantCode(t, err, CodeInvalidToken)
	})

	t.Run("usuário removido depois de emitir o token", func(t *testing.T) {
		_, err := svc.Authenticate(ctx, "token:"+uuid.NewString())
		wantCode(t, err, CodeInvalidToken)
	})
}

func TestEnsureAdmin(t *testing.T) {
	ctx := context.Background()

	t.Run("cria quando não há usuários", func(t *testing.T) {
		repo := newFakeRepo()
		created, err := newService(t, repo).EnsureAdmin(ctx, "Admin@Oficina.com", "senha-forte-123")
		if err != nil || !created {
			t.Fatalf("created=%v err=%v", created, err)
		}
		if _, ok := repo.users["admin@oficina.com"]; !ok {
			t.Error("usuário não foi gravado com e-mail normalizado")
		}
	})

	t.Run("não faz nada quando já existe usuário", func(t *testing.T) {
		repo := newFakeRepo()
		seedUser(t, repo, "outro@oficina.com", "senha-forte-123")
		created, err := newService(t, repo).EnsureAdmin(ctx, "admin@oficina.com", "senha-forte-123")
		if err != nil || created {
			t.Fatalf("created=%v err=%v", created, err)
		}
		if len(repo.users) != 1 {
			t.Error("não deveria criar outro usuário")
		}
	})

	t.Run("e-mail inválido", func(t *testing.T) {
		_, err := newService(t, newFakeRepo()).EnsureAdmin(ctx, "x", "senha-forte-123")
		wantCode(t, err, domain.CodeInvalidEmail)
	})

	t.Run("senha fraca", func(t *testing.T) {
		_, err := newService(t, newFakeRepo()).EnsureAdmin(ctx, "admin@oficina.com", "123")
		wantCode(t, err, domain.CodeInvalidPassword)
	})

	t.Run("conflito na criação é ignorado", func(t *testing.T) {
		repo := newFakeRepo()
		repo.createErr = apperr.Conflict(domain.CodeEmailTaken, "e-mail já cadastrado")
		created, err := newService(t, repo).EnsureAdmin(ctx, "admin@oficina.com", "senha-forte-123")
		if err != nil || created {
			t.Fatalf("created=%v err=%v", created, err)
		}
	})

	t.Run("outro erro na criação é propagado", func(t *testing.T) {
		repo := newFakeRepo()
		repo.createErr = errors.New("banco fora do ar")
		if _, err := newService(t, repo).EnsureAdmin(ctx, "admin@oficina.com", "senha-forte-123"); err == nil {
			t.Fatal("esperado erro")
		}
	})

	t.Run("erro ao contar usuários é propagado", func(t *testing.T) {
		repo := newFakeRepo()
		repo.countErr = errors.New("banco fora do ar")
		if _, err := newService(t, repo).EnsureAdmin(ctx, "admin@oficina.com", "senha-forte-123"); err == nil {
			t.Fatal("esperado erro")
		}
	})
}

func TestNewAuthServiceHashError(t *testing.T) {
	_, err := NewAuthService(newFakeRepo(), fakeHasher{hashErr: errors.New("falha")}, fakeTokens{}, clock.Fixed{})
	if err == nil {
		t.Fatal("esperado erro quando o hasher falha")
	}
}
