package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

// fakeModule registra uma rota pública e uma administrativa.
type fakeModule struct{}

func (fakeModule) Register(public, admin chi.Router) {
	public.Get("/aberta", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	admin.Get("/secreta", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

// requireHeader é um middleware de teste: exige o cabeçalho X-Auth.
func requireHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newRouter(db Pinger) http.Handler {
	return NewRouter(Config{
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		DB:             db,
		Authentication: requireHeader,
		Modules:        []Module{fakeModule{}},
	})
}

func do(h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	if rec := do(newRouter(fakePinger{}), "/health", nil); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", rec.Code)
	}
}

func TestReady(t *testing.T) {
	if rec := do(newRouter(fakePinger{}), "/ready", nil); rec.Code != http.StatusOK {
		t.Errorf("banco ok: status = %d, esperado 200", rec.Code)
	}
	if rec := do(newRouter(fakePinger{err: errors.New("fora")}), "/ready", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("banco fora: status = %d, esperado 503", rec.Code)
	}
}

func TestPublicAndAdminRoutes(t *testing.T) {
	h := newRouter(fakePinger{})

	if rec := do(h, "/api/v1/aberta", nil); rec.Code != http.StatusOK {
		t.Errorf("rota pública sem auth: status = %d, esperado 200", rec.Code)
	}
	if rec := do(h, "/api/v1/admin/secreta", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("rota admin sem auth: status = %d, esperado 401", rec.Code)
	}
	if rec := do(h, "/api/v1/admin/secreta", map[string]string{"X-Auth": "ok"}); rec.Code != http.StatusOK {
		t.Errorf("rota admin com auth: status = %d, esperado 200", rec.Code)
	}
}

// Rotas administrativas inexistentes também exigem autenticação (não revelam a estrutura da API).
func TestUnknownAdminRouteRequiresAuthentication(t *testing.T) {
	if rec := do(newRouter(fakePinger{}), "/api/v1/admin/nao-existe", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado 401", rec.Code)
	}
}

func TestNewRouter_PanicsWithoutAuthentication(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("esperava panic quando Authentication é nulo")
		}
	}()
	NewRouter(Config{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), DB: fakePinger{}})
}
