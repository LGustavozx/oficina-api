// Package http do contexto inventory: rotas de peças e estoque.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/inventory/domain"
	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
)

type inventoryService interface {
	Create(ctx context.Context, data domain.PartData) (domain.Part, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Part, error)
	FindBySKU(ctx context.Context, sku string) (domain.Part, error)
	List(ctx context.Context, limit, offset int) ([]domain.Part, error)
	Update(ctx context.Context, id uuid.UUID, data domain.PartData) (domain.Part, error)
	Delete(ctx context.Context, id uuid.UUID) error
	RecordMovement(ctx context.Context, partID uuid.UUID, data domain.MovementData) (domain.Part, domain.StockMovement, error)
	ListMovements(ctx context.Context, partID uuid.UUID, limit, offset int) ([]domain.StockMovement, error)
}

type PartHandler struct {
	svc    inventoryService
	logger *slog.Logger
}

func NewPartHandler(svc inventoryService, logger *slog.Logger) *PartHandler {
	return &PartHandler{svc: svc, logger: logger}
}

func (h *PartHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("POST /admin/parts", protect(http.HandlerFunc(h.create)))
	mux.Handle("GET /admin/parts", protect(http.HandlerFunc(h.list)))
	mux.Handle("GET /admin/parts/{id}", protect(http.HandlerFunc(h.get)))
	mux.Handle("PUT /admin/parts/{id}", protect(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /admin/parts/{id}", protect(http.HandlerFunc(h.delete)))
	mux.Handle("POST /admin/parts/{id}/stock-movements", protect(http.HandlerFunc(h.recordMovement)))
	mux.Handle("GET /admin/parts/{id}/stock-movements", protect(http.HandlerFunc(h.listMovements)))
}

// O saldo não faz parte do corpo: ele só muda por movimentação de estoque.
type partRequest struct {
	SKU            string `json:"sku"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

func (r partRequest) data() domain.PartData {
	return domain.PartData{SKU: r.SKU, Name: r.Name, Description: r.Description, UnitPriceCents: r.UnitPriceCents}
}

type partResponse struct {
	ID             string    `json:"id"`
	SKU            string    `json:"sku"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	StockQuantity  int       `json:"stock_quantity"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toPartResponse(p domain.Part) partResponse {
	return partResponse{
		ID:             p.ID.String(),
		SKU:            p.SKU,
		Name:           p.Name,
		Description:    p.Description,
		UnitPriceCents: p.UnitPrice.Cents(),
		StockQuantity:  p.Stock,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

type partListResponse struct {
	Items  []partResponse `json:"items"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func (h *PartHandler) create(w http.ResponseWriter, r *http.Request) {
	var req partRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	part, err := h.svc.Create(r.Context(), req.data())
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	w.Header().Set("Location", "/admin/parts/"+part.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toPartResponse(part))
}

// list aceita ?sku= para buscar pelo código; sem ele, lista paginada.
func (h *PartHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var parts []domain.Part
	if sku := r.URL.Query().Get("sku"); sku != "" {
		parts, err = h.listBySKU(r.Context(), sku)
	} else {
		parts, err = h.svc.List(r.Context(), limit, offset)
	}
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	items := make([]partResponse, 0, len(parts))
	for _, p := range parts {
		items = append(items, toPartResponse(p))
	}
	httpserver.WriteJSON(w, http.StatusOK, partListResponse{Items: items, Limit: limit, Offset: offset})
}

// listBySKU devolve lista vazia, e não 404, quando o código não está cadastrado.
func (h *PartHandler) listBySKU(ctx context.Context, sku string) ([]domain.Part, error) {
	part, err := h.svc.FindBySKU(ctx, sku)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Kind == apperr.KindNotFound {
			return nil, nil
		}
		return nil, err
	}
	return []domain.Part{part}, nil
}

func (h *PartHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	part, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toPartResponse(part))
}

func (h *PartHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var req partRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	part, err := h.svc.Update(r.Context(), id, req.data())
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toPartResponse(part))
}

func (h *PartHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type movementRequest struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
	Reason   string `json:"reason"`
}

type movementResponse struct {
	ID        string    `json:"id"`
	PartID    string    `json:"part_id"`
	Type      string    `json:"type"`
	Quantity  int       `json:"quantity"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func toMovementResponse(m domain.StockMovement) movementResponse {
	return movementResponse{
		ID:        m.ID.String(),
		PartID:    m.PartID.String(),
		Type:      string(m.Type),
		Quantity:  m.Quantity,
		Reason:    m.Reason,
		CreatedAt: m.CreatedAt,
	}
}

type recordMovementResponse struct {
	Movement      movementResponse `json:"movement"`
	StockQuantity int              `json:"stock_quantity"`
}

type movementListResponse struct {
	Items  []movementResponse `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

func (h *PartHandler) recordMovement(w http.ResponseWriter, r *http.Request) {
	partID, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var req movementRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	part, movement, err := h.svc.RecordMovement(r.Context(), partID, domain.MovementData{
		Type: req.Type, Quantity: req.Quantity, Reason: req.Reason,
	})
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	httpserver.WriteJSON(w, http.StatusCreated, recordMovementResponse{
		Movement:      toMovementResponse(movement),
		StockQuantity: part.Stock,
	})
}

func (h *PartHandler) listMovements(w http.ResponseWriter, r *http.Request) {
	partID, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	movements, err := h.svc.ListMovements(r.Context(), partID, limit, offset)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	items := make([]movementResponse, 0, len(movements))
	for _, m := range movements {
		items = append(items, toMovementResponse(m))
	}
	httpserver.WriteJSON(w, http.StatusOK, movementListResponse{Items: items, Limit: limit, Offset: offset})
}
