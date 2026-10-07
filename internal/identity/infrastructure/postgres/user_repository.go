// Package postgres implementa os repositórios do contexto Identidade sobre PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

const uniqueViolationCode = "23505"

// UserRepository implementa domain.UserRepository.
type UserRepository struct{ pool *pgxpool.Pool }

// NewUserRepository cria o repositório.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// FindByEmail devolve o usuário ou erro de categoria "não encontrado".
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `SELECT id, email, senha_hash, perfil, ativo, criado_em FROM usuarios WHERE email = $1`
	var u domain.User
	var role string
	err := r.pool.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &u.Active, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("USUARIO_NAO_ENCONTRADO", "usuário não encontrado")
	}
	if err != nil {
		return nil, fmt.Errorf("consultar usuário: %w", err)
	}
	u.Role = domain.Role(role)
	return &u, nil
}

// Save insere o usuário; e-mail repetido devolve erro de categoria "conflito".
func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	const query = `INSERT INTO usuarios (id, email, senha_hash, perfil, ativo, criado_em) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, query, u.ID, u.Email, u.PasswordHash, string(u.Role), u.Active, u.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return apperr.Conflict("EMAIL_JA_CADASTRADO", "e-mail já cadastrado")
	}
	if err != nil {
		return fmt.Errorf("inserir usuário: %w", err)
	}
	return nil
}
