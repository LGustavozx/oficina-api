package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
)

const (
	CodeInvalidBrand    = "INVALID_BRAND"
	CodeInvalidModel    = "INVALID_MODEL"
	CodeInvalidYear     = "INVALID_YEAR"
	CodeVehicleNotFound = "VEHICLE_NOT_FOUND"
	CodePlateTaken      = "PLATE_ALREADY_REGISTERED"
	CodeVehicleInUse    = "VEHICLE_IN_USE"

	maxVehicleTextLength = 60
	minVehicleYear       = 1900
)

type VehicleData struct {
	CustomerID uuid.UUID
	Plate      string
	Brand      string
	Model      string
	Year       int
}

type Vehicle struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	Plate      Plate
	Brand      string
	Model      string
	Year       int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewVehicle(id uuid.UUID, data VehicleData, now time.Time) (Vehicle, error) {
	return Vehicle{ID: id, CreatedAt: now}.apply(data, now)
}

func (v Vehicle) Update(data VehicleData, now time.Time) (Vehicle, error) {
	return v.apply(data, now)
}

func (v Vehicle) apply(data VehicleData, now time.Time) (Vehicle, error) {
	plate, err := NewPlate(data.Plate)
	if err != nil {
		return Vehicle{}, err
	}

	brand, ok := cleanText(data.Brand, maxVehicleTextLength)
	if !ok {
		return Vehicle{}, apperr.Validation(CodeInvalidBrand, "a marca deve ter entre 1 e 60 caracteres")
	}

	model, ok := cleanText(data.Model, maxVehicleTextLength)
	if !ok {
		return Vehicle{}, apperr.Validation(CodeInvalidModel, "o modelo deve ter entre 1 e 60 caracteres")
	}

	// Aceita o ano seguinte porque modelos do próximo ano chegam antes dele começar.
	if maxYear := now.Year() + 1; data.Year < minVehicleYear || data.Year > maxYear {
		return Vehicle{}, apperr.Validation(CodeInvalidYear, fmt.Sprintf("o ano deve estar entre %d e %d", minVehicleYear, maxYear))
	}

	v.CustomerID, v.Plate, v.Brand, v.Model, v.Year, v.UpdatedAt = data.CustomerID, plate, brand, model, data.Year, now
	return v, nil
}

// cleanText apara o texto e confere o tamanho, recusando UTF-8 corrompido (U+FFFD).
func cleanText(raw string, maxLength int) (string, bool) {
	value := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(value)
	if n < 1 || n > maxLength || strings.ContainsRune(value, utf8.RuneError) {
		return "", false
	}
	return value, true
}

type VehicleRepository interface {
	Create(ctx context.Context, vehicle Vehicle) error
	Update(ctx context.Context, vehicle Vehicle) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (Vehicle, error)
	FindByPlate(ctx context.Context, plate Plate) (Vehicle, error)
	// customerID uuid.Nil significa "sem filtro".
	List(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]Vehicle, error)
}
