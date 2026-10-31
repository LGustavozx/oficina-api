package application

import (
	"context"

	"github.com/google/uuid"

	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/clock"
)

type VehicleService struct {
	repo  domain.VehicleRepository
	clock clock.Clock
}

func NewVehicleService(repo domain.VehicleRepository, clk clock.Clock) *VehicleService {
	return &VehicleService{repo: repo, clock: clk}
}

func (s *VehicleService) Create(ctx context.Context, data domain.VehicleData) (domain.Vehicle, error) {
	vehicle, err := domain.NewVehicle(uuid.New(), data, s.clock.Now())
	if err != nil {
		return domain.Vehicle{}, err
	}

	if err := s.repo.Create(ctx, vehicle); err != nil {
		return domain.Vehicle{}, err
	}
	return vehicle, nil
}

func (s *VehicleService) Get(ctx context.Context, id uuid.UUID) (domain.Vehicle, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *VehicleService) FindByPlate(ctx context.Context, rawPlate string) (domain.Vehicle, error) {
	plate, err := domain.NewPlate(rawPlate)
	if err != nil {
		return domain.Vehicle{}, err
	}
	return s.repo.FindByPlate(ctx, plate)
}

// List filtra por cliente quando customerID não é uuid.Nil.
func (s *VehicleService) List(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]domain.Vehicle, error) {
	return s.repo.List(ctx, customerID, limit, offset)
}

func (s *VehicleService) Update(ctx context.Context, id uuid.UUID, data domain.VehicleData) (domain.Vehicle, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Vehicle{}, err
	}

	updated, err := current.Update(data, s.clock.Now())
	if err != nil {
		return domain.Vehicle{}, err
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return domain.Vehicle{}, err
	}
	return updated, nil
}

func (s *VehicleService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
