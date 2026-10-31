// Package application do contexto inventory: casos de uso de peças e estoque.
package application

import (
	"context"

	"github.com/google/uuid"

	"oficina-api/internal/inventory/domain"
	"oficina-api/internal/shared/clock"
)

type InventoryService struct {
	repo  domain.PartRepository
	clock clock.Clock
}

func NewInventoryService(repo domain.PartRepository, clk clock.Clock) *InventoryService {
	return &InventoryService{repo: repo, clock: clk}
}

func (s *InventoryService) Create(ctx context.Context, data domain.PartData) (domain.Part, error) {
	part, err := domain.NewPart(uuid.New(), data, s.clock.Now())
	if err != nil {
		return domain.Part{}, err
	}

	if err := s.repo.Create(ctx, part); err != nil {
		return domain.Part{}, err
	}
	return part, nil
}

func (s *InventoryService) Get(ctx context.Context, id uuid.UUID) (domain.Part, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *InventoryService) FindBySKU(ctx context.Context, rawSKU string) (domain.Part, error) {
	return s.repo.FindBySKU(ctx, normalizeSKU(rawSKU))
}

func (s *InventoryService) List(ctx context.Context, limit, offset int) ([]domain.Part, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *InventoryService) Update(ctx context.Context, id uuid.UUID, data domain.PartData) (domain.Part, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Part{}, err
	}

	updated, err := current.Update(data, s.clock.Now())
	if err != nil {
		return domain.Part{}, err
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return domain.Part{}, err
	}
	return updated, nil
}

func (s *InventoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// RecordMovement devolve a peça já com o saldo atualizado.
func (s *InventoryService) RecordMovement(ctx context.Context, partID uuid.UUID, data domain.MovementData) (domain.Part, domain.StockMovement, error) {
	movement, err := domain.NewStockMovement(uuid.New(), partID, data, s.clock.Now())
	if err != nil {
		return domain.Part{}, domain.StockMovement{}, err
	}

	part, err := s.repo.RecordMovement(ctx, movement)
	if err != nil {
		return domain.Part{}, domain.StockMovement{}, err
	}
	return part, movement, nil
}

// ListMovements confere antes se a peça existe, para uma peça inexistente
// dar 404 em vez de uma lista vazia.
func (s *InventoryService) ListMovements(ctx context.Context, partID uuid.UUID, limit, offset int) ([]domain.StockMovement, error) {
	if _, err := s.repo.FindByID(ctx, partID); err != nil {
		return nil, err
	}
	return s.repo.ListMovements(ctx, partID, limit, offset)
}
