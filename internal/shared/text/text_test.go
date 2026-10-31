package text

import (
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		min, max int
		want     string
		wantOK   bool
	}{
		{"apara espaços", "  Fiat  ", 1, 10, "Fiat", true},
		{"mínimo exato", "ab", 2, 5, "ab", true},
		{"máximo exato", "abcde", 2, 5, "abcde", true},
		{"conta caracteres e não bytes", "ççç", 1, 3, "ççç", true},
		{"curto demais", "a", 2, 5, "", false},
		{"longo demais", "abcdef", 2, 5, "", false},
		{"só espaços", "   ", 1, 5, "", false},
		{"vazio permitido quando min é 0", "", 0, 5, "", true},
		{"UTF-8 corrompido", "Pe�as", 1, 10, "", false},
		{"muito longo", strings.Repeat("a", 1000), 1, 10, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Clean(tt.raw, tt.min, tt.max)

			if ok != tt.wantOK || got != tt.want {
				t.Errorf("Clean(%q, %d, %d) = %q, %v; esperado %q, %v", tt.raw, tt.min, tt.max, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
