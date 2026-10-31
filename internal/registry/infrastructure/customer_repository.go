// Package infrastructure do contexto registry: persistência em PostgreSQL.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"oficina-api/internal/platform/database"
	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

const customerColumns = `id, nome, documento, email, telefone, criado_em, atualizado_em`

type CustomerRepository struct {
	db *sql.DB
}

func NewCustomerRepository(db *sql.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(ctx context.Context, c domain.Customer) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO clientes (id, nome, documento, tipo_documento, email, telefone, criado_em, atualizado_em)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		c.ID, c.Name, c.Document.String(), string(c.Document.Type()), c.Email, c.Phone, c.CreatedAt, c.UpdatedAt)
	return wrapWrite(err, "criar cliente")
}

func (r *CustomerRepository) Update(ctx context.Context, c domain.Customer) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE clientes
		 SET nome = $2, documento = $3, tipo_documento = $4, email = $5, telefone = $6, atualizado_em = $7
		 WHERE id = $1`,
		c.ID, c.Name, c.Document.String(), string(c.Document.Type()), c.Email, c.Phone, c.UpdatedAt)
	if err != nil {
		return wrapWrite(err, "atualizar cliente")
	}
	return database.RequireAffected(result, customerNotFound())
}

func (r *CustomerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM clientes WHERE id = $1`, id)
	if err != nil {
		return wrapWrite(err, "apagar cliente")
	}
	return database.RequireAffected(result, customerNotFound())
}

func (r *CustomerRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	return scanCustomer(r.db.QueryRowContext(ctx,
		`SELECT `+customerColumns+` FROM clientes WHERE id = $1`, id))
}

func (r *CustomerRepository) FindByDocument(ctx context.Context, document domain.Document) (domain.Customer, error) {
	return scanCustomer(r.db.QueryRowContext(ctx,
		`SELECT `+customerColumns+` FROM clientes WHERE documento = $1`, document.String()))
}

func (r *CustomerRepository) List(ctx context.Context, limit, offset int) ([]domain.Customer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+customerColumns+` FROM clientes ORDER BY nome, id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("registry: listar clientes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	customers := []domain.Customer{}
	for rows.Next() {
		customer, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registry: listar clientes: %w", err)
	}
	return customers, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCustomer(row rowScanner) (domain.Customer, error) {
	var (
		c           domain.Customer
		rawDocument string
	)

	err := row.Scan(&c.ID, &c.Name, &rawDocument, &c.Email, &c.Phone, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Customer{}, customerNotFound()
	}
	if err != nil {
		return domain.Customer{}, fmt.Errorf("registry: ler cliente: %w", err)
	}

	c.Document, err = domain.NewDocument(rawDocument)
	if err != nil {
		return domain.Customer{}, fmt.Errorf("registry: documento armazenado inválido: %w", err)
	}

	// O driver devolve no fuso da sessão; a API trabalha sempre em UTC.
	c.CreatedAt, c.UpdatedAt = c.CreatedAt.UTC(), c.UpdatedAt.UTC()
	return c, nil
}

func customerNotFound() error {
	return apperr.NotFound(domain.CodeCustomerNotFound, "cliente não encontrado")
}

func wrapWrite(err error, action string) error {
	switch {
	case err == nil:
		return nil
	case database.IsUniqueViolation(err):
		return apperr.Conflict(domain.CodeDocumentTaken, "documento já cadastrado")
	case database.IsForeignKeyViolation(err):
		return apperr.Conflict(domain.CodeCustomerInUse, "o cliente possui registros vinculados")
	}
	return fmt.Errorf("registry: %s: %w", action, err)
}
