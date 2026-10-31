package domain

import (
	"strings"
	"testing"

	"oficina-api/internal/shared/apperr"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		plain   string
		wantErr bool
	}{
		{"mínimo aceito", "12345678", false},
		{"máximo aceito", strings.Repeat("a", 72), false},
		{"curta", "1234567", true},
		{"vazia", "", true},
		{"longa demais para o bcrypt", strings.Repeat("a", 73), true},
		{"8 caracteres com acento", "senhaçãõ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.plain)
			if (err != nil) != tt.wantErr {
				t.Fatalf("erro = %v, esperado erro: %v", err, tt.wantErr)
			}
			if err != nil {
				if appErr, ok := apperr.As(err); !ok || appErr.Code != CodeInvalidPassword {
					t.Errorf("código errado: %v", err)
				}
			}
		})
	}
}
