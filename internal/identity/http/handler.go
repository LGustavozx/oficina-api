// Package http do contexto identity: rotas de login e proteção das rotas administrativas.
package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"oficina-api/internal/identity/application"
	"oficina-api/internal/identity/domain"
	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
)

const (
	codeInvalidRequest = "INVALID_REQUEST"
	maxBodyBytes       = 1 << 20
)

type authenticator interface {
	Login(ctx context.Context, email, password string) (application.Token, error)
	Authenticate(ctx context.Context, token string) (domain.User, error)
}

type Handler struct {
	auth   authenticator
	logger *slog.Logger
}

func NewHandler(auth authenticator, logger *slog.Logger) *Handler {
	return &Handler{auth: auth, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.login)
	mux.Handle("GET /admin/me", h.RequireAuth(http.HandlerFunc(h.me)))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpserver.WriteError(w, h.logger, apperr.Validation(codeInvalidRequest, "corpo da requisição inválido"))
		return
	}

	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	httpserver.WriteJSON(w, http.StatusOK, loginResponse{Token: token.Value, ExpiresAt: token.ExpiresAt})
}

type meResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	httpserver.WriteJSON(w, http.StatusOK, meResponse{ID: user.ID.String(), Email: user.Email.String()})
}

type ctxKey struct{}

// RequireAuth exige "Authorization: Bearer <token>" e coloca o usuário no contexto.
// Toda rota /admin/* de qualquer contexto deve passar por ele.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			httpserver.WriteError(w, h.logger, apperr.Unauthorized(application.CodeInvalidToken, "token de acesso ausente ou inválido"))
			return
		}

		user, err := h.auth.Authenticate(r.Context(), token)
		if err != nil {
			httpserver.WriteError(w, h.logger, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

func UserFromContext(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value(ctxKey{}).(domain.User)
	return user, ok
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
