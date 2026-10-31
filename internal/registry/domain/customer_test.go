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

func validData() CustomerData {
	return CustomerData{
		Name:     "  Maria Souza ",
		Document: "529.982.247-25",
		Email:    " Maria@Email.com ",
		Phone:    "(11) 98765-4321",
	}
}

func TestNewCustomerNormalizesData(t *testing.T) {
	id := uuid.New()

	c, err := NewCustomer(id, validData(), t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if c.ID != id || c.Name != "Maria Souza" || c.Document.String() != "52998224725" ||
		c.Email != "maria@email.com" || c.Phone != "11987654321" {
		t.Errorf("dados não normalizados: %+v", c)
	}
	if !c.CreatedAt.Equal(t0) || !c.UpdatedAt.Equal(t0) {
		t.Errorf("datas = %v / %v", c.CreatedAt, c.UpdatedAt)
	}
}

func TestNewCustomerOptionalFields(t *testing.T) {
	data := CustomerData{Name: "Oficina Teste Ltda", Document: "11.222.333/0001-81"}

	c, err := NewCustomer(uuid.New(), data, t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if c.Email != "" || c.Phone != "" || c.Document.Type() != DocumentCNPJ {
		t.Errorf("campos opcionais/tipo errados: %+v", c)
	}
}

func TestNewCustomerInvalid(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*CustomerData)
		wantCode string
	}{
		{"nome vazio", func(d *CustomerData) { d.Name = "  " }, CodeInvalidName},
		{"nome de 1 letra", func(d *CustomerData) { d.Name = "A" }, CodeInvalidName},
		{"nome longo demais", func(d *CustomerData) { d.Name = strings.Repeat("a", 121) }, CodeInvalidName},
		{"nome com UTF-8 corrompido", func(d *CustomerData) { d.Name = "Auto Pe�as" }, CodeInvalidName},
		{"documento inválido", func(d *CustomerData) { d.Document = "123" }, CodeInvalidDocument},
		{"e-mail inválido", func(d *CustomerData) { d.Email = "sem-arroba" }, CodeInvalidEmail},
		{"e-mail com nome de exibição", func(d *CustomerData) { d.Email = "Maria <m@e.com>" }, CodeInvalidEmail},
		{"telefone curto", func(d *CustomerData) { d.Phone = "12345" }, CodeInvalidPhone},
		{"telefone longo", func(d *CustomerData) { d.Phone = "12345678901234" }, CodeInvalidPhone},
		{"telefone com letras", func(d *CustomerData) { d.Phone = "1198765abcd" }, CodeInvalidPhone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validData()
			tt.mutate(&data)

			_, err := NewCustomer(uuid.New(), data, t0)

			appErr, ok := apperr.As(err)
			if !ok || appErr.Code != tt.wantCode || appErr.Kind != apperr.KindValidation {
				t.Errorf("erro = %v, esperado %s", err, tt.wantCode)
			}
		})
	}
}

func TestCustomerUpdate(t *testing.T) {
	original, err := NewCustomer(uuid.New(), validData(), t0)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := original.Update(CustomerData{Name: "Maria Silva", Document: "11.222.333/0001-81"}, t1)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if updated.ID != original.ID || !updated.CreatedAt.Equal(t0) {
		t.Error("ID e data de criação não podem mudar")
	}
	if updated.Name != "Maria Silva" || updated.Document.Type() != DocumentCNPJ || updated.Email != "" || !updated.UpdatedAt.Equal(t1) {
		t.Errorf("atualização errada: %+v", updated)
	}
	if original.Name != "Maria Souza" {
		t.Error("o original não deveria ser alterado")
	}

	if _, err := original.Update(CustomerData{Name: "x"}, t1); err == nil {
		t.Error("esperado erro de validação na atualização")
	}
}
