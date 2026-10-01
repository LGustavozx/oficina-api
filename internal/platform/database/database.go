// Package database provê a conexão com o PostgreSQL e a aplicação das migrations.
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/LGustavozx/oficina-api/migrations"
)

// Connect abre o pool de conexões e valida a disponibilidade do banco.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("criar pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping no banco: %w", err)
	}
	return pool, nil
}

// Migrate aplica todas as migrations pendentes (embutidas no binário).
func Migrate(pool *pgxpool.Pool) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("carregar migrations: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close() // não fecha o pool, apenas libera o *sql.DB

	drv, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("driver de migration: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", drv)
	if err != nil {
		return fmt.Errorf("criar migrator: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("aplicar migrations: %w", err)
	}
	return nil
}
