// Package infrastructure do contexto identity: PostgreSQL, bcrypt e JWT.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"oficina-api/internal/identity/domain"
	"oficina-api/internal/shared/apperr"
)

const pgUniqueViolation = "23505"

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO usuarios (id, email, senha_hash, criado_em) VALUES ($1, $2, $3, $4)`,
		user.ID, user.Email.String(), user.PasswordHash, user.CreatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return apperr.Conflict(domain.CodeEmailTaken, "e-mail já cadastrado")
	}
	if err != nil {
		return fmt.Errorf("identity: criar usuário: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	return r.findOne(ctx, `WHERE email = $1`, email.String())
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return r.findOne(ctx, `WHERE id = $1`, id)
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM usuarios`).Scan(&count); err != nil {
		return 0, fmt.Errorf("identity: contar usuários: %w", err)
	}
	return count, nil
}

func (r *UserRepository) findOne(ctx context.Context, where string, arg any) (domain.User, error) {
	var (
		user     domain.User
		rawEmail string
	)

	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, senha_hash, criado_em FROM usuarios `+where, arg).
		Scan(&user.ID, &rawEmail, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, apperr.NotFound(domain.CodeUserNotFound, "usuário não encontrado")
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("identity: buscar usuário: %w", err)
	}

	user.Email, err = domain.NewEmail(rawEmail)
	if err != nil {
		return domain.User{}, fmt.Errorf("identity: e-mail armazenado inválido: %w", err)
	}
	return user, nil
}
