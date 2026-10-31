// Package domain do contexto catalog: os serviços que a oficina oferece.
package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/shared/text"
)

const (
	CodeInvalidServiceName = "INVALID_SERVICE_NAME"
	CodeInvalidDescription = "INVALID_DESCRIPTION"
	CodeInvalidPrice       = "INVALID_PRICE"
	CodeInvalidDuration    = "INVALID_DURATION"
	CodeServiceNotFound    = "SERVICE_NOT_FOUND"
	CodeServiceNameTaken   = "SERVICE_NAME_ALREADY_REGISTERED"
	CodeServiceInUse       = "SERVICE_IN_USE"

	minNameLength   = 2
	maxNameLength   = 100
	maxDescription  = 500
	maxPriceCents   = 100_000_000 // R$ 1.000.000,00
	maxDurationMins = 7 * 24 * 60
)

type ServiceData struct {
	Name             string
	Description      string
	PriceCents       int64
	EstimatedMinutes int
}

type Service struct {
	ID               uuid.UUID
	Name             string
	Description      string // opcional
	Price            money.Money
	EstimatedMinutes int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewService(id uuid.UUID, data ServiceData, now time.Time) (Service, error) {
	return Service{ID: id, CreatedAt: now}.apply(data, now)
}

func (s Service) Update(data ServiceData, now time.Time) (Service, error) {
	return s.apply(data, now)
}

func (s Service) apply(data ServiceData, now time.Time) (Service, error) {
	name, ok := text.Clean(data.Name, minNameLength, maxNameLength)
	if !ok {
		return Service{}, apperr.Validation(CodeInvalidServiceName, "o nome deve ter entre 2 e 100 caracteres")
	}

	description, ok := text.Clean(data.Description, 0, maxDescription)
	if !ok {
		return Service{}, apperr.Validation(CodeInvalidDescription, "a descrição deve ter até 500 caracteres")
	}

	// Preço zero também é recusado: um campo omitido no JSON chega como 0
	// e não deve virar um serviço gratuito sem querer.
	if data.PriceCents < 1 || data.PriceCents > maxPriceCents {
		return Service{}, apperr.Validation(CodeInvalidPrice, fmt.Sprintf("o preço deve estar entre 1 centavo e %s", money.FromCents(maxPriceCents)))
	}

	if data.EstimatedMinutes < 1 || data.EstimatedMinutes > maxDurationMins {
		return Service{}, apperr.Validation(CodeInvalidDuration, fmt.Sprintf("a duração estimada deve estar entre 1 e %d minutos", maxDurationMins))
	}

	s.Name, s.Description, s.Price, s.EstimatedMinutes, s.UpdatedAt = name, description, money.FromCents(data.PriceCents), data.EstimatedMinutes, now
	return s, nil
}

type ServiceRepository interface {
	Create(ctx context.Context, service Service) error
	Update(ctx context.Context, service Service) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (Service, error)
	List(ctx context.Context, limit, offset int) ([]Service, error)
}
