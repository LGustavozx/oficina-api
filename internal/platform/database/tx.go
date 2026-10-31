package database

import (
	"context"
	"database/sql"
	"fmt"
)

// DBTX é o que *sql.DB e *sql.Tx têm em comum.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// Conn devolve a transação em andamento no contexto, se houver; senão o próprio banco.
// Os repositórios usam Conn para participarem de uma transação aberta por quem os chama.
func Conn(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// RunInTx executa fn dentro de uma transação: confirma se fn devolver nil,
// desfaz tudo se devolver erro. Se já houver transação no contexto, fn
// participa dela, e quem a abriu decide o resultado.
func RunInTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: iniciar transação: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: confirmar transação: %w", err)
	}
	return nil
}

// Transactor adapta RunInTx para quem recebe a dependência como interface.
type Transactor struct {
	DB *sql.DB
}

func (t Transactor) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	return RunInTx(ctx, t.DB, fn)
}
