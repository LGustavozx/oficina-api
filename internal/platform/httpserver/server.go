// Package httpserver monta o servidor HTTP e as rotas transversais.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger é uma interface mínima para o health check ser testável sem banco real.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Registrar permite que cada contexto registre suas próprias rotas.
type Registrar func(mux *http.ServeMux)

func NewHandler(db Pinger, logger *slog.Logger, registrars ...Registrar) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(db, logger))
	for _, register := range registrars {
		register(mux)
	}
	return requestLogger(logger, mux)
}

// Os timeouts evitam que clientes lentos mantenham conexões abertas indefinidamente.
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

func healthHandler(db Pinger, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status, code := "ok", http.StatusOK
		if err := db.PingContext(ctx); err != nil {
			logger.Error("health check: banco indisponível", "error", err)
			status, code = "unavailable", http.StatusServiceUnavailable
		}

		WriteJSON(w, code, healthResponse{Status: status})
	}
}
