// Package http do contexto workorder: rotas administrativas da ordem de serviço.
package http

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/workorder/application"
	"oficina-api/internal/workorder/domain"
)

type workOrderService interface {
	Create(ctx context.Context, vehicleID uuid.UUID, description string, in application.ItemsInput) (domain.WorkOrder, error)
	Get(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	List(ctx context.Context, rawStatus string, limit, offset int) ([]domain.WorkOrder, error)
	ReplaceItems(ctx context.Context, id uuid.UUID, in application.ItemsInput) (domain.WorkOrder, error)
	StartDiagnosis(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	SendQuote(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	Approve(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	Reject(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	Complete(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	Deliver(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error)
	ExecutionStats(ctx context.Context, from, to time.Time) (domain.ExecutionStats, error)
}

type Handler struct {
	svc    workOrderService
	logger *slog.Logger
}

func NewHandler(svc workOrderService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	route := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(handler))
	}

	route("POST /admin/work-orders", h.create)
	route("GET /admin/work-orders", h.list)
	route("GET /admin/work-orders/{id}", h.get)
	route("PUT /admin/work-orders/{id}/items", h.replaceItems)

	route("POST /admin/work-orders/{id}/start-diagnosis", h.action(h.svc.StartDiagnosis))
	route("POST /admin/work-orders/{id}/send-quote", h.action(h.svc.SendQuote))
	route("POST /admin/work-orders/{id}/approve", h.action(h.svc.Approve))
	route("POST /admin/work-orders/{id}/reject", h.action(h.svc.Reject))
	route("POST /admin/work-orders/{id}/complete", h.action(h.svc.Complete))
	route("POST /admin/work-orders/{id}/deliver", h.action(h.svc.Deliver))

	route("GET /admin/metrics/execution-time", h.executionTime)
}

type executionTimeResponse struct {
	CompletedOrders         int     `json:"completed_orders"`
	AverageExecutionMinutes float64 `json:"average_execution_minutes"`
}

// executionTime aceita ?from= e ?to= (YYYY-MM-DD); o intervalo vale pela data de
// finalização, com from incluso e to exclusivo. Sem eles, considera tudo.
func (h *Handler) executionTime(w http.ResponseWriter, r *http.Request) {
	from, err := parseDate(r, "from")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	to, err := parseDate(r, "to")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		httpserver.WriteError(w, h.logger, apperr.Validation(codeInvalidPeriod, "from deve ser anterior a to"))
		return
	}

	stats, err := h.svc.ExecutionStats(r.Context(), from, to)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	httpserver.WriteJSON(w, http.StatusOK, executionTimeResponse{
		CompletedOrders:         stats.CompletedOrders,
		AverageExecutionMinutes: math.Round(stats.AverageDuration.Minutes()*10) / 10,
	})
}

const codeInvalidPeriod = "INVALID_PERIOD"

func parseDate(r *http.Request, name string) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return time.Time{}, nil
	}

	date, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, apperr.Validation(codeInvalidPeriod, name+" deve ter o formato AAAA-MM-DD")
	}
	return date, nil
}

type partRequest struct {
	PartID   string `json:"part_id"`
	Quantity int    `json:"quantity"`
}

type itemsRequest struct {
	Services []string      `json:"services"`
	Parts    []partRequest `json:"parts"`
}

func (r itemsRequest) input() (application.ItemsInput, error) {
	in := application.ItemsInput{}

	for _, raw := range r.Services {
		id, err := parseID(raw, "identificador de serviço inválido")
		if err != nil {
			return application.ItemsInput{}, err
		}
		in.ServiceIDs = append(in.ServiceIDs, id)
	}

	for _, p := range r.Parts {
		id, err := parseID(p.PartID, "identificador de peça inválido")
		if err != nil {
			return application.ItemsInput{}, err
		}
		in.Parts = append(in.Parts, application.PartInput{PartID: id, Quantity: p.Quantity})
	}
	return in, nil
}

type createRequest struct {
	VehicleID   string `json:"vehicle_id"`
	Description string `json:"description"`
	itemsRequest
}

func parseID(raw, message string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Validation(httpserver.CodeInvalidID, message)
	}
	return id, nil
}

type itemResponse struct {
	Kind           string `json:"kind"`
	RefID          string `json:"ref_id"`
	Name           string `json:"name"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int    `json:"quantity"`
	TotalCents     int64  `json:"total_cents"`
}

type workOrderResponse struct {
	ID          string         `json:"id"`
	Number      int64          `json:"number"`
	CustomerID  string         `json:"customer_id"`
	VehicleID   string         `json:"vehicle_id"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Items       []itemResponse `json:"items"`
	TotalCents  int64          `json:"total_cents"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	DeliveredAt *time.Time     `json:"delivered_at"`
}

func toResponse(o domain.WorkOrder) workOrderResponse {
	items := make([]itemResponse, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, itemResponse{
			Kind:           string(item.Kind),
			RefID:          item.RefID.String(),
			Name:           item.Name,
			UnitPriceCents: item.UnitPrice.Cents(),
			Quantity:       item.Quantity,
			TotalCents:     item.Total().Cents(),
		})
	}

	return workOrderResponse{
		ID:          o.ID.String(),
		Number:      o.Number,
		CustomerID:  o.CustomerID.String(),
		VehicleID:   o.VehicleID.String(),
		Description: o.Description,
		Status:      string(o.Status),
		Items:       items,
		TotalCents:  o.Total().Cents(),
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
		StartedAt:   o.StartedAt,
		CompletedAt: o.CompletedAt,
		DeliveredAt: o.DeliveredAt,
	}
}

type listResponse struct {
	Items  []workOrderResponse `json:"items"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	vehicleID, err := parseID(req.VehicleID, "identificador do veículo inválido")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	in, err := req.input()
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	order, err := h.svc.Create(r.Context(), vehicleID, req.Description, in)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	w.Header().Set("Location", "/admin/work-orders/"+order.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toResponse(order))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	orders, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	items := make([]workOrderResponse, 0, len(orders))
	for _, o := range orders {
		items = append(items, toResponse(o))
	}
	httpserver.WriteJSON(w, http.StatusOK, listResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, h.svc.Get)
}

func (h *Handler) replaceItems(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var req itemsRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	in, err := req.input()
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	order, err := h.svc.ReplaceItems(r.Context(), id, in)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toResponse(order))
}

// action serve as rotas que só mudam o status (sem corpo na requisição).
func (h *Handler) action(run func(context.Context, uuid.UUID) (domain.WorkOrder, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.respond(w, r, run)
	}
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, run func(context.Context, uuid.UUID) (domain.WorkOrder, error)) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	order, err := run(r.Context(), id)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toResponse(order))
}
