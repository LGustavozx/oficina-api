// Package database cuida da conexão com o PostgreSQL e da aplicação das migrations.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	// Registra o driver "pgx" no database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// As migrations viajam dentro do binário (embed): a imagem Docker final
// não precisa carregar arquivos .sql soltos.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open abre o pool de conexões e confirma que o banco responde.
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

// Migrate aplica todas as migrations pendentes, em ordem.
func Migrate(ctx context.Context, db *sql.DB) error {
	// fs.Sub "entra" na pasta migrations para o goose enxergar os .sql na raiz.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("database: localizar migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, sub)
	if err != nil {
		return fmt.Errorf("database: criar provider de migrations: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("database: aplicar migrations: %w", err)
	}

	return nil
}
