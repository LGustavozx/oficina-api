// Package http expõe o contexto Identidade e Acesso via API REST.
package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/LGustavozx/oficina-api/internal/identity/application"
	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/platform/httpx"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

// Handler agrupa as rotas de autenticação.
type Handler struct {
	authenticate *application.Authenticate
	tokens       domain.TokenIssuer
}

// NewHandler cria o handler.
func NewHandler(authenticate *application.Authenticate, tokens domain.TokenIssuer) *Handler {
	return &Handler{authenticate: authenticate, tokens: tokens}
}

// Register monta as rotas: login público e /admin/me protegida.
func (h *Handler) Register(public, admin chi.Router) {
	public.Post("/auth/login", h.login)
	admin.Get("/me", h.me)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"senha"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	token, err := h.authenticate.Execute(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, loginResponse{
		AccessToken: token.Value,
		TokenType:   "Bearer",
		ExpiresIn:   int(token.TTL / time.Second),
	})
}

type meResponse struct {
	UserID    string    `json:"usuario_id"`
	Role      string    `json:"perfil"`
	ExpiresAt time.Time `json:"expira_em"`
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		httpx.Error(w, apperr.Unauthorized("TOKEN_INVALIDO", "token de acesso inválido"))
		return
	}
	httpx.JSON(w, http.StatusOK, meResponse{UserID: claims.UserID, Role: string(claims.Role), ExpiresAt: claims.ExpiresAt})
}

type claimsKey struct{}

// ClaimsFromContext recupera as claims inseridas pelo middleware de autenticação.
func ClaimsFromContext(ctx context.Context) (domain.Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(domain.Claims)
	return c, ok
}

// Authentication devolve o middleware que exige um JWT válido no cabeçalho Authorization.
func (h *Handler) Authentication() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				deny(w, apperr.Unauthorized("TOKEN_AUSENTE", "cabeçalho Authorization Bearer é obrigatório"))
				return
			}
			claims, err := h.tokens.Validate(value)
			if err != nil {
				deny(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey{}, claims)))
		})
	}
}

func deny(w http.ResponseWriter, err error) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.Error(w, err)
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}
