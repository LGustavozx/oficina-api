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

	"golang.org/x/crypto/bcrypt"

	identityapp "oficina-api/internal/identity/application"
	identityhttp "oficina-api/internal/identity/http"
	identityinfra "oficina-api/internal/identity/infrastructure"
	"oficina-api/internal/platform/config"
	"oficina-api/internal/platform/database"
	"oficina-api/internal/platform/httpserver"
	registryapp "oficina-api/internal/registry/application"
	registryhttp "oficina-api/internal/registry/http"
	registryinfra "oficina-api/internal/registry/infrastructure"
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

	clk := clock.System{}
	auth, err := identityapp.NewAuthService(
		identityinfra.NewUserRepository(db),
		identityinfra.NewBcryptHasher(bcrypt.DefaultCost),
		identityinfra.NewJWTManager(cfg.JWTSecret, cfg.JWTTTL, clk),
		clk,
	)
	if err != nil {
		return err
	}

	if cfg.AdminEmail != "" {
		created, err := auth.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword)
		if err != nil {
			return err
		}
		if created {
			logger.Info("administrador inicial criado", "email", cfg.AdminEmail)
		}
	}

	identityHandler := identityhttp.NewHandler(auth, logger)

	customers := registryapp.NewCustomerService(registryinfra.NewCustomerRepository(db), clk)
	customerHandler := registryhttp.NewCustomerHandler(customers, logger)

	vehicles := registryapp.NewVehicleService(registryinfra.NewVehicleRepository(db), clk)
	vehicleHandler := registryhttp.NewVehicleHandler(vehicles, logger)

	handler := httpserver.NewHandler(db, logger,
		identityHandler.Register,
		func(mux *http.ServeMux) { customerHandler.Register(mux, identityHandler.RequireAuth) },
		func(mux *http.ServeMux) { vehicleHandler.Register(mux, identityHandler.RequireAuth) },
	)
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
