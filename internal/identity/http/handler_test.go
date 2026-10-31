package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/identity/application"
	"oficina-api/internal/identity/domain"
	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
)

var testUser = func() domain.User {
	email, _ := domain.NewEmail("admin@oficina.com")
	return domain.User{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Email: email}
}()

type fakeAuth struct{}

func (fakeAuth) Login(_ context.Context, email, password string) (application.Token, error) {
	if email != "admin@oficina.com" || password != "senha-forte-123" {
		return application.Token{}, apperr.Unauthorized(application.CodeInvalidCredentials, "e-mail ou senha inválidos")
	}
	return application.Token{Value: "token-valido", ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func (fakeAuth) Authenticate(_ context.Context, token string) (domain.User, error) {
	if token != "token-valido" {
		return domain.User{}, apperr.Unauthorized(application.CodeInvalidToken, "token inválido")
	}
	return testUser, nil
}

type nopPinger struct{}

func (nopPinger) PingContext(context.Context) error { return nil }

func newServer() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpserver.NewHandler(nopPinger{}, logger, NewHandler(fakeAuth{}, logger).Register)
}

func do(t *testing.T, method, path, body, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	newServer().ServeHTTP(rec, req)
	return rec
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"sucesso", `{"email":"admin@oficina.com","password":"senha-forte-123"}`, 200, `"token":"token-valido"`},
		{"senha errada", `{"email":"admin@oficina.com","password":"errada"}`, 401, `"code":"INVALID_CREDENTIALS"`},
		{"JSON malformado", `{"email":`, 400, `"code":"INVALID_REQUEST"`},
		{"campo desconhecido", `{"email":"a@b.com","password":"x","admin":true}`, 400, `"code":"INVALID_REQUEST"`},
		{"corpo vazio", ``, 400, `"code":"INVALID_REQUEST"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, http.MethodPost, "/auth/login", tt.body, "")

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d (corpo: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("corpo = %s, esperado conter %s", rec.Body, tt.wantBody)
			}
		})
	}
}

func TestLoginRejectsGet(t *testing.T) {
	if rec := do(t, http.MethodGet, "/auth/login", "", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, esperado 405", rec.Code)
	}
}

func TestAdminMe(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"token válido", "Bearer token-valido", 200},
		{"esquema em minúsculas", "bearer token-valido", 200},
		{"sem cabeçalho", "", 401},
		{"token inválido", "Bearer outro", 401},
		{"esquema errado", "Basic token-valido", 401},
		{"sem token", "Bearer ", 401},
		{"só o token", "token-valido", 401},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, http.MethodGet, "/admin/me", "", tt.header)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == 200 {
				for _, want := range []string{`"id":"` + testUser.ID.String() + `"`, `"email":"admin@oficina.com"`} {
					if !strings.Contains(rec.Body.String(), want) {
						t.Errorf("corpo = %s, esperado conter %s", rec.Body, want)
					}
				}
			}
		})
	}
}

func TestUserFromContextWithoutUser(t *testing.T) {
	if _, ok := UserFromContext(context.Background()); ok {
		t.Error("não deveria haver usuário em um contexto vazio")
	}
}
