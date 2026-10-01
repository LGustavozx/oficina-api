// Package httpserver monta o roteador HTTP base e os endpoints operacionais.
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Pinger verifica a disponibilidade de uma dependência (ex.: banco de dados).
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter cria o roteador com middlewares globais e rotas operacionais.
func NewRouter(logger *slog.Logger, db Pinger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.SetHeader("X-Content-Type-Options", "nosniff"))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponivel"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			inicio := time.Now()
			next.ServeHTTP(ww, r)
			logger.Info("requisição",
				"request_id", middleware.GetReqID(r.Context()),
				"metodo", r.Method,
				"caminho", r.URL.Path,
				"status", ww.Status(),
				"duracao_ms", time.Since(inicio).Milliseconds(),
			)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
