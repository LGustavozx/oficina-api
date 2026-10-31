package database_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"oficina-api/internal/platform/database"
	"oficina-api/internal/platform/testdb"
)

func TestOpenFailsWhenDatabaseIsUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.Open(ctx, "postgres://oficina:oficina@127.0.0.1:1/oficina?sslmode=disable&connect_timeout=1")

	if err == nil {
		_ = db.Close()
		t.Fatal("esperado erro ao conectar em um banco inexistente")
	}
}

func TestOpenRejectsMalformedURL(t *testing.T) {
	if _, err := database.Open(context.Background(), "://isso não é uma url"); err == nil {
		t.Fatal("esperado erro com URL malformada")
	}
}

func TestMigrateCreatesTablesAndIsIdempotent(t *testing.T) {
	db := testdb.Open(t) // já aplica as migrations uma vez
	ctx := context.Background()

	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("segunda execução deveria ser um no-op: %v", err)
	}

	for _, table := range []string{"usuarios", "clientes", "veiculos", "servicos"} {
		var exists bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`,
			table).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("tabela %s não existe (erro: %v)", table, err)
		}
	}
}

// Simula várias instâncias da API subindo ao mesmo tempo: o lock das
// migrations deve impedir que elas se atropelem.
func TestMigrateIsSafeUnderConcurrency(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- database.Migrate(ctx, db)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("Migrate concorrente falhou: %v", err)
		}
	}
}
