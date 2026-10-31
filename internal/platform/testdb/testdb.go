// Package testdb abre o banco usado nos testes de integração.
package testdb

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"oficina-api/internal/platform/database"
)

// Open conecta em TEST_DATABASE_URL e aplica as migrations. Sem a variável,
// o teste é pulado, para o go test comum não depender de um banco rodando.
func Open(t *testing.T) *sql.DB {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida; pulando teste de integração")
	}

	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
