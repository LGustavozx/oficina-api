package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSQLStateChecks(t *testing.T) {
	unique := &pgconn.PgError{Code: "23505"}
	foreignKey := &pgconn.PgError{Code: "23503"}
	other := &pgconn.PgError{Code: "42601"}

	tests := []struct {
		name           string
		err            error
		wantUnique     bool
		wantForeignKey bool
	}{
		{"violação de unicidade", unique, true, false},
		{"violação de chave estrangeira", foreignKey, false, true},
		{"outro erro do Postgres", other, false, false},
		{"erro comum", errors.New("x"), false, false},
		{"nil", nil, false, false},
		{"embrulhado", fmt.Errorf("gravar: %w", unique), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUniqueViolation(tt.err); got != tt.wantUnique {
				t.Errorf("IsUniqueViolation = %v, esperado %v", got, tt.wantUnique)
			}
			if got := IsForeignKeyViolation(tt.err); got != tt.wantForeignKey {
				t.Errorf("IsForeignKeyViolation = %v, esperado %v", got, tt.wantForeignKey)
			}
		})
	}
}

type fakeResult struct {
	affected int64
	err      error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.affected, r.err }

func TestRequireAffected(t *testing.T) {
	notFound := errors.New("não encontrado")

	if err := RequireAffected(fakeResult{affected: 1}, notFound); err != nil {
		t.Errorf("1 linha afetada: erro inesperado %v", err)
	}
	if err := RequireAffected(fakeResult{affected: 0}, notFound); !errors.Is(err, notFound) {
		t.Errorf("0 linhas: erro = %v, esperado notFound", err)
	}
	if err := RequireAffected(fakeResult{err: errors.New("driver")}, notFound); err == nil || errors.Is(err, notFound) {
		t.Errorf("erro do driver deveria ser propagado, não virar notFound: %v", err)
	}
}
