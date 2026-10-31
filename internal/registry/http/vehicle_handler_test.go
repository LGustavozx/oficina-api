package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

var ownerID = uuid.MustParse("22222222-2222-2222-2222-222222222222")

type fakeVehicleService struct {
	listCalls []listCall
	failWith  error
}

type listCall struct {
	customerID    uuid.UUID
	limit, offset int
}

func (f *fakeVehicleService) vehicle() domain.Vehicle {
	v, _ := domain.NewVehicle(knownID, domain.VehicleData{
		CustomerID: ownerID, Plate: "ABC1D23", Brand: "Fiat", Model: "Uno", Year: 2010,
	}, testTime)
	return v
}

func (f *fakeVehicleService) Create(_ context.Context, data domain.VehicleData) (domain.Vehicle, error) {
	if f.failWith != nil {
		return domain.Vehicle{}, f.failWith
	}
	return domain.NewVehicle(knownID, data, testTime)
}

func (f *fakeVehicleService) Get(_ context.Context, id uuid.UUID) (domain.Vehicle, error) {
	if id != knownID {
		return domain.Vehicle{}, apperr.NotFound(domain.CodeVehicleNotFound, "veículo não encontrado")
	}
	return f.vehicle(), nil
}

func (f *fakeVehicleService) FindByPlate(_ context.Context, raw string) (domain.Vehicle, error) {
	plate, err := domain.NewPlate(raw)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if plate.String() != "ABC1D23" {
		return domain.Vehicle{}, apperr.NotFound(domain.CodeVehicleNotFound, "veículo não encontrado")
	}
	return f.vehicle(), nil
}

func (f *fakeVehicleService) List(_ context.Context, customerID uuid.UUID, limit, offset int) ([]domain.Vehicle, error) {
	f.listCalls = append(f.listCalls, listCall{customerID, limit, offset})
	if f.failWith != nil {
		return nil, f.failWith
	}
	return []domain.Vehicle{f.vehicle()}, nil
}

func (f *fakeVehicleService) Update(ctx context.Context, id uuid.UUID, data domain.VehicleData) (domain.Vehicle, error) {
	current, err := f.Get(ctx, id)
	if err != nil {
		return domain.Vehicle{}, err
	}
	return current.Update(data, testTime.Add(time.Hour))
}

func (f *fakeVehicleService) Delete(_ context.Context, id uuid.UUID) error {
	if id != knownID {
		return apperr.NotFound(domain.CodeVehicleNotFound, "veículo não encontrado")
	}
	return nil
}

func setupVehicles(svc *fakeVehicleService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewVehicleHandler(svc, logger)
	return httpserver.NewHandler(nopPinger{}, logger, func(mux *http.ServeMux) {
		handler.Register(mux, requireHeader)
	})
}

const vehicleBody = `{"customer_id":"22222222-2222-2222-2222-222222222222","plate":"abc-1d23","brand":"Fiat","model":"Uno","year":2010}`

func TestVehicleRoutesRequireAuthentication(t *testing.T) {
	h := setupVehicles(&fakeVehicleService{})
	path := "/admin/vehicles/" + knownID.String()

	routes := []struct{ method, path string }{
		{http.MethodPost, "/admin/vehicles"},
		{http.MethodGet, "/admin/vehicles"},
		{http.MethodGet, path},
		{http.MethodPut, path},
		{http.MethodDelete, path},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if rec := call(h, route.method, route.path, `{}`, false); rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, esperado 401", rec.Code)
			}
		})
	}
}

func TestVehicleCreate(t *testing.T) {
	rec := call(setupVehicles(&fakeVehicleService{}), http.MethodPost, "/admin/vehicles", vehicleBody, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/admin/vehicles/"+knownID.String() {
		t.Errorf("Location = %q", got)
	}
	assertBody(t, rec, `"plate":"ABC1D23"`, `"customer_id":"`+ownerID.String()+`"`, `"year":2010`)
}

func TestVehicleCreateErrors(t *testing.T) {
	tests := []struct {
		name       string
		svc        *fakeVehicleService
		body       string
		wantStatus int
		wantCode   string
	}{
		{"JSON inválido", &fakeVehicleService{}, `{`, 400, "INVALID_REQUEST"},
		{"customer_id inválido", &fakeVehicleService{}, `{"customer_id":"abc","plate":"ABC1234"}`, 400, "INVALID_ID"},
		{"customer_id ausente", &fakeVehicleService{}, `{"plate":"ABC1234"}`, 400, "INVALID_ID"},
		{"dados inválidos", &fakeVehicleService{}, `{"customer_id":"` + ownerID.String() + `","plate":"123"}`, 400, "INVALID_PLATE"},
		{"placa duplicada", &fakeVehicleService{failWith: apperr.Conflict(domain.CodePlateTaken, "placa já cadastrada")}, vehicleBody, 409, "PLATE_ALREADY_REGISTERED"},
		{"cliente inexistente", &fakeVehicleService{failWith: apperr.Validation(domain.CodeCustomerNotFound, "o cliente informado não existe")}, vehicleBody, 400, "CUSTOMER_NOT_FOUND"},
		{"erro interno", &fakeVehicleService{failWith: errors.New("banco caiu")}, vehicleBody, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setupVehicles(tt.svc), http.MethodPost, "/admin/vehicles", tt.body, true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

func TestVehicleList(t *testing.T) {
	t.Run("sem filtro e paginação padrão", func(t *testing.T) {
		svc := &fakeVehicleService{}
		rec := call(setupVehicles(svc), http.MethodGet, "/admin/vehicles", "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"items":[{`, `"limit":50`)
		if len(svc.listCalls) != 1 || svc.listCalls[0] != (listCall{uuid.Nil, 50, 0}) {
			t.Errorf("chamadas = %+v", svc.listCalls)
		}
	})

	t.Run("filtro por cliente e paginação", func(t *testing.T) {
		svc := &fakeVehicleService{}
		call(setupVehicles(svc), http.MethodGet, "/admin/vehicles?customer_id="+ownerID.String()+"&limit=5&offset=10", "", true)

		if len(svc.listCalls) != 1 || svc.listCalls[0] != (listCall{ownerID, 5, 10}) {
			t.Errorf("chamadas = %+v", svc.listCalls)
		}
	})

	errorCases := []struct {
		name, query string
		svc         *fakeVehicleService
		wantStatus  int
		wantCode    string
	}{
		{"customer_id inválido", "?customer_id=abc", &fakeVehicleService{}, 400, "INVALID_ID"},
		{"paginação inválida", "?limit=0", &fakeVehicleService{}, 400, "INVALID_PAGINATION"},
		{"erro do serviço", "", &fakeVehicleService{failWith: errors.New("x")}, 500, "INTERNAL_ERROR"},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setupVehicles(tt.svc), http.MethodGet, "/admin/vehicles"+tt.query, "", true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

func TestVehicleListByPlate(t *testing.T) {
	h := setupVehicles(&fakeVehicleService{})

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantBody   string
	}{
		{"encontrada", "?plate=ABC1D23", 200, `"brand":"Fiat"`},
		{"com hífen e minúsculas", "?plate=abc1d23", 200, `"brand":"Fiat"`},
		{"não cadastrada devolve lista vazia", "?plate=ZZZ9999", 200, `"items":[]`},
		{"placa inválida", "?plate=123", 400, `"code":"INVALID_PLATE"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(h, http.MethodGet, "/admin/vehicles"+tt.query, "", true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, tt.wantBody)
		})
	}
}

func TestVehicleGet(t *testing.T) {
	h := setupVehicles(&fakeVehicleService{})

	if rec := call(h, http.MethodGet, "/admin/vehicles/"+knownID.String(), "", true); rec.Code != 200 {
		t.Errorf("existente: status = %d", rec.Code)
	}

	rec := call(h, http.MethodGet, "/admin/vehicles/"+uuid.NewString(), "", true)
	if rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	assertBody(t, rec, `"code":"VEHICLE_NOT_FOUND"`)

	if rec := call(h, http.MethodGet, "/admin/vehicles/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestVehicleUpdate(t *testing.T) {
	h := setupVehicles(&fakeVehicleService{})
	path := "/admin/vehicles/" + knownID.String()

	rec := call(h, http.MethodPut, path, `{"customer_id":"`+ownerID.String()+`","plate":"ABC1D23","brand":"VW","model":"Gol","year":2015}`, true)
	if rec.Code != 200 {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	assertBody(t, rec, `"model":"Gol"`, `"updated_at":"2026-03-01T11:00:00Z"`)

	if rec := call(h, http.MethodPut, path, `{`, true); rec.Code != 400 {
		t.Errorf("JSON inválido: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, path, `{"customer_id":"abc"}`, true); rec.Code != 400 {
		t.Errorf("customer_id inválido: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/vehicles/"+uuid.NewString(), vehicleBody, true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/vehicles/abc", vehicleBody, true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestVehicleDelete(t *testing.T) {
	h := setupVehicles(&fakeVehicleService{})

	rec := call(h, http.MethodDelete, "/admin/vehicles/"+knownID.String(), "", true)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("status = %d, corpo = %q", rec.Code, rec.Body)
	}
	if rec := call(h, http.MethodDelete, "/admin/vehicles/"+uuid.NewString(), "", true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, "/admin/vehicles/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}
