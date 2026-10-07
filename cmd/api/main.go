// Comando api: ponto de entrada e composição das dependências do monolito.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	identityapp "github.com/LGustavozx/oficina-api/internal/identity/application"
	identityhttp "github.com/LGustavozx/oficina-api/internal/identity/http"
	identitypg "github.com/LGustavozx/oficina-api/internal/identity/infrastructure/postgres"
	"github.com/LGustavozx/oficina-api/internal/identity/infrastructure/security"
	"github.com/LGustavozx/oficina-api/internal/platform/database"
	"github.com/LGustavozx/oficina-api/internal/platform/httpserver"
	"github.com/LGustavozx/oficina-api/internal/shared/clock"
	"github.com/LGustavozx/oficina-api/internal/shared/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("falha ao executar a aplicação", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(pool); err != nil {
		return err
	}
	logger.Info("migrations aplicadas")

	identityHandler, err := wireIdentity(ctx, cfg, pool, logger)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpserver.NewRouter(httpserver.Config{
			Logger:         logger,
			DB:             pool,
			Authentication: identityHandler.Authentication(),
			Modules:        []httpserver.Module{identityHandler},
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("servidor iniciado", "porta", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("encerrando servidor")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// wireIdentity compõe o contexto de identidade e garante o administrador inicial (seed).
func wireIdentity(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*identityhttp.Handler, error) {
	hasher := security.NewBcrypt(0) // custo padrão do bcrypt
	issuer, err := security.NewJWT(cfg.JWTSecret, time.Duration(cfg.JWTTTLMinutes)*time.Minute, clock.System{})
	if err != nil {
		return nil, err
	}
	repo := identitypg.NewUserRepository(pool)

	authenticate, err := identityapp.NewAuthenticate(repo, hasher, issuer)
	if err != nil {
		return nil, err
	}

	if cfg.AdminEmail != "" {
		ensureAdmin := identityapp.NewEnsureAdmin(repo, hasher, clock.System{}, uuid.NewString)
		created, err := ensureAdmin.Execute(ctx, cfg.AdminEmail, cfg.AdminPassword)
		if err != nil {
			return nil, fmt.Errorf("criar administrador inicial: %w", err)
		}
		logger.Info("administrador inicial verificado", "criado", created)
	}

	return identityhttp.NewHandler(authenticate, issuer), nil
}

func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}
