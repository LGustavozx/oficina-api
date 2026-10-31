package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/text"
)

const (
	CodeInvalidMovementType = "INVALID_MOVEMENT_TYPE"
	CodeInvalidQuantity     = "INVALID_QUANTITY"
	CodeInvalidReason       = "INVALID_REASON"
	CodeInsufficientStock   = "INSUFFICIENT_STOCK"
	CodeStockLimitExceeded  = "STOCK_LIMIT_EXCEEDED"

	// MaxStock é o maior saldo de uma peça; também é o teto por movimentação.
	MaxStock = 1_000_000

	maxReasonLength = 200
)

type MovementType string

const (
	MovementIn  MovementType = "IN"  // entrada: compra, devolução, ajuste para mais
	MovementOut MovementType = "OUT" // saída: consumo, perda, ajuste para menos
)

type MovementData struct {
	Type     string
	Quantity int
	Reason   string
}

type StockMovement struct {
	ID        uuid.UUID
	PartID    uuid.UUID
	Type      MovementType
	Quantity  int
	Reason    string
	CreatedAt time.Time
}

func NewStockMovement(id, partID uuid.UUID, data MovementData, now time.Time) (StockMovement, error) {
	movementType := MovementType(data.Type)
	if movementType != MovementIn && movementType != MovementOut {
		return StockMovement{}, apperr.Validation(CodeInvalidMovementType, "o tipo deve ser IN (entrada) ou OUT (saída)")
	}

	if data.Quantity < 1 || data.Quantity > MaxStock {
		return StockMovement{}, apperr.Validation(CodeInvalidQuantity, fmt.Sprintf("a quantidade deve estar entre 1 e %d", MaxStock))
	}

	reason, ok := text.Clean(data.Reason, 1, maxReasonLength)
	if !ok {
		return StockMovement{}, apperr.Validation(CodeInvalidReason, "informe o motivo, com até 200 caracteres")
	}

	return StockMovement{ID: id, PartID: partID, Type: movementType, Quantity: data.Quantity, Reason: reason, CreatedAt: now}, nil
}
