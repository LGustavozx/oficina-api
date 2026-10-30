// Package httpserver monta o servidor HTTP: rotas transversais e ciclo de vida.
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Pinger é tudo que o health check precisa do banco. Usar uma interface
// pequena (em vez de *sql.DB) permite testar o handler sem banco real.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// NewHandler monta o roteador. Usa o ServeMux da biblioteca padrão, que
// desde o Go 1.22 aceita método na rota ("GET /health").
func NewHandler(db Pinger, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(db, logger))
	return mux
}

// New cria o *http.Server com timeouts. Sem timeouts, um cliente lento
// poderia manter conexões abertas para sempre (ataque "slowloris").
func New(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

type healthResponse struct {
	Status string `json:"status"`
}

// healthHandler responde 200 se a aplicação e o banco estão de pé, 503 se não.
// É o endpoint que orquestradores (Docker, Kubernetes) consultam.
func healthHandler(db Pinger, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status, code := "ok", http.StatusOK
		if err := db.PingContext(ctx); err != nil {
			logger.Error("health check: banco indisponível", "error", err)
			status, code = "unavailable", http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(healthResponse{Status: status})
	}
}
