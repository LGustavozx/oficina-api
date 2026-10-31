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

	"oficina-api/internal/app"
	"oficina-api/internal/platform/config"
	"oficina-api/internal/platform/database"
	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/clock"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("aplicação encerrada com erro", "error", err)
		os.Exit(1)
	}
}

// run fica separado de main para os defers rodarem antes do os.Exit.
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

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

	handler, err := app.NewHandler(ctx, db, cfg, logger, clock.System{})
	if err != nil {
		return err
	}
	server := httpserver.New(cfg.HTTPPort, handler)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
