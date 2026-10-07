// Package httpserver monta o roteador HTTP base, os endpoints operacionais e o registro de módulos.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/LGustavozx/oficina-api/internal/platform/httpx"
)

// Pinger verifica a disponibilidade de uma dependência (ex.: banco de dados).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Module é um contexto da aplicação que registra suas rotas.
// Rotas em public não exigem autenticação; rotas em admin passam pelo middleware JWT.
type Module interface {
	Register(public, admin chi.Router)
}

// Config reúne as dependências do roteador.
type Config struct {
	Logger *slog.Logger
	DB     Pinger
	// Authentication protege todas as rotas administrativas; é obrigatório.
	Authentication func(http.Handler) http.Handler
	Modules        []Module
}

// NewRouter cria o roteador com middlewares globais, rotas operacionais e módulos em /api/v1.
// Rotas administrativas ficam em /api/v1/admin e exigem autenticação.
func NewRouter(cfg Config) *chi.Mux {
	if cfg.Authentication == nil {
		panic("httpserver: Config.Authentication é obrigatório")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(cfg.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.SetHeader("X-Content-Type-Options", "nosniff"))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := cfg.DB.Ping(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponivel"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	api := chi.NewRouter()
	admin := chi.NewRouter()
	admin.Use(cfg.Authentication)
	for _, m := range cfg.Modules {
		m.Register(api, admin)
	}
	api.Mount("/admin", admin)
	r.Mount("/api/v1", api)

	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			logger.Info("requisição",
				"request_id", middleware.GetReqID(r.Context()),
				"metodo", r.Method,
				"caminho", r.URL.Path,
				"status", ww.Status(),
				"duracao_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
