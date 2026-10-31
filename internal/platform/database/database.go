// Package database cuida da conexão com o PostgreSQL e da aplicação das migrations.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(ctx context.Context, url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("database: abrir conexão: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	// O goose espera os .sql na raiz do FS, e o embed os coloca em migrations/.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("database: localizar migrations: %w", err)
	}

	// O lock evita que duas instâncias (ou testes em paralelo) apliquem as
	// migrations ao mesmo tempo.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("database: criar lock de migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, sub, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("database: criar provider de migrations: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("database: aplicar migrations: %w", err)
	}

	return nil
}
