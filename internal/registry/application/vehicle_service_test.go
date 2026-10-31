package application

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/google/uuid"

	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

type fakeVehicleRepo struct {
	vehicles map[uuid.UUID]domain.Vehicle
	err      error
}

func newFakeVehicleRepo() *fakeVehicleRepo {
	return &fakeVehicleRepo{vehicles: map[uuid.UUID]domain.Vehicle{}}
}

func vehicleNotFound() error {
	return apperr.NotFound(domain.CodeVehicleNotFound, "veículo não encontrado")
}

func (r *fakeVehicleRepo) Create(_ context.Context, v domain.Vehicle) error {
	if r.err != nil {
		return r.err
	}
	r.vehicles[v.ID] = v
	return nil
}

func (r *fakeVehicleRepo) Update(_ context.Context, v domain.Vehicle) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.vehicles[v.ID]; !ok {
		return vehicleNotFound()
	}
	r.vehicles[v.ID] = v
	return nil
}

func (r *fakeVehicleRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.vehicles[id]; !ok {
		return vehicleNotFound()
	}
	delete(r.vehicles, id)
	return nil
}

func (r *fakeVehicleRepo) FindByID(_ context.Context, id uuid.UUID) (domain.Vehicle, error) {
	v, ok := r.vehicles[id]
	if !ok {
		return domain.Vehicle{}, vehicleNotFound()
	}
	return v, nil
}

func (r *fakeVehicleRepo) FindByPlate(_ context.Context, p domain.Plate) (domain.Vehicle, error) {
	for _, v := range r.vehicles {
		if v.Plate == p {
			return v, nil
		}
	}
	return domain.Vehicle{}, vehicleNotFound()
}

func (r *fakeVehicleRepo) List(_ context.Context, customerID uuid.UUID, limit, offset int) ([]domain.Vehicle, error) {
	var all []domain.Vehicle
	for _, v := range r.vehicles {
		if customerID == uuid.Nil || v.CustomerID == customerID {
			all = append(all, v)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Plate.String() < all[j].Plate.String() })

	if offset >= len(all) {
		return nil, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

var ownerA, ownerB = uuid.New(), uuid.New()

func newVehicleService(repo *fakeVehicleRepo) *VehicleService {
	return NewVehicleService(repo, clock.Fixed{Time: now})
}

func uno(owner uuid.UUID, plate string) domain.VehicleData {
	return domain.VehicleData{CustomerID: owner, Plate: plate, Brand: "Fiat", Model: "Uno", Year: 2010}
}

func TestVehicleCreate(t *testing.T) {
	repo := newFakeVehicleRepo()
	svc := newVehicleService(repo)

	v, err := svc.Create(context.Background(), uno(ownerA, "ABC-1234"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if v.ID == uuid.Nil || v.Plate.String() != "ABC1234" || !v.CreatedAt.Equal(now) {
		t.Errorf("veículo errado: %+v", v)
	}
	if _, ok := repo.vehicles[v.ID]; !ok {
		t.Error("veículo não foi gravado")
	}
}

func TestVehicleCreateErrors(t *testing.T) {
	repo := newFakeVehicleRepo()
	svc := newVehicleService(repo)

	if _, err := svc.Create(context.Background(), domain.VehicleData{}); err == nil {
		t.Error("esperado erro de validação")
	}
	if len(repo.vehicles) != 0 {
		t.Error("dados inválidos não devem chegar ao repositório")
	}

	repo.err = apperr.Conflict(domain.CodePlateTaken, "placa já cadastrada")
	_, err := svc.Create(context.Background(), uno(ownerA, "ABC1234"))
	if appErr, ok := apperr.As(err); !ok || appErr.Code != domain.CodePlateTaken {
		t.Errorf("erro do repositório não propagado: %v", err)
	}
}

func TestVehicleGetAndFindByPlate(t *testing.T) {
	svc := newVehicleService(newFakeVehicleRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, uno(ownerA, "ABC1234"))

	if got, err := svc.Get(ctx, created.ID); err != nil || got.ID != created.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := svc.Get(ctx, uuid.New()); err == nil {
		t.Error("esperado not found")
	}

	if got, err := svc.FindByPlate(ctx, "abc-1234"); err != nil || got.ID != created.ID {
		t.Fatalf("FindByPlate = %+v, %v", got, err)
	}
	if _, err := svc.FindByPlate(ctx, "###"); err == nil {
		t.Error("placa inválida deveria falhar na validação")
	}
	if _, err := svc.FindByPlate(ctx, "ZZZ9999"); err == nil {
		t.Error("esperado not found para placa não cadastrada")
	}
}

func TestVehicleListFiltersByCustomer(t *testing.T) {
	svc := newVehicleService(newFakeVehicleRepo())
	ctx := context.Background()
	_, _ = svc.Create(ctx, uno(ownerA, "AAA1111"))
	_, _ = svc.Create(ctx, uno(ownerA, "BBB2222"))
	_, _ = svc.Create(ctx, uno(ownerB, "CCC3333"))

	all, err := svc.List(ctx, uuid.Nil, 10, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("sem filtro: %d itens, %v", len(all), err)
	}

	mine, err := svc.List(ctx, ownerA, 10, 0)
	if err != nil || len(mine) != 2 {
		t.Fatalf("com filtro: %d itens, %v", len(mine), err)
	}
}

func TestVehicleUpdate(t *testing.T) {
	repo := newFakeVehicleRepo()
	svc := newVehicleService(repo)
	ctx := context.Background()
	created, _ := svc.Create(ctx, uno(ownerA, "ABC1234"))

	data := uno(ownerB, "ABC1234")
	data.Model = "Palio"
	updated, err := svc.Update(ctx, created.ID, data)
	if err != nil || updated.Model != "Palio" || updated.CustomerID != ownerB || updated.ID != created.ID {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	if repo.vehicles[created.ID].Model != "Palio" {
		t.Error("alteração não foi gravada")
	}

	if _, err := svc.Update(ctx, uuid.New(), data); err == nil {
		t.Error("esperado not found")
	}
	if _, err := svc.Update(ctx, created.ID, domain.VehicleData{}); err == nil {
		t.Error("esperado erro de validação")
	}

	repo.err = errors.New("banco fora do ar")
	if _, err := svc.Update(ctx, created.ID, data); err == nil {
		t.Error("esperado erro do repositório")
	}
}

func TestVehicleDelete(t *testing.T) {
	svc := newVehicleService(newFakeVehicleRepo())
	ctx := context.Background()
	created, _ := svc.Create(ctx, uno(ownerA, "ABC1234"))

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := svc.Delete(ctx, created.ID); err == nil {
		t.Error("esperado not found ao apagar de novo")
	}
}
