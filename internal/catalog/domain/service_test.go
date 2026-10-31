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

func validData() ServiceData {
	return ServiceData{Name: "  Troca de óleo ", Description: " Óleo e filtro ", PriceCents: 12000, EstimatedMinutes: 45}
}

func TestNewServiceNormalizesData(t *testing.T) {
	id := uuid.New()

	s, err := NewService(id, validData(), t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if s.ID != id || s.Name != "Troca de óleo" || s.Description != "Óleo e filtro" || s.Price.Cents() != 12000 || s.EstimatedMinutes != 45 {
		t.Errorf("dados errados: %+v", s)
	}
	if !s.CreatedAt.Equal(t0) || !s.UpdatedAt.Equal(t0) {
		t.Errorf("datas = %v / %v", s.CreatedAt, s.UpdatedAt)
	}
}

func TestNewServiceOptionalDescription(t *testing.T) {
	data := validData()
	data.Description = ""

	s, err := NewService(uuid.New(), data, t0)
	if err != nil || s.Description != "" {
		t.Errorf("descrição vazia deveria ser aceita: %+v, %v", s, err)
	}
}

func TestNewServiceLimits(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ServiceData)
		wantErr bool
	}{
		{"preço mínimo", func(d *ServiceData) { d.PriceCents = 1 }, false},
		{"preço máximo", func(d *ServiceData) { d.PriceCents = maxPriceCents }, false},
		{"preço acima do máximo", func(d *ServiceData) { d.PriceCents = maxPriceCents + 1 }, true},
		{"duração mínima", func(d *ServiceData) { d.EstimatedMinutes = 1 }, false},
		{"duração máxima", func(d *ServiceData) { d.EstimatedMinutes = maxDurationMins }, false},
		{"duração acima do máximo", func(d *ServiceData) { d.EstimatedMinutes = maxDurationMins + 1 }, true},
		{"descrição no limite", func(d *ServiceData) { d.Description = strings.Repeat("a", 500) }, false},
		{"descrição acima do limite", func(d *ServiceData) { d.Description = strings.Repeat("a", 501) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validData()
			tt.mutate(&data)

			_, err := NewService(uuid.New(), data, t0)

			if (err != nil) != tt.wantErr {
				t.Errorf("erro = %v, esperado erro: %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewServiceInvalid(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*ServiceData)
		wantCode string
	}{
		{"nome vazio", func(d *ServiceData) { d.Name = " " }, CodeInvalidServiceName},
		{"nome curto", func(d *ServiceData) { d.Name = "a" }, CodeInvalidServiceName},
		{"nome longo", func(d *ServiceData) { d.Name = strings.Repeat("a", 101) }, CodeInvalidServiceName},
		{"nome com UTF-8 corrompido", func(d *ServiceData) { d.Name = "Troca de �leo" }, CodeInvalidServiceName},
		{"descrição com UTF-8 corrompido", func(d *ServiceData) { d.Description = "�" }, CodeInvalidDescription},
		{"preço zero (campo omitido)", func(d *ServiceData) { d.PriceCents = 0 }, CodeInvalidPrice},
		{"preço negativo", func(d *ServiceData) { d.PriceCents = -100 }, CodeInvalidPrice},
		{"duração zero", func(d *ServiceData) { d.EstimatedMinutes = 0 }, CodeInvalidDuration},
		{"duração negativa", func(d *ServiceData) { d.EstimatedMinutes = -5 }, CodeInvalidDuration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validData()
			tt.mutate(&data)

			_, err := NewService(uuid.New(), data, t0)

			appErr, ok := apperr.As(err)
			if !ok || appErr.Code != tt.wantCode || appErr.Kind != apperr.KindValidation {
				t.Errorf("erro = %v, esperado %s", err, tt.wantCode)
			}
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	original, err := NewService(uuid.New(), validData(), t0)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := original.Update(ServiceData{Name: "Alinhamento", PriceCents: 8000, EstimatedMinutes: 30}, t1)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if updated.ID != original.ID || !updated.CreatedAt.Equal(t0) {
		t.Error("ID e data de criação não podem mudar")
	}
	if updated.Name != "Alinhamento" || updated.Price.Cents() != 8000 || updated.Description != "" || !updated.UpdatedAt.Equal(t1) {
		t.Errorf("atualização errada: %+v", updated)
	}
	if original.Name != "Troca de óleo" {
		t.Error("o original não deveria ser alterado")
	}

	if _, err := original.Update(ServiceData{}, t1); err == nil {
		t.Error("esperado erro de validação na atualização")
	}
}
