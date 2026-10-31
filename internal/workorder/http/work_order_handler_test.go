package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/workorder/application"
	"oficina-api/internal/workorder/domain"
)

var (
	knownID   = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	vehicleID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	serviceID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	partID    = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	testTime  = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
)

type fakeService struct {
	failWith  error
	lastInput application.ItemsInput
	lastDesc  string
	lastCalls []string
	listArgs  [3]any
}

func (f *fakeService) order() domain.WorkOrder {
	items := []domain.Item{
		{Kind: domain.ItemService, RefID: serviceID, Name: "Troca de óleo", UnitPrice: money.FromCents(12000), Quantity: 1},
		{Kind: domain.ItemPart, RefID: partID, Name: "Filtro", UnitPrice: money.FromCents(3590), Quantity: 2},
	}
	o, _ := domain.NewWorkOrder(knownID, uuid.New(), vehicleID, "Barulho no motor", items, testTime)
	o.Number = 7
	return o
}

func (f *fakeService) result(called string, id uuid.UUID) (domain.WorkOrder, error) {
	f.lastCalls = append(f.lastCalls, called)
	if f.failWith != nil {
		return domain.WorkOrder{}, f.failWith
	}
	if id != knownID {
		return domain.WorkOrder{}, apperr.NotFound(domain.CodeWorkOrderNotFound, "ordem de serviço não encontrada")
	}
	return f.order(), nil
}

func (f *fakeService) Create(_ context.Context, v uuid.UUID, description string, in application.ItemsInput) (domain.WorkOrder, error) {
	f.lastInput, f.lastDesc = in, description
	return f.result("Create", knownID)
}

func (f *fakeService) Get(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("Get", id)
}

func (f *fakeService) List(_ context.Context, status string, limit, offset int) ([]domain.WorkOrder, error) {
	f.listArgs = [3]any{status, limit, offset}
	if f.failWith != nil {
		return nil, f.failWith
	}
	return []domain.WorkOrder{f.order()}, nil
}

func (f *fakeService) ReplaceItems(_ context.Context, id uuid.UUID, in application.ItemsInput) (domain.WorkOrder, error) {
	f.lastInput = in
	return f.result("ReplaceItems", id)
}

func (f *fakeService) StartDiagnosis(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("StartDiagnosis", id)
}
func (f *fakeService) SendQuote(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("SendQuote", id)
}
func (f *fakeService) Approve(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("Approve", id)
}
func (f *fakeService) Reject(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("Reject", id)
}
func (f *fakeService) Complete(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("Complete", id)
}
func (f *fakeService) Deliver(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return f.result("Deliver", id)
}

type nopPinger struct{}

func (nopPinger) PingContext(context.Context) error { return nil }

func requireHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "não autenticado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setup(svc *fakeService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(svc, logger)
	return httpserver.NewHandler(nopPinger{}, logger, func(mux *http.ServeMux) {
		handler.Register(mux, requireHeader)
	})
}

func call(h http.Handler, method, path, body string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authenticated {
		req.Header.Set("Authorization", "Bearer x")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertBody(t *testing.T, rec *httptest.ResponseRecorder, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("corpo = %s, esperado conter %s", rec.Body, want)
		}
	}
}

var orderPath = "/admin/work-orders/" + knownID.String()

func TestAllRoutesRequireAuthentication(t *testing.T) {
	h := setup(&fakeService{})

	routes := []struct{ method, path string }{
		{http.MethodPost, "/admin/work-orders"},
		{http.MethodGet, "/admin/work-orders"},
		{http.MethodGet, orderPath},
		{http.MethodPut, orderPath + "/items"},
		{http.MethodPost, orderPath + "/start-diagnosis"},
		{http.MethodPost, orderPath + "/send-quote"},
		{http.MethodPost, orderPath + "/approve"},
		{http.MethodPost, orderPath + "/reject"},
		{http.MethodPost, orderPath + "/complete"},
		{http.MethodPost, orderPath + "/deliver"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if rec := call(h, route.method, route.path, `{}`, false); rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, esperado 401", rec.Code)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	svc := &fakeService{}
	body := `{"vehicle_id":"` + vehicleID.String() + `","description":"Barulho no motor",
		"services":["` + serviceID.String() + `"],"parts":[{"part_id":"` + partID.String() + `","quantity":2}]}`

	rec := call(setup(svc), http.MethodPost, "/admin/work-orders", body, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != orderPath {
		t.Errorf("Location = %q", got)
	}
	assertBody(t, rec, `"number":7`, `"status":"RECEIVED"`, `"total_cents":19180`,
		`"kind":"SERVICE"`, `"kind":"PART"`, `"unit_price_cents":3590`, `"total_cents":7180`,
		`"started_at":null`, `"completed_at":null`, `"delivered_at":null`)

	if svc.lastDesc != "Barulho no motor" || len(svc.lastInput.ServiceIDs) != 1 || svc.lastInput.ServiceIDs[0] != serviceID ||
		len(svc.lastInput.Parts) != 1 || svc.lastInput.Parts[0] != (application.PartInput{PartID: partID, Quantity: 2}) {
		t.Errorf("o serviço recebeu dados errados: %q %+v", svc.lastDesc, svc.lastInput)
	}
}

func TestCreateWithoutItems(t *testing.T) {
	svc := &fakeService{}

	rec := call(setup(svc), http.MethodPost, "/admin/work-orders", `{"vehicle_id":"`+vehicleID.String()+`","description":"Revisão"}`, true)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d (%s)", rec.Code, rec.Body)
	}
}

func TestCreateErrors(t *testing.T) {
	validVehicle := `"vehicle_id":"` + vehicleID.String() + `"`

	tests := []struct {
		name       string
		svc        *fakeService
		body       string
		wantStatus int
		wantCode   string
	}{
		{"JSON inválido", &fakeService{}, `{`, 400, "INVALID_REQUEST"},
		{"campo desconhecido", &fakeService{}, `{` + validVehicle + `,"status":"DELIVERED"}`, 400, "INVALID_REQUEST"},
		{"veículo inválido", &fakeService{}, `{"vehicle_id":"abc","description":"x"}`, 400, "INVALID_ID"},
		{"veículo ausente", &fakeService{}, `{"description":"x"}`, 400, "INVALID_ID"},
		{"serviço inválido", &fakeService{}, `{` + validVehicle + `,"description":"x","services":["abc"]}`, 400, "INVALID_ID"},
		{"peça inválida", &fakeService{}, `{` + validVehicle + `,"description":"x","parts":[{"part_id":"abc","quantity":1}]}`, 400, "INVALID_ID"},
		{"regra de negócio", &fakeService{failWith: apperr.Validation(domain.CodeInvalidDescription, "descreva o problema")}, `{` + validVehicle + `,"description":""}`, 400, "INVALID_DESCRIPTION"},
		{"erro interno", &fakeService{failWith: errors.New("banco caiu")}, `{` + validVehicle + `,"description":"x"}`, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(tt.svc), http.MethodPost, "/admin/work-orders", tt.body, true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

func TestList(t *testing.T) {
	t.Run("padrão", func(t *testing.T) {
		svc := &fakeService{}
		rec := call(setup(svc), http.MethodGet, "/admin/work-orders", "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"items":[{`, `"limit":50`, `"offset":0`)
		if svc.listArgs != [3]any{"", 50, 0} {
			t.Errorf("argumentos = %v", svc.listArgs)
		}
	})

	t.Run("com filtro e paginação", func(t *testing.T) {
		svc := &fakeService{}
		call(setup(svc), http.MethodGet, "/admin/work-orders?status=IN_EXECUTION&limit=5&offset=10", "", true)

		if svc.listArgs != [3]any{"IN_EXECUTION", 5, 10} {
			t.Errorf("argumentos = %v", svc.listArgs)
		}
	})

	t.Run("status inválido vindo do serviço", func(t *testing.T) {
		rec := call(setup(&fakeService{failWith: apperr.Validation(domain.CodeInvalidStatus, "status inválido")}), http.MethodGet, "/admin/work-orders?status=X", "", true)

		if rec.Code != 400 {
			t.Errorf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"code":"INVALID_STATUS"`)
	})

	t.Run("paginação inválida", func(t *testing.T) {
		rec := call(setup(&fakeService{}), http.MethodGet, "/admin/work-orders?limit=0", "", true)

		if rec.Code != 400 {
			t.Errorf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"code":"INVALID_PAGINATION"`)
	})
}

func TestGet(t *testing.T) {
	h := setup(&fakeService{})

	rec := call(h, http.MethodGet, orderPath, "", true)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	assertBody(t, rec, `"id":"`+knownID.String()+`"`)

	rec = call(h, http.MethodGet, "/admin/work-orders/"+uuid.NewString(), "", true)
	if rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	assertBody(t, rec, `"code":"WORK_ORDER_NOT_FOUND"`)

	if rec := call(h, http.MethodGet, "/admin/work-orders/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestReplaceItems(t *testing.T) {
	svc := &fakeService{}
	h := setup(svc)

	rec := call(h, http.MethodPut, orderPath+"/items", `{"services":["`+serviceID.String()+`"]}`, true)
	if rec.Code != 200 {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if len(svc.lastInput.ServiceIDs) != 1 || len(svc.lastInput.Parts) != 0 {
		t.Errorf("entrada = %+v", svc.lastInput)
	}

	errorCases := []struct {
		name, path, body string
		wantStatus       int
	}{
		{"id inválido", "/admin/work-orders/abc/items", `{}`, 400},
		{"JSON inválido", orderPath + "/items", `{`, 400},
		{"serviço inválido", orderPath + "/items", `{"services":["abc"]}`, 400},
		{"inexistente", "/admin/work-orders/" + uuid.NewString() + "/items", `{}`, 404},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			if rec := call(h, http.MethodPut, tt.path, tt.body, true); rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestActionsCallTheRightUseCase(t *testing.T) {
	actions := map[string]string{
		"start-diagnosis": "StartDiagnosis",
		"send-quote":      "SendQuote",
		"approve":         "Approve",
		"reject":          "Reject",
		"complete":        "Complete",
		"deliver":         "Deliver",
	}

	for route, useCase := range actions {
		t.Run(route, func(t *testing.T) {
			svc := &fakeService{}

			rec := call(setup(svc), http.MethodPost, orderPath+"/"+route, "", true)

			if rec.Code != 200 {
				t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
			}
			if len(svc.lastCalls) != 1 || svc.lastCalls[0] != useCase {
				t.Errorf("chamou %v, esperado %s", svc.lastCalls, useCase)
			}
			assertBody(t, rec, `"id":"`+knownID.String()+`"`)
		})
	}
}

func TestActionErrors(t *testing.T) {
	tests := []struct {
		name       string
		svc        *fakeService
		path       string
		wantStatus int
		wantCode   string
	}{
		{"id inválido", &fakeService{}, "/admin/work-orders/abc/approve", 400, "INVALID_ID"},
		{"inexistente", &fakeService{}, "/admin/work-orders/" + uuid.NewString() + "/approve", 404, "WORK_ORDER_NOT_FOUND"},
		{"status errado", &fakeService{failWith: apperr.Conflict(domain.CodeInvalidTransition, "não aceita esta ação")}, orderPath + "/approve", 409, "INVALID_STATUS_TRANSITION"},
		{"estoque insuficiente", &fakeService{failWith: apperr.Conflict("INSUFFICIENT_STOCK", "estoque insuficiente para a peça Filtro")}, orderPath + "/approve", 409, "INSUFFICIENT_STOCK"},
		{"orçamento sem itens", &fakeService{failWith: apperr.Validation(domain.CodeQuoteWithoutItems, "inclua itens")}, orderPath + "/send-quote", 400, "QUOTE_WITHOUT_ITEMS"},
		{"erro interno", &fakeService{failWith: errors.New("banco caiu")}, orderPath + "/complete", 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(tt.svc), http.MethodPost, tt.path, "", true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}
