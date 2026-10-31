package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/workorder/domain"
)

// DocumentHeader leva o CPF/CNPJ do cliente. Vai em cabeçalho, e não na URL,
// para o documento não ficar registrado em históricos e logs de servidores.
const DocumentHeader = "X-Customer-Document"

const codeDocumentRequired = "DOCUMENT_REQUIRED"

type customerService interface {
	GetForCustomer(ctx context.Context, number int64, rawDocument string) (domain.WorkOrder, error)
	ApproveForCustomer(ctx context.Context, number int64, rawDocument string) (domain.WorkOrder, error)
	RejectForCustomer(ctx context.Context, number int64, rawDocument string) (domain.WorkOrder, error)
}

// PublicHandler atende o cliente, sem login: ele se identifica pelo número da
// ordem de serviço e pelo próprio CPF/CNPJ.
type PublicHandler struct {
	svc    customerService
	logger *slog.Logger
}

func NewPublicHandler(svc customerService, logger *slog.Logger) *PublicHandler {
	return &PublicHandler{svc: svc, logger: logger}
}

func (h *PublicHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /public/work-orders/{number}", h.respond(h.svc.GetForCustomer))
	mux.HandleFunc("POST /public/work-orders/{number}/approve", h.respond(h.svc.ApproveForCustomer))
	mux.HandleFunc("POST /public/work-orders/{number}/reject", h.respond(h.svc.RejectForCustomer))
}

// A resposta pública só traz o que o cliente precisa; nenhum identificador interno.

type publicItem struct {
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int    `json:"quantity"`
	TotalCents     int64  `json:"total_cents"`
}

type publicOrder struct {
	Number      int64        `json:"number"`
	Status      string       `json:"status"`
	StatusLabel string       `json:"status_label"`
	Description string       `json:"description"`
	Items       []publicItem `json:"items"`
	TotalCents  int64        `json:"total_cents"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	StartedAt   *time.Time   `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at"`
	DeliveredAt *time.Time   `json:"delivered_at"`
}

func toPublic(o domain.WorkOrder) publicOrder {
	items := make([]publicItem, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, publicItem{
			Kind:           string(item.Kind),
			Name:           item.Name,
			UnitPriceCents: item.UnitPrice.Cents(),
			Quantity:       item.Quantity,
			TotalCents:     item.Total().Cents(),
		})
	}

	return publicOrder{
		Number:      o.Number,
		Status:      string(o.Status),
		StatusLabel: o.Status.Label(),
		Description: o.Description,
		Items:       items,
		TotalCents:  o.Total().Cents(),
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
		StartedAt:   o.StartedAt,
		CompletedAt: o.CompletedAt,
		DeliveredAt: o.DeliveredAt,
	}
}

func (h *PublicHandler) respond(run func(context.Context, int64, string) (domain.WorkOrder, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		number, err := strconv.ParseInt(r.PathValue("number"), 10, 64)
		if err != nil || number < 1 {
			httpserver.WriteError(w, h.logger, apperr.Validation(httpserver.CodeInvalidID, "número da ordem de serviço inválido"))
			return
		}

		document := r.Header.Get(DocumentHeader)
		if document == "" {
			httpserver.WriteError(w, h.logger, apperr.Validation(codeDocumentRequired, "informe o CPF/CNPJ no cabeçalho "+DocumentHeader))
			return
		}

		order, err := run(r.Context(), number, document)
		if err != nil {
			httpserver.WriteError(w, h.logger, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, toPublic(order))
	}
}
