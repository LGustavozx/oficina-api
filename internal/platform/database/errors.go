package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
)

// IsUniqueViolation informa se o erro é de chave duplicada (UNIQUE/PRIMARY KEY).
func IsUniqueViolation(err error) bool { return hasSQLState(err, sqlStateUniqueViolation) }

// IsForeignKeyViolation informa se o erro é de chave estrangeira, ao gravar uma
// referência inexistente ou ao apagar uma linha que ainda é referenciada.
func IsForeignKeyViolation(err error) bool { return hasSQLState(err, sqlStateForeignKeyViolation) }

func hasSQLState(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

// RequireAffected devolve notFound quando um UPDATE/DELETE não atingiu nenhuma linha.
func RequireAffected(result sql.Result, notFound error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("database: linhas afetadas: %w", err)
	}
	if affected == 0 {
		return notFound
	}
	return nil
}
