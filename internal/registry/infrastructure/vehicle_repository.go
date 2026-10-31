package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

const vehicleColumns = `id, cliente_id, placa, marca, modelo, ano, criado_em, atualizado_em`

type VehicleRepository struct {
	db *sql.DB
}

func NewVehicleRepository(db *sql.DB) *VehicleRepository {
	return &VehicleRepository{db: db}
}

func (r *VehicleRepository) Create(ctx context.Context, v domain.Vehicle) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO veiculos (id, cliente_id, placa, marca, modelo, ano, criado_em, atualizado_em)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		v.ID, v.CustomerID, v.Plate.String(), v.Brand, v.Model, v.Year, v.CreatedAt, v.UpdatedAt)
	return wrapVehicleWrite(err, "criar veículo", customerMissing())
}

func (r *VehicleRepository) Update(ctx context.Context, v domain.Vehicle) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE veiculos
		 SET cliente_id = $2, placa = $3, marca = $4, modelo = $5, ano = $6, atualizado_em = $7
		 WHERE id = $1`,
		v.ID, v.CustomerID, v.Plate.String(), v.Brand, v.Model, v.Year, v.UpdatedAt)
	if err != nil {
		return wrapVehicleWrite(err, "atualizar veículo", customerMissing())
	}
	return requireAffected(result, vehicleNotFound())
}

func (r *VehicleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM veiculos WHERE id = $1`, id)
	if err != nil {
		return wrapVehicleWrite(err, "apagar veículo", vehicleInUse())
	}
	return requireAffected(result, vehicleNotFound())
}

func (r *VehicleRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Vehicle, error) {
	return scanVehicle(r.db.QueryRowContext(ctx,
		`SELECT `+vehicleColumns+` FROM veiculos WHERE id = $1`, id))
}

func (r *VehicleRepository) FindByPlate(ctx context.Context, plate domain.Plate) (domain.Vehicle, error) {
	return scanVehicle(r.db.QueryRowContext(ctx,
		`SELECT `+vehicleColumns+` FROM veiculos WHERE placa = $1`, plate.String()))
}

// O filtro `$1 = '00000000-...' OR cliente_id = $1` evita montar SQL por concatenação.
func (r *VehicleRepository) List(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]domain.Vehicle, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+vehicleColumns+` FROM veiculos
		 WHERE $1::uuid = '00000000-0000-0000-0000-000000000000' OR cliente_id = $1
		 ORDER BY placa LIMIT $2 OFFSET $3`,
		customerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("registry: listar veículos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	vehicles := []domain.Vehicle{}
	for rows.Next() {
		vehicle, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		vehicles = append(vehicles, vehicle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registry: listar veículos: %w", err)
	}
	return vehicles, nil
}

func scanVehicle(row rowScanner) (domain.Vehicle, error) {
	var (
		v        domain.Vehicle
		rawPlate string
	)

	err := row.Scan(&v.ID, &v.CustomerID, &rawPlate, &v.Brand, &v.Model, &v.Year, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Vehicle{}, vehicleNotFound()
	}
	if err != nil {
		return domain.Vehicle{}, fmt.Errorf("registry: ler veículo: %w", err)
	}

	v.Plate, err = domain.NewPlate(rawPlate)
	if err != nil {
		return domain.Vehicle{}, fmt.Errorf("registry: placa armazenada inválida: %w", err)
	}

	v.CreatedAt, v.UpdatedAt = v.CreatedAt.UTC(), v.UpdatedAt.UTC()
	return v, nil
}

func vehicleNotFound() error {
	return apperr.NotFound(domain.CodeVehicleNotFound, "veículo não encontrado")
}

func customerMissing() error {
	return apperr.Validation(domain.CodeCustomerNotFound, "o cliente informado não existe")
}

func vehicleInUse() error {
	return apperr.Conflict(domain.CodeVehicleInUse, "o veículo possui registros vinculados")
}

// O SQL state de violação de FK é o mesmo ao gravar com cliente inexistente e ao
// apagar um veículo referenciado; quem chama diz qual dos dois casos é.
func wrapVehicleWrite(err error, action string, foreignKeyErr error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return apperr.Conflict(domain.CodePlateTaken, "placa já cadastrada")
		case pgForeignKeyViolation:
			return foreignKeyErr
		}
	}
	return fmt.Errorf("registry: %s: %w", action, err)
}
