package domain

import (
	"errors"
	"testing"

	"oficina-api/internal/shared/apperr"
)

func TestNewDocumentValid(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantValue     string
		wantType      DocumentType
		wantFormatted string
	}{
		{"CPF formatado", "529.982.247-25", "52998224725", DocumentCPF, "529.982.247-25"},
		{"CPF só dígitos", "52998224725", "52998224725", DocumentCPF, "529.982.247-25"},
		{"CPF com espaços", " 529.982.247-25 ", "52998224725", DocumentCPF, "529.982.247-25"},
		{"CNPJ formatado", "11.222.333/0001-81", "11222333000181", DocumentCNPJ, "11.222.333/0001-81"},
		{"CNPJ só dígitos", "11222333000181", "11222333000181", DocumentCNPJ, "11.222.333/0001-81"},
		{"CNPJ alfanumérico", "12.ABC.345/01DE-35", "12ABC34501DE35", DocumentCNPJ, "12.ABC.345/01DE-35"},
		{"CNPJ alfanumérico em minúsculas", "12abc34501de35", "12ABC34501DE35", DocumentCNPJ, "12.ABC.345/01DE-35"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewDocument(tt.raw)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if doc.String() != tt.wantValue {
				t.Errorf("String() = %q, esperado %q", doc.String(), tt.wantValue)
			}
			if doc.Type() != tt.wantType {
				t.Errorf("Type() = %q, esperado %q", doc.Type(), tt.wantType)
			}
			if doc.Formatted() != tt.wantFormatted {
				t.Errorf("Formatted() = %q, esperado %q", doc.Formatted(), tt.wantFormatted)
			}
		})
	}
}

func TestNewDocumentInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"vazio", ""},
		{"CPF com dígito verificador errado", "529.982.247-26"},
		{"CPF com 10 dígitos", "5299822472"},
		{"CPF com dígitos repetidos", "111.111.111-11"},
		{"CPF com letras", "529.982.24A-25"},
		{"CNPJ com dígito verificador errado", "11.222.333/0001-82"},
		{"CNPJ com dígitos repetidos", "00.000.000/0000-00"},
		{"CNPJ alfanumérico com DV errado", "12.ABC.345/01DE-36"},
		{"CNPJ com letra no dígito verificador", "11.222.333/0001-8A"},
		{"CNPJ com caractere especial", "11.222.33#/0001-81"},
		{"tamanho intermediário", "123456789012"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDocument(tt.raw)
			if err == nil {
				t.Fatal("esperado erro")
			}
			appErr, ok := apperr.As(err)
			if !ok || appErr.Code != CodeInvalidDocument || appErr.Kind != apperr.KindValidation {
				t.Errorf("erro = %v, esperado validação %s", err, CodeInvalidDocument)
			}
		})
	}
}

func TestNewDocumentErrorIsAppErr(t *testing.T) {
	_, err := NewDocument("x")

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("o erro deveria ser *apperr.Error")
	}
}
