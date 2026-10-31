package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"oficina-api/internal/platform/database"
	"oficina-api/internal/platform/testdb"
)

// Usa a tabela usuarios só como lugar para gravar e conferir linhas.
func insertUser(ctx context.Context, q database.DBTX, id uuid.UUID) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO usuarios (id, email, senha_hash, criado_em) VALUES ($1, $2, 'x', now())`,
		id, "tx-"+id.String()+"@teste.com")
	return err
}

func userExists(t *testing.T, db *sql.DB, id uuid.UUID) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM usuarios WHERE id = $1)`, id).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func cleanup(t *testing.T, db *sql.DB, ids ...uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = db.Exec(`DELETE FROM usuarios WHERE id = $1`, id)
		}
	})
}

func TestRunInTxCommitsOnSuccess(t *testing.T) {
	db := testdb.Open(t)
	id := uuid.New()
	cleanup(t, db, id)

	err := database.RunInTx(context.Background(), db, func(ctx context.Context) error {
		return insertUser(ctx, database.Conn(ctx, db), id)
	})

	if err != nil || !userExists(t, db, id) {
		t.Fatalf("a gravação deveria ter sido confirmada (erro: %v)", err)
	}
}

func TestRunInTxRollsBackOnError(t *testing.T) {
	db := testdb.Open(t)
	id := uuid.New()
	cleanup(t, db, id)
	boom := errors.New("falha no meio")

	err := database.RunInTx(context.Background(), db, func(ctx context.Context) error {
		if err := insertUser(ctx, database.Conn(ctx, db), id); err != nil {
			return err
		}
		return boom
	})

	if !errors.Is(err, boom) {
		t.Fatalf("erro = %v, esperado o erro de fn", err)
	}
	if userExists(t, db, id) {
		t.Error("a gravação deveria ter sido desfeita")
	}
}

func TestRunInTxNestedJoinsOuterTransaction(t *testing.T) {
	db := testdb.Open(t)
	inner, outer := uuid.New(), uuid.New()
	cleanup(t, db, inner, outer)
	boom := errors.New("falha depois do aninhado")

	err := database.RunInTx(context.Background(), db, func(ctx context.Context) error {
		innerErr := database.RunInTx(ctx, db, func(ctx context.Context) error {
			return insertUser(ctx, database.Conn(ctx, db), inner)
		})
		if innerErr != nil {
			return innerErr
		}
		if err := insertUser(ctx, database.Conn(ctx, db), outer); err != nil {
			return err
		}
		return boom
	})

	if !errors.Is(err, boom) {
		t.Fatalf("erro = %v", err)
	}
	if userExists(t, db, inner) || userExists(t, db, outer) {
		t.Error("o aninhado participa da transação externa: tudo deveria ter sido desfeito")
	}
}

func TestConnOutsideTransactionReturnsDB(t *testing.T) {
	db := testdb.Open(t)
	id := uuid.New()
	cleanup(t, db, id)

	if err := insertUser(context.Background(), database.Conn(context.Background(), db), id); err != nil {
		t.Fatal(err)
	}
	if !userExists(t, db, id) {
		t.Error("sem transação, a gravação deveria ir direto ao banco")
	}
}

func TestTransactorRuns(t *testing.T) {
	db := testdb.Open(t)
	id := uuid.New()
	cleanup(t, db, id)

	err := database.Transactor{DB: db}.Run(context.Background(), func(ctx context.Context) error {
		return insertUser(ctx, database.Conn(ctx, db), id)
	})

	if err != nil || !userExists(t, db, id) {
		t.Fatalf("Transactor.Run falhou (erro: %v)", err)
	}
}

func TestRunInTxFailsWhenBeginFails(t *testing.T) {
	db := testdb.Open(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := database.RunInTx(ctx, db, func(context.Context) error { return nil })

	if err == nil {
		t.Error("esperado erro com o contexto já cancelado")
	}
}
