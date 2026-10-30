// Comando api: ponto de entrada da aplicação. Só "costura" as peças
// (config, banco, servidor); nenhuma regra de negócio mora aqui.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oficina-api/internal/platform/config"
	"oficina-api/internal/platform/database"
	"oficina-api/internal/platform/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("aplicação encerrada com erro", "error", err)
		os.Exit(1)
	}
}

// run existe separado de main para podermos devolver erros (main não devolve)
// e garantir que os defers rodem antes do os.Exit.
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx é cancelado quando o processo recebe Ctrl+C ou SIGTERM (docker stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("fechar conexão com o banco", "error", err)
		}
	}()

	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	logger.Info("migrations aplicadas")

	server := httpserver.New(cfg.HTTPPort, httpserver.NewHandler(db, logger))

	// O servidor roda em outra goroutine para a main poder esperar o sinal de parada.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("servidor iniciado", "port", cfg.HTTPPort)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("sinal de parada recebido, encerrando")
	}

	// Graceful shutdown: para de aceitar conexões novas e dá até 10s
	// para as requisições em andamento terminarem.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
