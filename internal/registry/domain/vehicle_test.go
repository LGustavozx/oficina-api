package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
)

var ownerID = uuid.MustParse("22222222-2222-2222-2222-222222222222")

func validVehicle() VehicleData {
	return VehicleData{CustomerID: ownerID, Plate: "abc-1d23", Brand: " Fiat ", Model: "Uno Mille", Year: 2010}
}

func TestNewVehicleNormalizesData(t *testing.T) {
	id := uuid.New()

	v, err := NewVehicle(id, validVehicle(), t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if v.ID != id || v.CustomerID != ownerID || v.Plate.String() != "ABC1D23" || v.Brand != "Fiat" || v.Model != "Uno Mille" || v.Year != 2010 {
		t.Errorf("dados errados: %+v", v)
	}
	if !v.CreatedAt.Equal(t0) || !v.UpdatedAt.Equal(t0) {
		t.Errorf("datas = %v / %v", v.CreatedAt, v.UpdatedAt)
	}
}

func TestNewVehicleYearLimits(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		year    int
		wantErr bool
	}{
		{1900, false},
		{2026, false},
		{2027, false},
		{1899, true},
		{2028, true},
		{0, true},
		{-1, true},
	}

	for _, tt := range tests {
		data := validVehicle()
		data.Year = tt.year

		_, err := NewVehicle(uuid.New(), data, now)

		if (err != nil) != tt.wantErr {
			t.Errorf("ano %d: erro = %v, esperado erro: %v", tt.year, err, tt.wantErr)
		}
	}
}

func TestNewVehicleInvalid(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*VehicleData)
		wantCode string
	}{
		{"placa inválida", func(d *VehicleData) { d.Plate = "123" }, CodeInvalidPlate},
		{"marca vazia", func(d *VehicleData) { d.Brand = "  " }, CodeInvalidBrand},
		{"marca longa demais", func(d *VehicleData) { d.Brand = strings.Repeat("a", 61) }, CodeInvalidBrand},
		{"marca com UTF-8 corrompido", func(d *VehicleData) { d.Brand = "Fi�at" }, CodeInvalidBrand},
		{"modelo vazio", func(d *VehicleData) { d.Model = "" }, CodeInvalidModel},
		{"modelo longo demais", func(d *VehicleData) { d.Model = strings.Repeat("a", 61) }, CodeInvalidModel},
		{"ano inválido", func(d *VehicleData) { d.Year = 1800 }, CodeInvalidYear},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validVehicle()
			tt.mutate(&data)

			_, err := NewVehicle(uuid.New(), data, t0)

			appErr, ok := apperr.As(err)
			if !ok || appErr.Code != tt.wantCode || appErr.Kind != apperr.KindValidation {
				t.Errorf("erro = %v, esperado %s", err, tt.wantCode)
			}
		})
	}
}

func TestVehicleUpdate(t *testing.T) {
	original, err := NewVehicle(uuid.New(), validVehicle(), t0)
	if err != nil {
		t.Fatal(err)
	}
	newOwner := uuid.New()

	updated, err := original.Update(VehicleData{CustomerID: newOwner, Plate: "XYZ9876", Brand: "VW", Model: "Gol", Year: 2015}, t1)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if updated.ID != original.ID || !updated.CreatedAt.Equal(t0) {
		t.Error("ID e data de criação não podem mudar")
	}
	if updated.CustomerID != newOwner || updated.Plate.String() != "XYZ9876" || updated.Model != "Gol" || !updated.UpdatedAt.Equal(t1) {
		t.Errorf("atualização errada: %+v", updated)
	}
	if original.Model != "Uno Mille" {
		t.Error("o original não deveria ser alterado")
	}

	if _, err := original.Update(VehicleData{}, t1); err == nil {
		t.Error("esperado erro de validação na atualização")
	}
}
