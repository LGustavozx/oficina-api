package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewStockMovement(t *testing.T) {
	id, partID := uuid.New(), uuid.New()

	m, err := NewStockMovement(id, partID, MovementData{Type: "IN", Quantity: 10, Reason: "  Compra NF 123 "}, t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if m.ID != id || m.PartID != partID || m.Type != MovementIn || m.Quantity != 10 || m.Reason != "Compra NF 123" || !m.CreatedAt.Equal(t0) {
		t.Errorf("movimentação errada: %+v", m)
	}

	out, err := NewStockMovement(id, partID, MovementData{Type: "OUT", Quantity: 1, Reason: "Perda"}, t0)
	if err != nil || out.Type != MovementOut {
		t.Errorf("saída = %+v, %v", out, err)
	}
}

func TestNewStockMovementInvalid(t *testing.T) {
	tests := []struct {
		name     string
		data     MovementData
		wantCode string
	}{
		{"tipo vazio", MovementData{Quantity: 1, Reason: "x"}, CodeInvalidMovementType},
		{"tipo em minúsculas", MovementData{Type: "in", Quantity: 1, Reason: "x"}, CodeInvalidMovementType},
		{"tipo desconhecido", MovementData{Type: "ADJUST", Quantity: 1, Reason: "x"}, CodeInvalidMovementType},
		{"quantidade zero", MovementData{Type: "IN", Quantity: 0, Reason: "x"}, CodeInvalidQuantity},
		{"quantidade negativa", MovementData{Type: "OUT", Quantity: -3, Reason: "x"}, CodeInvalidQuantity},
		{"quantidade acima do teto", MovementData{Type: "IN", Quantity: MaxStock + 1, Reason: "x"}, CodeInvalidQuantity},
		{"motivo vazio", MovementData{Type: "IN", Quantity: 1, Reason: "  "}, CodeInvalidReason},
		{"motivo longo", MovementData{Type: "IN", Quantity: 1, Reason: strings.Repeat("a", 201)}, CodeInvalidReason},
		{"motivo com UTF-8 corrompido", MovementData{Type: "IN", Quantity: 1, Reason: "Compra �"}, CodeInvalidReason},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStockMovement(uuid.New(), uuid.New(), tt.data, t0)

			wantCode(t, err, tt.wantCode)
		})
	}
}

func TestNewStockMovementAcceptsMaxQuantity(t *testing.T) {
	if _, err := NewStockMovement(uuid.New(), uuid.New(), MovementData{Type: "IN", Quantity: MaxStock, Reason: "x"}, t0); err != nil {
		t.Errorf("quantidade no teto foi recusada: %v", err)
	}
}
