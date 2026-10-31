package infrastructure

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/testdb"
	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

// savedCustomer grava um cliente no banco; a limpeza roda depois da dos veículos
// criados em seguida, porque o t.Cleanup executa na ordem inversa.
func savedCustomer(t *testing.T, db *sql.DB) domain.Customer {
	t.Helper()
	customer := newCustomer(t, db, "Dono do Veículo")
	if err := NewCustomerRepository(db).Create(context.Background(), customer); err != nil {
		t.Fatal(err)
	}
	return customer
}

func newVehicle(t *testing.T, db *sql.DB, owner uuid.UUID) domain.Vehicle {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Microsecond)
	vehicle, err := domain.NewVehicle(uuid.New(), domain.VehicleData{
		CustomerID: owner, Plate: testdb.RandomPlate(), Brand: "Fiat", Model: "Uno", Year: 2010,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM veiculos WHERE id = $1`, vehicle.ID) })
	return vehicle
}

func sameVehicle(a, b domain.Vehicle) bool {
	return a.ID == b.ID && a.CustomerID == b.CustomerID && a.Plate == b.Plate &&
		a.Brand == b.Brand && a.Model == b.Model && a.Year == b.Year &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}

func TestVehicleRepositoryCreateAndFind(t *testing.T) {
	db := testdb.Open(t)
	repo := NewVehicleRepository(db)
	ctx := context.Background()
	vehicle := newVehicle(t, db, savedCustomer(t, db).ID)

	if err := repo.Create(ctx, vehicle); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byID, err := repo.FindByID(ctx, vehicle.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	byPlate, err := repo.FindByPlate(ctx, vehicle.Plate)
	if err != nil {
		t.Fatalf("FindByPlate: %v", err)
	}

	for _, got := range []domain.Vehicle{byID, byPlate} {
		if !sameVehicle(got, vehicle) {
			t.Errorf("veículo lido = %+v, esperado %+v", got, vehicle)
		}
	}
}

func TestVehicleRepositoryNotFound(t *testing.T) {
	db := testdb.Open(t)
	repo := NewVehicleRepository(db)
	ctx := context.Background()
	ghost := newVehicle(t, db, uuid.New())

	_, err := repo.FindByID(ctx, uuid.New())
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeVehicleNotFound)

	_, err = repo.FindByPlate(ctx, ghost.Plate)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeVehicleNotFound)

	wantAppErr(t, repo.Update(ctx, ghost), apperr.KindNotFound, domain.CodeVehicleNotFound)
	wantAppErr(t, repo.Delete(ctx, ghost.ID), apperr.KindNotFound, domain.CodeVehicleNotFound)
}

func TestVehicleRepositoryUnknownCustomer(t *testing.T) {
	db := testdb.Open(t)
	repo := NewVehicleRepository(db)
	ctx := context.Background()
	owner := savedCustomer(t, db)

	orphan := newVehicle(t, db, uuid.New())
	wantAppErr(t, repo.Create(ctx, orphan), apperr.KindValidation, domain.CodeCustomerNotFound)

	vehicle := newVehicle(t, db, owner.ID)
	if err := repo.Create(ctx, vehicle); err != nil {
		t.Fatal(err)
	}
	moved, _ := vehicle.Update(domain.VehicleData{CustomerID: uuid.New(), Plate: vehicle.Plate.String(), Brand: "Fiat", Model: "Uno", Year: 2010}, vehicle.UpdatedAt)
	wantAppErr(t, repo.Update(ctx, moved), apperr.KindValidation, domain.CodeCustomerNotFound)
}

func TestVehicleRepositoryDuplicatePlate(t *testing.T) {
	db := testdb.Open(t)
	repo := NewVehicleRepository(db)
	ctx := context.Background()
	owner := savedCustomer(t, db).ID
	first, second := newVehicle(t, db, owner), newVehicle(t, db, owner)
	for _, v := range []domain.Vehicle{first, second} {
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	clone := newVehicle(t, db, owner)
	clone.Plate = first.Plate
	wantAppErr(t, repo.Create(ctx, clone), apperr.KindConflict, domain.CodePlateTaken)

	second.Plate = first.Plate
	wantAppErr(t, repo.Update(ctx, second), apperr.KindConflict, domain.CodePlateTaken)
}

func TestVehicleRepositoryUpdateAndDelete(t *testing.T) {
	db := testdb.Open(t)
	repo := NewVehicleRepository(db)
	ctx := context.Background()
	vehicle := newVehicle(t, db, savedCustomer(t, db).ID)
	newOwner := savedCustomer(t, db).ID
	if err := repo.Create(ctx, vehicle); err != nil {
		t.Fatal(err)
	}

	updated, err := vehicle.Update(domain.VehicleData{CustomerID: newOwner, Plate: vehicle.Plate.String(), Brand: "VW", Model: "Gol", Year: 2015}, vehicle.UpdatedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.FindByID(ctx, vehicle.ID)
	if err != nil || !sameVehicle(got, updated) {
		t.Fatalf("veículo lido = %+v (%v), esperado %+v", got, err, updated)
	}

	if err := repo.Delete(ctx, vehicle.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.FindByID(ctx, vehicle.ID)
	wantAppErr(t, err, apperr.KindNotFound, domain.CodeVehicleNotFound)
}

func TestVehicleRepositoryListFiltersAndPaginates(t *testing.T) {
	db := testdb.Open(t)
	repo := NewVehicleRepository(db)
	ctx := context.Background()
	ownerA, ownerB := savedCustomer(t, db).ID, savedCustomer(t, db).ID

	for _, owner := range []uuid.UUID{ownerA, ownerA, ownerB} {
		if err := repo.Create(ctx, newVehicle(t, db, owner)); err != nil {
			t.Fatal(err)
		}
	}

	mine, err := repo.List(ctx, ownerA, 10, 0)
	if err != nil || len(mine) != 2 {
		t.Fatalf("filtrado: %d itens, %v", len(mine), err)
	}
	if mine[0].Plate.String() > mine[1].Plate.String() {
		t.Error("lista deveria estar ordenada por placa")
	}

	page, err := repo.List(ctx, ownerA, 1, 1)
	if err != nil || len(page) != 1 || page[0].ID != mine[1].ID {
		t.Errorf("página 2 = %+v, %v", page, err)
	}

	unfiltered, err := repo.List(ctx, uuid.Nil, 200, 0)
	if err != nil || len(unfiltered) < 3 {
		t.Errorf("sem filtro: %d itens, %v", len(unfiltered), err)
	}

	empty, err := repo.List(ctx, uuid.New(), 10, 0)
	if err != nil || len(empty) != 0 {
		t.Errorf("cliente sem veículos: %d itens, %v", len(empty), err)
	}
}

func TestCustomerWithVehicleCannotBeDeleted(t *testing.T) {
	db := testdb.Open(t)
	customers := NewCustomerRepository(db)
	ctx := context.Background()
	owner := savedCustomer(t, db)
	vehicle := newVehicle(t, db, owner.ID)
	if err := NewVehicleRepository(db).Create(ctx, vehicle); err != nil {
		t.Fatal(err)
	}

	wantAppErr(t, customers.Delete(ctx, owner.ID), apperr.KindConflict, domain.CodeCustomerInUse)

	if _, err := customers.FindByID(ctx, owner.ID); err != nil {
		t.Errorf("o cliente não deveria ter sido apagado: %v", err)
	}
}
