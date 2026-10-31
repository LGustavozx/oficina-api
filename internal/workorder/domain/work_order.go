// Package domain do contexto workorder: a ordem de serviço, do recebimento do veículo à entrega.
package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/shared/text"
)

const (
	CodeInvalidDescription = "INVALID_DESCRIPTION"
	CodeInvalidItems       = "INVALID_ITEMS"
	CodeInvalidStatus      = "INVALID_STATUS"
	CodeQuoteWithoutItems  = "QUOTE_WITHOUT_ITEMS"
	CodeInvalidTransition  = "INVALID_STATUS_TRANSITION"
	CodeWorkOrderNotFound  = "WORK_ORDER_NOT_FOUND"
	CodeWorkOrderChanged   = "WORK_ORDER_CHANGED"
	maxDescription         = 500
	maxItems               = 50
	maxPartQuantity        = 1000
	serviceQuantity        = 1
)

type Status string

const (
	StatusReceived         Status = "RECEIVED"
	StatusInDiagnosis      Status = "IN_DIAGNOSIS"
	StatusAwaitingApproval Status = "AWAITING_APPROVAL"
	StatusInExecution      Status = "IN_EXECUTION"
	StatusCompleted        Status = "COMPLETED"
	StatusDelivered        Status = "DELIVERED"
	StatusCancelled        Status = "CANCELLED"
)

var statusLabels = map[Status]string{
	StatusReceived:         "recebida",
	StatusInDiagnosis:      "em diagnóstico",
	StatusAwaitingApproval: "aguardando aprovação",
	StatusInExecution:      "em execução",
	StatusCompleted:        "finalizada",
	StatusDelivered:        "entregue",
	StatusCancelled:        "cancelada",
}

// ParseStatus valida um status vindo de fora (por exemplo, o filtro da listagem).
func ParseStatus(raw string) (Status, error) {
	status := Status(raw)
	if _, ok := statusLabels[status]; !ok {
		return "", apperr.Validation(CodeInvalidStatus, "status inválido")
	}
	return status, nil
}

func (s Status) Label() string { return statusLabels[s] }

type ItemKind string

const (
	ItemService ItemKind = "SERVICE"
	ItemPart    ItemKind = "PART"
)

// Item guarda o nome e o preço do momento em que foi incluído: o orçamento
// não muda se o catálogo for alterado depois.
type Item struct {
	Kind      ItemKind
	RefID     uuid.UUID // id do serviço ou da peça
	Name      string
	UnitPrice money.Money
	Quantity  int
}

func (i Item) Total() money.Money { return i.UnitPrice.Mul(int64(i.Quantity)) }

type WorkOrder struct {
	ID          uuid.UUID
	Number      int64 // sequencial, gerado pelo banco
	CustomerID  uuid.UUID
	VehicleID   uuid.UUID
	Description string
	Status      Status
	Items       []Item
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Marcos usados para medir o tempo de execução.
	StartedAt   *time.Time
	CompletedAt *time.Time
	DeliveredAt *time.Time
}

// Total é o valor do orçamento: soma de preço × quantidade de todos os itens.
func (w WorkOrder) Total() money.Money {
	total := money.Zero()
	for _, item := range w.Items {
		total = total.Add(item.Total())
	}
	return total
}

func NewWorkOrder(id, customerID, vehicleID uuid.UUID, description string, items []Item, now time.Time) (WorkOrder, error) {
	description, ok := text.Clean(description, 1, maxDescription)
	if !ok {
		return WorkOrder{}, apperr.Validation(CodeInvalidDescription, "descreva o problema, com até 500 caracteres")
	}
	if err := validateItems(items); err != nil {
		return WorkOrder{}, err
	}

	return WorkOrder{
		ID: id, CustomerID: customerID, VehicleID: vehicleID, Description: description,
		Status: StatusReceived, Items: items, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func validateItems(items []Item) error {
	if len(items) > maxItems {
		return apperr.Validation(CodeInvalidItems, fmt.Sprintf("a ordem de serviço aceita até %d itens", maxItems))
	}

	seen := map[[2]string]bool{}
	for _, item := range items {
		key := [2]string{string(item.Kind), item.RefID.String()}
		if seen[key] {
			return apperr.Validation(CodeInvalidItems, "há itens repetidos; use a quantidade em vez de repetir a peça")
		}
		seen[key] = true

		switch {
		case item.Kind == ItemService && item.Quantity != serviceQuantity:
			return apperr.Validation(CodeInvalidItems, "a quantidade de um serviço é sempre 1")
		case item.Kind == ItemPart && (item.Quantity < 1 || item.Quantity > maxPartQuantity):
			return apperr.Validation(CodeInvalidItems, fmt.Sprintf("a quantidade de uma peça deve estar entre 1 e %d", maxPartQuantity))
		case item.Kind != ItemService && item.Kind != ItemPart:
			return apperr.Validation(CodeInvalidItems, "tipo de item inválido")
		}
	}
	return nil
}

// As transições abaixo são a única forma de mudar o status. Cada uma só vale
// a partir de um status específico, e devolve uma cópia da ordem de serviço.

func (w WorkOrder) StartDiagnosis(now time.Time) (WorkOrder, error) {
	return w.moveTo(StatusInDiagnosis, now, StatusReceived)
}

// ReplaceItems troca todos os itens. Só é possível antes de o orçamento ser enviado.
func (w WorkOrder) ReplaceItems(items []Item, now time.Time) (WorkOrder, error) {
	if w.Status != StatusReceived && w.Status != StatusInDiagnosis {
		return WorkOrder{}, invalidTransition(w.Status)
	}
	if err := validateItems(items); err != nil {
		return WorkOrder{}, err
	}
	w.Items, w.UpdatedAt = items, now
	return w, nil
}

// SendQuote envia o orçamento para aprovação do cliente.
func (w WorkOrder) SendQuote(now time.Time) (WorkOrder, error) {
	if w.Status == StatusInDiagnosis && len(w.Items) == 0 {
		return WorkOrder{}, apperr.Validation(CodeQuoteWithoutItems, "inclua ao menos um serviço ou peça antes de enviar o orçamento")
	}
	return w.moveTo(StatusAwaitingApproval, now, StatusInDiagnosis)
}

// Approve inicia a execução. A baixa do estoque das peças acontece junto,
// na camada de aplicação, dentro da mesma transação.
func (w WorkOrder) Approve(now time.Time) (WorkOrder, error) {
	approved, err := w.moveTo(StatusInExecution, now, StatusAwaitingApproval)
	if err != nil {
		return WorkOrder{}, err
	}
	approved.StartedAt = &now
	return approved, nil
}

func (w WorkOrder) Reject(now time.Time) (WorkOrder, error) {
	return w.moveTo(StatusCancelled, now, StatusAwaitingApproval)
}

func (w WorkOrder) Complete(now time.Time) (WorkOrder, error) {
	completed, err := w.moveTo(StatusCompleted, now, StatusInExecution)
	if err != nil {
		return WorkOrder{}, err
	}
	completed.CompletedAt = &now
	return completed, nil
}

func (w WorkOrder) Deliver(now time.Time) (WorkOrder, error) {
	delivered, err := w.moveTo(StatusDelivered, now, StatusCompleted)
	if err != nil {
		return WorkOrder{}, err
	}
	delivered.DeliveredAt = &now
	return delivered, nil
}

func (w WorkOrder) moveTo(next Status, now time.Time, allowedFrom Status) (WorkOrder, error) {
	if w.Status != allowedFrom {
		return WorkOrder{}, invalidTransition(w.Status)
	}
	w.Status, w.UpdatedAt = next, now
	return w, nil
}

func invalidTransition(current Status) error {
	return apperr.Conflict(CodeInvalidTransition, fmt.Sprintf("a ordem de serviço está %s e não aceita esta ação", current.Label()))
}

type Repository interface {
	// Create grava a ordem de serviço e devolve-a com o Number gerado.
	Create(ctx context.Context, order WorkOrder) (WorkOrder, error)

	// Save grava o estado atual (status, marcos e itens), mas só se o status
	// gravado ainda for expected. Se outra operação mudou a ordem de serviço
	// no meio do caminho, devolve WORK_ORDER_CHANGED.
	Save(ctx context.Context, order WorkOrder, expected Status) error

	FindByID(ctx context.Context, id uuid.UUID) (WorkOrder, error)

	// status vazio significa "sem filtro".
	List(ctx context.Context, status Status, limit, offset int) ([]WorkOrder, error)
}
