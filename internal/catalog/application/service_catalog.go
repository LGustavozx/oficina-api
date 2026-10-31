// Package application do contexto catalog: casos de uso do catálogo de serviços.
package application

import (
	"context"

	"github.com/google/uuid"

	"oficina-api/internal/catalog/domain"
	"oficina-api/internal/shared/clock"
)

type ServiceCatalog struct {
	repo  domain.ServiceRepository
	clock clock.Clock
}

func NewServiceCatalog(repo domain.ServiceRepository, clk clock.Clock) *ServiceCatalog {
	return &ServiceCatalog{repo: repo, clock: clk}
}

func (c *ServiceCatalog) Create(ctx context.Context, data domain.ServiceData) (domain.Service, error) {
	service, err := domain.NewService(uuid.New(), data, c.clock.Now())
	if err != nil {
		return domain.Service{}, err
	}

	if err := c.repo.Create(ctx, service); err != nil {
		return domain.Service{}, err
	}
	return service, nil
}

func (c *ServiceCatalog) Get(ctx context.Context, id uuid.UUID) (domain.Service, error) {
	return c.repo.FindByID(ctx, id)
}

func (c *ServiceCatalog) List(ctx context.Context, limit, offset int) ([]domain.Service, error) {
	return c.repo.List(ctx, limit, offset)
}

func (c *ServiceCatalog) Update(ctx context.Context, id uuid.UUID, data domain.ServiceData) (domain.Service, error) {
	current, err := c.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Service{}, err
	}

	updated, err := current.Update(data, c.clock.Now())
	if err != nil {
		return domain.Service{}, err
	}

	if err := c.repo.Update(ctx, updated); err != nil {
		return domain.Service{}, err
	}
	return updated, nil
}

func (c *ServiceCatalog) Delete(ctx context.Context, id uuid.UUID) error {
	return c.repo.Delete(ctx, id)
}
