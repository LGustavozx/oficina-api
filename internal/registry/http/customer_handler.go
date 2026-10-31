// Package http do contexto registry: rotas dos cadastros.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/registry/domain"
)

type customerService interface {
	Create(ctx context.Context, data domain.CustomerData) (domain.Customer, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Customer, error)
	FindByDocument(ctx context.Context, rawDocument string) (domain.Customer, error)
	List(ctx context.Context, limit, offset int) ([]domain.Customer, error)
	Update(ctx context.Context, id uuid.UUID, data domain.CustomerData) (domain.Customer, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CustomerHandler struct {
	svc    customerService
	logger *slog.Logger
}

func NewCustomerHandler(svc customerService, logger *slog.Logger) *CustomerHandler {
	return &CustomerHandler{svc: svc, logger: logger}
}

// Register recebe o middleware de autenticação como parâmetro para o registry
// não depender do contexto identity.
func (h *CustomerHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("POST /admin/customers", protect(http.HandlerFunc(h.create)))
	mux.Handle("GET /admin/customers", protect(http.HandlerFunc(h.list)))
	mux.Handle("GET /admin/customers/{id}", protect(http.HandlerFunc(h.get)))
	mux.Handle("PUT /admin/customers/{id}", protect(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /admin/customers/{id}", protect(http.HandlerFunc(h.delete)))
}

type customerRequest struct {
	Name     string `json:"name"`
	Document string `json:"document"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

func (r customerRequest) data() domain.CustomerData {
	return domain.CustomerData{Name: r.Name, Document: r.Document, Email: r.Email, Phone: r.Phone}
}

type customerResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Document     string    `json:"document"`
	DocumentType string    `json:"document_type"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toCustomerResponse(c domain.Customer) customerResponse {
	return customerResponse{
		ID:           c.ID.String(),
		Name:         c.Name,
		Document:     c.Document.String(),
		DocumentType: string(c.Document.Type()),
		Email:        c.Email,
		Phone:        c.Phone,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

type customerListResponse struct {
	Items  []customerResponse `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

func (h *CustomerHandler) create(w http.ResponseWriter, r *http.Request) {
	var req customerRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	customer, err := h.svc.Create(r.Context(), req.data())
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	w.Header().Set("Location", "/admin/customers/"+customer.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toCustomerResponse(customer))
}

// list aceita ?document= para buscar pelo CPF/CNPJ; sem ele, lista paginada.
func (h *CustomerHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var customers []domain.Customer
	if raw := r.URL.Query().Get("document"); raw != "" {
		customers, err = h.listByDocument(r.Context(), raw)
	} else {
		customers, err = h.svc.List(r.Context(), limit, offset)
	}
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	items := make([]customerResponse, 0, len(customers))
	for _, c := range customers {
		items = append(items, toCustomerResponse(c))
	}
	httpserver.WriteJSON(w, http.StatusOK, customerListResponse{Items: items, Limit: limit, Offset: offset})
}

// listByDocument devolve lista vazia, e não 404, quando o documento não está cadastrado.
func (h *CustomerHandler) listByDocument(ctx context.Context, raw string) ([]domain.Customer, error) {
	customer, err := h.svc.FindByDocument(ctx, raw)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return []domain.Customer{customer}, nil
}

func (h *CustomerHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	customer, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toCustomerResponse(customer))
}

func (h *CustomerHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var req customerRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	customer, err := h.svc.Update(r.Context(), id, req.data())
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toCustomerResponse(customer))
}

func (h *CustomerHandler) delete(w http.ResponseWriter, r *http.Request) {
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
