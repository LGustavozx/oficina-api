package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/LGustavozx/oficina-api/internal/identity/application"
	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/identity/infrastructure/security"
	"github.com/LGustavozx/oficina-api/internal/platform/httpserver"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
	"github.com/LGustavozx/oficina-api/internal/shared/clock"
)

const (
	testEmail      = "admin@oficina.local"
	testPassword   = "segredo123"
	testSigningKey = "um-segredo-de-teste-com-mais-de-32-bytes"
)

var testNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

type memoryRepo struct{ users map[string]*domain.User }

func (r *memoryRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if u, ok := r.users[email]; ok {
		return u, nil
	}
	return nil, apperr.NotFound("USUARIO_NAO_ENCONTRADO", "usuário não encontrado")
}

func (r *memoryRepo) Save(_ context.Context, u *domain.User) error {
	r.users[u.Email] = u
	return nil
}

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

type server struct {
	handler nethttp.Handler
	clock   *clock.Fixed
}

// newServer monta o roteador real com o módulo de identidade e um administrador de teste.
func newServer(t *testing.T) server {
	t.Helper()
	clk := &clock.Fixed{Instant: testNow}
	hasher := security.NewBcrypt(bcrypt.MinCost)
	repo := &memoryRepo{users: map[string]*domain.User{}}

	issuer, err := security.NewJWT(testSigningKey, time.Hour, clk)
	if err != nil {
		t.Fatal(err)
	}
	authenticate, err := application.NewAuthenticate(repo, hasher, issuer)
	if err != nil {
		t.Fatal(err)
	}
	ensureAdmin := application.NewEnsureAdmin(repo, hasher, clk, func() string { return "admin-1" })
	if _, err := ensureAdmin.Execute(context.Background(), testEmail, testPassword); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(authenticate, issuer)
	router := httpserver.NewRouter(httpserver.Config{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		DB:             okPinger{},
		Authentication: h.Authentication(),
		Modules:        []httpserver.Module{h},
	})
	return server{handler: router, clock: clk}
}

func (s server) do(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func (s server) login(t *testing.T) string {
	t.Helper()
	rec := s.do(nethttp.MethodPost, "/api/v1/auth/login", `{"email":"`+testEmail+`","senha":"`+testPassword+`"}`, nil)
	if rec.Code != nethttp.StatusOK {
		t.Fatalf("login falhou: %d %s", rec.Code, rec.Body)
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.AccessToken
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"codigo"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo não é JSON de erro: %s", rec.Body)
	}
	return body.Code
}

func TestLogin_Success(t *testing.T) {
	s := newServer(t)
	rec := s.do(nethttp.MethodPost, "/api/v1/auth/login", `{"email":" ADMIN@oficina.local ","senha":"`+testPassword+`"}`, nil)
	if rec.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body)
	}
	var resp loginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.AccessToken == "" || resp.TokenType != "Bearer" || resp.ExpiresIn != 3600 {
		t.Errorf("resposta = %+v", resp)
	}
}

func TestLogin_Failures(t *testing.T) {
	s := newServer(t)
	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"senha errada", `{"email":"admin@oficina.local","senha":"errada123"}`, nethttp.StatusUnauthorized, "CREDENCIAIS_INVALIDAS"},
		{"usuário inexistente", `{"email":"x@oficina.local","senha":"segredo123"}`, nethttp.StatusUnauthorized, "CREDENCIAIS_INVALIDAS"},
		{"campos vazios", `{}`, nethttp.StatusUnauthorized, "CREDENCIAIS_INVALIDAS"},
		{"json inválido", `{`, nethttp.StatusUnprocessableEntity, "JSON_INVALIDO"},
		{"campo desconhecido", `{"email":"a@b.com","senha":"12345678","perfil":"ADMIN"}`, nethttp.StatusUnprocessableEntity, "JSON_INVALIDO"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := s.do(nethttp.MethodPost, "/api/v1/auth/login", c.body, nil)
			if rec.Code != c.status || errorCode(t, rec) != c.code {
				t.Errorf("status = %d, código = %q; esperado %d, %q", rec.Code, errorCode(t, rec), c.status, c.code)
			}
		})
	}
}

// Credenciais inválidas não podem revelar se o e-mail existe.
func TestLogin_NoAccountEnumeration(t *testing.T) {
	s := newServer(t)
	wrongPassword := s.do(nethttp.MethodPost, "/api/v1/auth/login", `{"email":"admin@oficina.local","senha":"errada123"}`, nil)
	unknownUser := s.do(nethttp.MethodPost, "/api/v1/auth/login", `{"email":"x@oficina.local","senha":"errada123"}`, nil)
	if wrongPassword.Code != unknownUser.Code || wrongPassword.Body.String() != unknownUser.Body.String() {
		t.Errorf("respostas distintas:\n%d %s\n%d %s", wrongPassword.Code, wrongPassword.Body, unknownUser.Code, unknownUser.Body)
	}
}

func TestAdminRoute_RequiresValidToken(t *testing.T) {
	s := newServer(t)

	t.Run("sem cabeçalho", func(t *testing.T) {
		rec := s.do(nethttp.MethodGet, "/api/v1/admin/me", "", nil)
		if rec.Code != nethttp.StatusUnauthorized || errorCode(t, rec) != "TOKEN_AUSENTE" {
			t.Errorf("status = %d, código = %q", rec.Code, errorCode(t, rec))
		}
		if rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Error("faltou o cabeçalho WWW-Authenticate")
		}
	})
	for name, header := range map[string]string{
		"esquema errado": "Basic abc",
		"bearer vazio":   "Bearer ",
		"só o esquema":   "Bearer",
		"token inválido": "Bearer abc.def.ghi",
	} {
		t.Run(name, func(t *testing.T) {
			rec := s.do(nethttp.MethodGet, "/api/v1/admin/me", "", map[string]string{"Authorization": header})
			if rec.Code != nethttp.StatusUnauthorized {
				t.Errorf("status = %d, esperado 401", rec.Code)
			}
		})
	}
	t.Run("token expirado", func(t *testing.T) {
		token := s.login(t)
		s.clock.Advance(2 * time.Hour)
		rec := s.do(nethttp.MethodGet, "/api/v1/admin/me", "", map[string]string{"Authorization": "Bearer " + token})
		if rec.Code != nethttp.StatusUnauthorized || errorCode(t, rec) != "TOKEN_EXPIRADO" {
			t.Errorf("status = %d, código = %q", rec.Code, errorCode(t, rec))
		}
	})
}

func TestAdminMe_WithValidToken(t *testing.T) {
	s := newServer(t)
	for _, scheme := range []string{"Bearer", "bearer"} {
		rec := s.do(nethttp.MethodGet, "/api/v1/admin/me", "", map[string]string{"Authorization": scheme + " " + s.login(t)})
		if rec.Code != nethttp.StatusOK {
			t.Fatalf("%s: status = %d, corpo = %s", scheme, rec.Code, rec.Body)
		}
		var me meResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &me)
		if me.UserID != "admin-1" || me.Role != "ADMIN" {
			t.Errorf("%s: resposta = %+v", scheme, me)
		}
	}
}

func TestMe_WithoutClaimsInContext(t *testing.T) {
	// Defesa em profundidade: se o handler for chamado sem o middleware, responde 401.
	h := &Handler{}
	rec := httptest.NewRecorder()
	h.me(rec, httptest.NewRequest(nethttp.MethodGet, "/", nil))
	if rec.Code != nethttp.StatusUnauthorized {
		t.Errorf("status = %d, esperado 401", rec.Code)
	}
}
