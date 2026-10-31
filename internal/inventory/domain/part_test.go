package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
)

var (
	t0 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
)

func validPart() PartData {
	return PartData{SKU: " flt-oleo-01 ", Name: "  Filtro de óleo ", Description: " Para motores 1.0 ", UnitPriceCents: 3590}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != code || appErr.Kind != apperr.KindValidation {
		t.Fatalf("erro = %v, esperado validação %s", err, code)
	}
}

func TestNewPartNormalizesData(t *testing.T) {
	id := uuid.New()

	p, err := NewPart(id, validPart(), t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if p.ID != id || p.SKU != "FLT-OLEO-01" || p.Name != "Filtro de óleo" || p.Description != "Para motores 1.0" || p.UnitPrice.Cents() != 3590 {
		t.Errorf("dados errados: %+v", p)
	}
	if p.Stock != 0 {
		t.Errorf("peça nova deve começar com estoque 0, veio %d", p.Stock)
	}
	if !p.CreatedAt.Equal(t0) || !p.UpdatedAt.Equal(t0) {
		t.Errorf("datas = %v / %v", p.CreatedAt, p.UpdatedAt)
	}
}

func TestNewPartInvalid(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*PartData)
		wantCode string
	}{
		{"SKU vazio", func(d *PartData) { d.SKU = "" }, CodeInvalidSKU},
		{"SKU curto", func(d *PartData) { d.SKU = "A" }, CodeInvalidSKU},
		{"SKU longo", func(d *PartData) { d.SKU = strings.Repeat("A", 31) }, CodeInvalidSKU},
		{"SKU com espaço", func(d *PartData) { d.SKU = "AB CD" }, CodeInvalidSKU},
		{"SKU com acento", func(d *PartData) { d.SKU = "FILTRO-ÓLEO" }, CodeInvalidSKU},
		{"SKU começando com hífen", func(d *PartData) { d.SKU = "-ABC" }, CodeInvalidSKU},
		{"nome curto", func(d *PartData) { d.Name = "a" }, CodeInvalidPartName},
		{"nome longo", func(d *PartData) { d.Name = strings.Repeat("a", 101) }, CodeInvalidPartName},
		{"nome com UTF-8 corrompido", func(d *PartData) { d.Name = "Filtro �" }, CodeInvalidPartName},
		{"descrição longa", func(d *PartData) { d.Description = strings.Repeat("a", 501) }, CodeInvalidDescription},
		{"preço zero (campo omitido)", func(d *PartData) { d.UnitPriceCents = 0 }, CodeInvalidPrice},
		{"preço negativo", func(d *PartData) { d.UnitPriceCents = -1 }, CodeInvalidPrice},
		{"preço acima do máximo", func(d *PartData) { d.UnitPriceCents = maxPriceCents + 1 }, CodeInvalidPrice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validPart()
			tt.mutate(&data)

			_, err := NewPart(uuid.New(), data, t0)

			wantCode(t, err, tt.wantCode)
		})
	}
}

func TestNewPartAcceptsLimits(t *testing.T) {
	data := validPart()
	data.SKU = "AB"
	data.UnitPriceCents = maxPriceCents
	data.Description = ""

	if _, err := NewPart(uuid.New(), data, t0); err != nil {
		t.Errorf("limites válidos foram recusados: %v", err)
	}
}

func TestPartUpdateKeepsStock(t *testing.T) {
	original, err := NewPart(uuid.New(), validPart(), t0)
	if err != nil {
		t.Fatal(err)
	}
	original.Stock = 42

	updated, err := original.Update(PartData{SKU: "VELA-01", Name: "Vela de ignição", UnitPriceCents: 2500}, t1)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if updated.Stock != 42 {
		t.Errorf("editar a peça não pode mexer no estoque: %d", updated.Stock)
	}
	if updated.ID != original.ID || !updated.CreatedAt.Equal(t0) || !updated.UpdatedAt.Equal(t1) {
		t.Errorf("ID/datas erradas: %+v", updated)
	}
	if updated.SKU != "VELA-01" || updated.Description != "" {
		t.Errorf("atualização errada: %+v", updated)
	}
	if original.SKU != "FLT-OLEO-01" {
		t.Error("o original não deveria ser alterado")
	}

	if _, err := original.Update(PartData{}, t1); err == nil {
		t.Error("esperado erro de validação na atualização")
	}
}
