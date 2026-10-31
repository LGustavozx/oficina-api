// Package http do contexto catalog: rotas do catálogo de serviços.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/catalog/domain"
	"oficina-api/internal/platform/httpserver"
)

type serviceCatalog interface {
	Create(ctx context.Context, data domain.ServiceData) (domain.Service, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Service, error)
	List(ctx context.Context, limit, offset int) ([]domain.Service, error)
	Update(ctx context.Context, id uuid.UUID, data domain.ServiceData) (domain.Service, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ServiceHandler struct {
	catalog serviceCatalog
	logger  *slog.Logger
}

func NewServiceHandler(catalog serviceCatalog, logger *slog.Logger) *ServiceHandler {
	return &ServiceHandler{catalog: catalog, logger: logger}
}

func (h *ServiceHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("POST /admin/services", protect(http.HandlerFunc(h.create)))
	mux.Handle("GET /admin/services", protect(http.HandlerFunc(h.list)))
	mux.Handle("GET /admin/services/{id}", protect(http.HandlerFunc(h.get)))
	mux.Handle("PUT /admin/services/{id}", protect(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /admin/services/{id}", protect(http.HandlerFunc(h.delete)))
}

// O preço trafega em centavos (inteiro), o mesmo formato usado internamente.
type serviceRequest struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	PriceCents       int64  `json:"price_cents"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

func (r serviceRequest) data() domain.ServiceData {
	return domain.ServiceData{
		Name: r.Name, Description: r.Description, PriceCents: r.PriceCents, EstimatedMinutes: r.EstimatedMinutes,
	}
}

type serviceResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	PriceCents       int64     `json:"price_cents"`
	EstimatedMinutes int       `json:"estimated_minutes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func toServiceResponse(s domain.Service) serviceResponse {
	return serviceResponse{
		ID:               s.ID.String(),
		Name:             s.Name,
		Description:      s.Description,
		PriceCents:       s.Price.Cents(),
		EstimatedMinutes: s.EstimatedMinutes,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

type serviceListResponse struct {
	Items  []serviceResponse `json:"items"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func (h *ServiceHandler) create(w http.ResponseWriter, r *http.Request) {
	var req serviceRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	service, err := h.catalog.Create(r.Context(), req.data())
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	w.Header().Set("Location", "/admin/services/"+service.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toServiceResponse(service))
}

func (h *ServiceHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	services, err := h.catalog.List(r.Context(), limit, offset)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	items := make([]serviceResponse, 0, len(services))
	for _, s := range services {
		items = append(items, toServiceResponse(s))
	}
	httpserver.WriteJSON(w, http.StatusOK, serviceListResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *ServiceHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	service, err := h.catalog.Get(r.Context(), id)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toServiceResponse(service))
}

func (h *ServiceHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var req serviceRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	service, err := h.catalog.Update(r.Context(), id, req.data())
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toServiceResponse(service))
}

func (h *ServiceHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	if err := h.catalog.Delete(r.Context(), id); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
