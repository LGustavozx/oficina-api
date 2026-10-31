// Package infrastructure do contexto catalog: persistência em PostgreSQL.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"oficina-api/internal/catalog/domain"
	"oficina-api/internal/platform/database"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
)

const serviceColumns = `id, nome, descricao, preco_centavos, duracao_estimada_min, criado_em, atualizado_em`

type ServiceRepository struct {
	db *sql.DB
}

func NewServiceRepository(db *sql.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) Create(ctx context.Context, s domain.Service) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO servicos (id, nome, descricao, preco_centavos, duracao_estimada_min, criado_em, atualizado_em)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, s.Name, s.Description, s.Price.Cents(), s.EstimatedMinutes, s.CreatedAt, s.UpdatedAt)
	return wrapWrite(err, "criar serviço")
}

func (r *ServiceRepository) Update(ctx context.Context, s domain.Service) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE servicos
		 SET nome = $2, descricao = $3, preco_centavos = $4, duracao_estimada_min = $5, atualizado_em = $6
		 WHERE id = $1`,
		s.ID, s.Name, s.Description, s.Price.Cents(), s.EstimatedMinutes, s.UpdatedAt)
	if err != nil {
		return wrapWrite(err, "atualizar serviço")
	}
	return database.RequireAffected(result, serviceNotFound())
}

func (r *ServiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM servicos WHERE id = $1`, id)
	if err != nil {
		return wrapWrite(err, "apagar serviço")
	}
	return database.RequireAffected(result, serviceNotFound())
}

func (r *ServiceRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Service, error) {
	return scanService(r.db.QueryRowContext(ctx,
		`SELECT `+serviceColumns+` FROM servicos WHERE id = $1`, id))
}

func (r *ServiceRepository) List(ctx context.Context, limit, offset int) ([]domain.Service, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+serviceColumns+` FROM servicos ORDER BY nome, id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("catalog: listar serviços: %w", err)
	}
	defer func() { _ = rows.Close() }()

	services := []domain.Service{}
	for rows.Next() {
		service, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: listar serviços: %w", err)
	}
	return services, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanService(row rowScanner) (domain.Service, error) {
	var (
		s     domain.Service
		cents int64
	)

	err := row.Scan(&s.ID, &s.Name, &s.Description, &cents, &s.EstimatedMinutes, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Service{}, serviceNotFound()
	}
	if err != nil {
		return domain.Service{}, fmt.Errorf("catalog: ler serviço: %w", err)
	}

	s.Price = money.FromCents(cents)
	s.CreatedAt, s.UpdatedAt = s.CreatedAt.UTC(), s.UpdatedAt.UTC()
	return s, nil
}

func serviceNotFound() error {
	return apperr.NotFound(domain.CodeServiceNotFound, "serviço não encontrado")
}

func wrapWrite(err error, action string) error {
	switch {
	case err == nil:
		return nil
	case database.IsUniqueViolation(err):
		return apperr.Conflict(domain.CodeServiceNameTaken, "já existe um serviço com esse nome")
	case database.IsForeignKeyViolation(err):
		return apperr.Conflict(domain.CodeServiceInUse, "o serviço está em uso em ordens de serviço")
	}
	return fmt.Errorf("catalog: %s: %w", action, err)
}
