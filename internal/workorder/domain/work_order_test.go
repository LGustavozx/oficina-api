package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
)

var (
	t0 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
)

func service(price int64) Item {
	return Item{Kind: ItemService, RefID: uuid.New(), Name: "Troca de óleo", UnitPrice: money.FromCents(price), Quantity: 1}
}

func part(price int64, qty int) Item {
	return Item{Kind: ItemPart, RefID: uuid.New(), Name: "Filtro", UnitPrice: money.FromCents(price), Quantity: qty}
}

func newOrder(t *testing.T, items ...Item) WorkOrder {
	t.Helper()
	order, err := NewWorkOrder(uuid.New(), uuid.New(), uuid.New(), "  Barulho no motor ", items, t0)
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func inStatus(t *testing.T, status Status, items ...Item) WorkOrder {
	t.Helper()
	order := newOrder(t, items...)
	order.Status = status
	return order
}

func wantCode(t *testing.T, err error, kind apperr.Kind, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != code || appErr.Kind != kind {
		t.Fatalf("erro = %v, esperado %s/%s", err, kind, code)
	}
}

func TestNewWorkOrder(t *testing.T) {
	id, customerID, vehicleID := uuid.New(), uuid.New(), uuid.New()
	items := []Item{service(12000), part(3590, 2)}

	order, err := NewWorkOrder(id, customerID, vehicleID, "  Barulho no motor ", items, t0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if order.ID != id || order.CustomerID != customerID || order.VehicleID != vehicleID || order.Description != "Barulho no motor" {
		t.Errorf("dados errados: %+v", order)
	}
	if order.Status != StatusReceived || len(order.Items) != 2 {
		t.Errorf("status = %s, itens = %d", order.Status, len(order.Items))
	}
	if !order.CreatedAt.Equal(t0) || !order.UpdatedAt.Equal(t0) {
		t.Errorf("datas = %v / %v", order.CreatedAt, order.UpdatedAt)
	}
	if order.StartedAt != nil || order.CompletedAt != nil || order.DeliveredAt != nil {
		t.Error("marcos de execução devem começar vazios")
	}
}

func TestNewWorkOrderWithoutItems(t *testing.T) {
	if _, err := NewWorkOrder(uuid.New(), uuid.New(), uuid.New(), "Revisão", nil, t0); err != nil {
		t.Errorf("itens são opcionais na criação: %v", err)
	}
}

func TestNewWorkOrderInvalid(t *testing.T) {
	tests := []struct {
		name        string
		description string
		items       []Item
		wantCode    string
	}{
		{"descrição vazia", "  ", nil, CodeInvalidDescription},
		{"descrição longa", strings.Repeat("a", 501), nil, CodeInvalidDescription},
		{"descrição com UTF-8 corrompido", "Barulho �", nil, CodeInvalidDescription},
		{"itens demais", "ok", make51Items(), CodeInvalidItems},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWorkOrder(uuid.New(), uuid.New(), uuid.New(), tt.description, tt.items, t0)

			wantCode(t, err, apperr.KindValidation, tt.wantCode)
		})
	}
}

func make51Items() []Item {
	items := make([]Item, 51)
	for i := range items {
		items[i] = part(100, 1)
	}
	return items
}

func TestValidateItems(t *testing.T) {
	repeated := part(100, 1)

	tests := []struct {
		name    string
		items   []Item
		wantErr bool
	}{
		{"serviço com quantidade 1", []Item{service(100)}, false},
		{"peça no limite", []Item{part(100, 1000)}, false},
		{"50 itens", make51Items()[:50], false},
		{"serviço com quantidade 2", []Item{{Kind: ItemService, RefID: uuid.New(), UnitPrice: money.FromCents(1), Quantity: 2}}, true},
		{"peça com quantidade zero", []Item{part(100, 0)}, true},
		{"peça com quantidade negativa", []Item{part(100, -1)}, true},
		{"peça acima do limite", []Item{part(100, 1001)}, true},
		{"tipo desconhecido", []Item{{Kind: "OUTRO", RefID: uuid.New(), Quantity: 1}}, true},
		{"mesma peça repetida", []Item{repeated, repeated}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateItems(tt.items)

			if (err != nil) != tt.wantErr {
				t.Fatalf("erro = %v, esperado erro: %v", err, tt.wantErr)
			}
			if err != nil {
				wantCode(t, err, apperr.KindValidation, CodeInvalidItems)
			}
		})
	}
}

func TestTotal(t *testing.T) {
	order := newOrder(t, service(12000), part(3590, 2), part(500, 3))

	// 120,00 + 2 × 35,90 + 3 × 5,00 = 206,80
	if got := order.Total().Cents(); got != 20680 {
		t.Errorf("Total = %d, esperado 20680", got)
	}
	if got := newOrder(t).Total(); !got.IsZero() {
		t.Errorf("sem itens, o total deveria ser zero, veio %v", got)
	}
}

func TestItemTotal(t *testing.T) {
	if got := part(3590, 3).Total().Cents(); got != 10770 {
		t.Errorf("Item.Total = %d, esperado 10770", got)
	}
}

// Cada ação só vale a partir de um status; em todos os outros deve ser recusada.
func TestTransitionsOnlyFromTheRightStatus(t *testing.T) {
	allStatuses := []Status{StatusReceived, StatusInDiagnosis, StatusAwaitingApproval, StatusInExecution, StatusCompleted, StatusDelivered, StatusCancelled}

	actions := []struct {
		name string
		from Status
		to   Status
		run  func(WorkOrder, time.Time) (WorkOrder, error)
	}{
		{"StartDiagnosis", StatusReceived, StatusInDiagnosis, WorkOrder.StartDiagnosis},
		{"SendQuote", StatusInDiagnosis, StatusAwaitingApproval, WorkOrder.SendQuote},
		{"Approve", StatusAwaitingApproval, StatusInExecution, WorkOrder.Approve},
		{"Reject", StatusAwaitingApproval, StatusCancelled, WorkOrder.Reject},
		{"Complete", StatusInExecution, StatusCompleted, WorkOrder.Complete},
		{"Deliver", StatusCompleted, StatusDelivered, WorkOrder.Deliver},
	}

	for _, action := range actions {
		for _, status := range allStatuses {
			t.Run(action.name+" a partir de "+string(status), func(t *testing.T) {
				order := inStatus(t, status, service(100))

				got, err := action.run(order, t1)

				if status == action.from {
					if err != nil {
						t.Fatalf("deveria ser permitido: %v", err)
					}
					if got.Status != action.to || !got.UpdatedAt.Equal(t1) {
						t.Errorf("status = %s, atualizado em %v", got.Status, got.UpdatedAt)
					}
					return
				}
				wantCode(t, err, apperr.KindConflict, CodeInvalidTransition)
			})
		}
	}
}

func TestTransitionsDoNotMutateOriginal(t *testing.T) {
	original := newOrder(t, service(100))

	if _, err := original.StartDiagnosis(t1); err != nil {
		t.Fatal(err)
	}

	if original.Status != StatusReceived || !original.UpdatedAt.Equal(t0) {
		t.Errorf("o original foi alterado: %+v", original)
	}
}

func TestSendQuoteRequiresItems(t *testing.T) {
	_, err := inStatus(t, StatusInDiagnosis).SendQuote(t1)

	wantCode(t, err, apperr.KindValidation, CodeQuoteWithoutItems)
}

func TestExecutionMilestones(t *testing.T) {
	order := inStatus(t, StatusAwaitingApproval, service(100))

	running, err := order.Approve(t1)
	if err != nil || running.StartedAt == nil || !running.StartedAt.Equal(t1) {
		t.Fatalf("Approve deveria marcar o início: %+v (%v)", running, err)
	}

	t2 := t1.Add(2 * time.Hour)
	done, err := running.Complete(t2)
	if err != nil || done.CompletedAt == nil || !done.CompletedAt.Equal(t2) {
		t.Fatalf("Complete deveria marcar o fim: %+v (%v)", done, err)
	}
	if !done.StartedAt.Equal(t1) {
		t.Error("o início não pode mudar ao finalizar")
	}

	t3 := t2.Add(time.Hour)
	delivered, err := done.Deliver(t3)
	if err != nil || delivered.DeliveredAt == nil || !delivered.DeliveredAt.Equal(t3) {
		t.Fatalf("Deliver deveria marcar a entrega: %+v (%v)", delivered, err)
	}
}

func TestRejectDoesNotStartExecution(t *testing.T) {
	cancelled, err := inStatus(t, StatusAwaitingApproval, service(100)).Reject(t1)

	if err != nil || cancelled.StartedAt != nil {
		t.Errorf("recusar não pode iniciar a execução: %+v (%v)", cancelled, err)
	}
}

func TestReplaceItems(t *testing.T) {
	for _, status := range []Status{StatusReceived, StatusInDiagnosis} {
		t.Run("permitido em "+string(status), func(t *testing.T) {
			order := inStatus(t, status, service(100))

			updated, err := order.ReplaceItems([]Item{part(500, 2)}, t1)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if len(updated.Items) != 1 || updated.Items[0].Kind != ItemPart || !updated.UpdatedAt.Equal(t1) {
				t.Errorf("itens não foram trocados: %+v", updated)
			}
			if len(order.Items) != 1 || order.Items[0].Kind != ItemService {
				t.Error("o original não deveria ser alterado")
			}
		})
	}

	for _, status := range []Status{StatusAwaitingApproval, StatusInExecution, StatusCompleted, StatusDelivered, StatusCancelled} {
		t.Run("recusado em "+string(status), func(t *testing.T) {
			_, err := inStatus(t, status, service(100)).ReplaceItems([]Item{service(100)}, t1)

			wantCode(t, err, apperr.KindConflict, CodeInvalidTransition)
		})
	}

	t.Run("itens inválidos", func(t *testing.T) {
		_, err := newOrder(t).ReplaceItems([]Item{part(100, 0)}, t1)

		wantCode(t, err, apperr.KindValidation, CodeInvalidItems)
	})

	t.Run("pode esvaziar os itens", func(t *testing.T) {
		updated, err := newOrder(t, service(100)).ReplaceItems(nil, t1)

		if err != nil || len(updated.Items) != 0 {
			t.Errorf("itens = %d (%v)", len(updated.Items), err)
		}
	})
}

func TestParseStatusAndLabel(t *testing.T) {
	for _, s := range []Status{StatusReceived, StatusInDiagnosis, StatusAwaitingApproval, StatusInExecution, StatusCompleted, StatusDelivered, StatusCancelled} {
		got, err := ParseStatus(string(s))
		if err != nil || got != s {
			t.Errorf("ParseStatus(%q) = %q, %v", s, got, err)
		}
		if s.Label() == "" {
			t.Errorf("status %s sem rótulo", s)
		}
	}

	for _, raw := range []string{"", "received", "ABC"} {
		_, err := ParseStatus(raw)
		wantCode(t, err, apperr.KindValidation, CodeInvalidStatus)
	}
}

func TestInvalidTransitionMessageNamesCurrentStatus(t *testing.T) {
	_, err := inStatus(t, StatusDelivered).StartDiagnosis(t1)

	appErr, _ := apperr.As(err)
	if appErr == nil || !strings.Contains(appErr.Message, "entregue") {
		t.Errorf("a mensagem deveria citar o status atual: %v", err)
	}
}
