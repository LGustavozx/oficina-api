// Package application do contexto registry: casos de uso dos cadastros.
package application

import (
	"context"

	"github.com/google/uuid"

	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/clock"
)

type CustomerService struct {
	repo  domain.CustomerRepository
	clock clock.Clock
}

func NewCustomerService(repo domain.CustomerRepository, clk clock.Clock) *CustomerService {
	return &CustomerService{repo: repo, clock: clk}
}

func (s *CustomerService) Create(ctx context.Context, data domain.CustomerData) (domain.Customer, error) {
	customer, err := domain.NewCustomer(uuid.New(), data, s.clock.Now())
	if err != nil {
		return domain.Customer{}, err
	}

	if err := s.repo.Create(ctx, customer); err != nil {
		return domain.Customer{}, err
	}
	return customer, nil
}

func (s *CustomerService) Get(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *CustomerService) FindByDocument(ctx context.Context, rawDocument string) (domain.Customer, error) {
	document, err := domain.NewDocument(rawDocument)
	if err != nil {
		return domain.Customer{}, err
	}
	return s.repo.FindByDocument(ctx, document)
}

func (s *CustomerService) List(ctx context.Context, limit, offset int) ([]domain.Customer, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *CustomerService) Update(ctx context.Context, id uuid.UUID, data domain.CustomerData) (domain.Customer, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Customer{}, err
	}

	updated, err := current.Update(data, s.clock.Now())
	if err != nil {
		return domain.Customer{}, err
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return domain.Customer{}, err
	}
	return updated, nil
}

func (s *CustomerService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
