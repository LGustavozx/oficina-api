// Package domain do contexto inventory: peças, insumos e o controle do estoque.
package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/shared/text"
)

const (
	CodeInvalidSKU         = "INVALID_SKU"
	CodeInvalidPartName    = "INVALID_PART_NAME"
	CodeInvalidDescription = "INVALID_DESCRIPTION"
	CodeInvalidPrice       = "INVALID_PRICE"
	CodePartNotFound       = "PART_NOT_FOUND"
	CodeSKUTaken           = "SKU_ALREADY_REGISTERED"
	CodePartInUse          = "PART_IN_USE"

	minNameLength  = 2
	maxNameLength  = 100
	maxDescription = 500
	maxPriceCents  = 100_000_000 // R$ 1.000.000,00
)

// Letras maiúsculas, dígitos e hífen, de 2 a 30 caracteres.
var skuPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,29}$`)

type PartData struct {
	SKU            string
	Name           string
	Description    string
	UnitPriceCents int64
}

// Part é uma peça ou insumo. O saldo (Stock) só muda por movimentação de
// estoque, nunca pela edição da peça.
type Part struct {
	ID          uuid.UUID
	SKU         string
	Name        string
	Description string // opcional
	UnitPrice   money.Money
	Stock       int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewPart(id uuid.UUID, data PartData, now time.Time) (Part, error) {
	return Part{ID: id, CreatedAt: now}.apply(data, now)
}

// Update devolve uma cópia com os novos dados cadastrais; o saldo não muda.
func (p Part) Update(data PartData, now time.Time) (Part, error) {
	return p.apply(data, now)
}

func (p Part) apply(data PartData, now time.Time) (Part, error) {
	sku := strings.ToUpper(strings.TrimSpace(data.SKU))
	if !skuPattern.MatchString(sku) {
		return Part{}, apperr.Validation(CodeInvalidSKU, "o código deve ter de 2 a 30 caracteres: letras, números e hífen")
	}

	name, ok := text.Clean(data.Name, minNameLength, maxNameLength)
	if !ok {
		return Part{}, apperr.Validation(CodeInvalidPartName, "o nome deve ter entre 2 e 100 caracteres")
	}

	description, ok := text.Clean(data.Description, 0, maxDescription)
	if !ok {
		return Part{}, apperr.Validation(CodeInvalidDescription, "a descrição deve ter até 500 caracteres")
	}

	// Preço zero também é recusado: um campo omitido no JSON chega como 0.
	if data.UnitPriceCents < 1 || data.UnitPriceCents > maxPriceCents {
		return Part{}, apperr.Validation(CodeInvalidPrice, fmt.Sprintf("o preço deve estar entre 1 centavo e %s", money.FromCents(maxPriceCents)))
	}

	p.SKU, p.Name, p.Description, p.UnitPrice, p.UpdatedAt = sku, name, description, money.FromCents(data.UnitPriceCents), now
	return p, nil
}

type PartRepository interface {
	Create(ctx context.Context, part Part) error
	// Update altera só os dados cadastrais, nunca o saldo.
	Update(ctx context.Context, part Part) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (Part, error)
	FindBySKU(ctx context.Context, sku string) (Part, error)
	List(ctx context.Context, limit, offset int) ([]Part, error)

	// RecordMovement grava a movimentação e atualiza o saldo de forma atômica,
	// devolvendo a peça com o novo saldo. Uma saída maior que o saldo falha com
	// INSUFFICIENT_STOCK, mesmo com várias requisições simultâneas.
	RecordMovement(ctx context.Context, movement StockMovement) (Part, error)
	ListMovements(ctx context.Context, partID uuid.UUID, limit, offset int) ([]StockMovement, error)
}
