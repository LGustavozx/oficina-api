package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

type vehicleService interface {
	Create(ctx context.Context, data domain.VehicleData) (domain.Vehicle, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Vehicle, error)
	FindByPlate(ctx context.Context, rawPlate string) (domain.Vehicle, error)
	List(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]domain.Vehicle, error)
	Update(ctx context.Context, id uuid.UUID, data domain.VehicleData) (domain.Vehicle, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type VehicleHandler struct {
	svc    vehicleService
	logger *slog.Logger
}

func NewVehicleHandler(svc vehicleService, logger *slog.Logger) *VehicleHandler {
	return &VehicleHandler{svc: svc, logger: logger}
}

func (h *VehicleHandler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("POST /admin/vehicles", protect(http.HandlerFunc(h.create)))
	mux.Handle("GET /admin/vehicles", protect(http.HandlerFunc(h.list)))
	mux.Handle("GET /admin/vehicles/{id}", protect(http.HandlerFunc(h.get)))
	mux.Handle("PUT /admin/vehicles/{id}", protect(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /admin/vehicles/{id}", protect(http.HandlerFunc(h.delete)))
}

type vehicleRequest struct {
	CustomerID string `json:"customer_id"`
	Plate      string `json:"plate"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	Year       int    `json:"year"`
}

func (r vehicleRequest) data() (domain.VehicleData, error) {
	customerID, err := parseCustomerID(r.CustomerID)
	if err != nil {
		return domain.VehicleData{}, err
	}
	return domain.VehicleData{CustomerID: customerID, Plate: r.Plate, Brand: r.Brand, Model: r.Model, Year: r.Year}, nil
}

func parseCustomerID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Validation(httpserver.CodeInvalidID, "identificador do cliente inválido")
	}
	return id, nil
}

type vehicleResponse struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Plate      string    `json:"plate"`
	Brand      string    `json:"brand"`
	Model      string    `json:"model"`
	Year       int       `json:"year"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toVehicleResponse(v domain.Vehicle) vehicleResponse {
	return vehicleResponse{
		ID:         v.ID.String(),
		CustomerID: v.CustomerID.String(),
		Plate:      v.Plate.String(),
		Brand:      v.Brand,
		Model:      v.Model,
		Year:       v.Year,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}
}

type vehicleListResponse struct {
	Items  []vehicleResponse `json:"items"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func (h *VehicleHandler) create(w http.ResponseWriter, r *http.Request) {
	data, err := h.readRequest(w, r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	vehicle, err := h.svc.Create(r.Context(), data)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	w.Header().Set("Location", "/admin/vehicles/"+vehicle.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toVehicleResponse(vehicle))
}

// list aceita ?plate= (busca exata) ou ?customer_id= (veículos do cliente).
func (h *VehicleHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	var vehicles []domain.Vehicle
	if plate := r.URL.Query().Get("plate"); plate != "" {
		vehicles, err = h.listByPlate(r.Context(), plate)
	} else {
		vehicles, err = h.listByCustomer(r, limit, offset)
	}
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	items := make([]vehicleResponse, 0, len(vehicles))
	for _, v := range vehicles {
		items = append(items, toVehicleResponse(v))
	}
	httpserver.WriteJSON(w, http.StatusOK, vehicleListResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *VehicleHandler) listByPlate(ctx context.Context, plate string) ([]domain.Vehicle, error) {
	vehicle, err := h.svc.FindByPlate(ctx, plate)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return []domain.Vehicle{vehicle}, nil
}

func (h *VehicleHandler) listByCustomer(r *http.Request, limit, offset int) ([]domain.Vehicle, error) {
	customerID := uuid.Nil
	if raw := r.URL.Query().Get("customer_id"); raw != "" {
		var err error
		if customerID, err = parseCustomerID(raw); err != nil {
			return nil, err
		}
	}
	return h.svc.List(r.Context(), customerID, limit, offset)
}

func (h *VehicleHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	vehicle, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toVehicleResponse(vehicle))
}

func (h *VehicleHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpserver.PathUUID(r, "id")
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	data, err := h.readRequest(w, r)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}

	vehicle, err := h.svc.Update(r.Context(), id, data)
	if err != nil {
		httpserver.WriteError(w, h.logger, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toVehicleResponse(vehicle))
}

func (h *VehicleHandler) delete(w http.ResponseWriter, r *http.Request) {
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

func (h *VehicleHandler) readRequest(w http.ResponseWriter, r *http.Request) (domain.VehicleData, error) {
	var req vehicleRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		return domain.VehicleData{}, err
	}
	return req.data()
}
