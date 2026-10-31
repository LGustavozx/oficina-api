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

	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/workorder/domain"
)

const customerDocument = "52998224725"

type fakeCustomerService struct {
	calls    []string
	failWith error
}

func (f *fakeCustomerService) result(call string, number int64, document string) (domain.WorkOrder, error) {
	f.calls = append(f.calls, call)
	if f.failWith != nil {
		return domain.WorkOrder{}, f.failWith
	}
	if number != 7 || document != customerDocument {
		return domain.WorkOrder{}, apperr.NotFound(domain.CodeWorkOrderNotFound, "ordem de serviço não encontrada")
	}
	return (&fakeService{}).order(), nil
}

func (f *fakeCustomerService) GetForCustomer(_ context.Context, number int64, document string) (domain.WorkOrder, error) {
	return f.result("Get", number, document)
}

func (f *fakeCustomerService) ApproveForCustomer(_ context.Context, number int64, document string) (domain.WorkOrder, error) {
	return f.result("Approve", number, document)
}

func (f *fakeCustomerService) RejectForCustomer(_ context.Context, number int64, document string) (domain.WorkOrder, error) {
	return f.result("Reject", number, document)
}

func setupPublic(svc *fakeCustomerService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpserver.NewHandler(nopPinger{}, logger, NewPublicHandler(svc, logger).Register)
}

func callPublic(h http.Handler, method, path, document string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if document != "" {
		req.Header.Set(DocumentHeader, document)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPublicRoutesWork(t *testing.T) {
	routes := []struct{ method, path, useCase string }{
		{http.MethodGet, "/public/work-orders/7", "Get"},
		{http.MethodPost, "/public/work-orders/7/approve", "Approve"},
		{http.MethodPost, "/public/work-orders/7/reject", "Reject"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			svc := &fakeCustomerService{}

			rec := callPublic(setupPublic(svc), route.method, route.path, customerDocument)

			if rec.Code != 200 {
				t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
			}
			if len(svc.calls) != 1 || svc.calls[0] != route.useCase {
				t.Errorf("chamou %v, esperado %s", svc.calls, route.useCase)
			}
			assertBody(t, rec, `"number":7`, `"status":"RECEIVED"`, `"status_label":"recebida"`, `"total_cents":19180`,
				`"name":"Troca de óleo"`, `"unit_price_cents":3590`, `"started_at":null`)
		})
	}
}

// O cliente não precisa dos identificadores internos, e não deve recebê-los.
func TestPublicResponseHidesInternalIdentifiers(t *testing.T) {
	order := (&fakeService{}).order()

	rec := callPublic(setupPublic(&fakeCustomerService{}), http.MethodGet, "/public/work-orders/7", customerDocument)

	hidden := map[string]string{
		"id da OS":        order.ID.String(),
		"id do cliente":   order.CustomerID.String(),
		"id do veículo":   order.VehicleID.String(),
		"id do serviço":   serviceID.String(),
		"id da peça":      partID.String(),
		"campo customer":  `"customer_id"`,
		"campo vehicle":   `"vehicle_id"`,
		"campo ref_id":    `"ref_id"`,
		"o CPF informado": customerDocument,
	}
	for name, secret := range hidden {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("a resposta pública não deveria conter o %s (%s)", name, secret)
		}
	}
}

func TestPublicRequiresDocumentHeader(t *testing.T) {
	svc := &fakeCustomerService{}

	rec := callPublic(setupPublic(svc), http.MethodGet, "/public/work-orders/7", "")

	if rec.Code != 400 {
		t.Errorf("status = %d, esperado 400", rec.Code)
	}
	assertBody(t, rec, `"code":"DOCUMENT_REQUIRED"`, DocumentHeader)
	if len(svc.calls) != 0 {
		t.Error("sem documento, o caso de uso não deve ser chamado")
	}
}

func TestPublicRejectsInvalidNumbers(t *testing.T) {
	for _, number := range []string{"abc", "0", "-3", "1.5", "99999999999999999999"} {
		t.Run(number, func(t *testing.T) {
			svc := &fakeCustomerService{}

			rec := callPublic(setupPublic(svc), http.MethodGet, "/public/work-orders/"+number, customerDocument)

			if rec.Code != 400 {
				t.Errorf("status = %d, esperado 400", rec.Code)
			}
			assertBody(t, rec, `"code":"INVALID_ID"`)
			if len(svc.calls) != 0 {
				t.Error("número inválido não deve chegar ao caso de uso")
			}
		})
	}
}

func TestPublicErrorsComeFromTheUseCase(t *testing.T) {
	tests := []struct {
		name       string
		svc        *fakeCustomerService
		number     string
		document   string
		wantStatus int
		wantCode   string
	}{
		{"documento de outro cliente", &fakeCustomerService{}, "7", "11222333000181", 404, "WORK_ORDER_NOT_FOUND"},
		{"OS inexistente", &fakeCustomerService{}, "999", customerDocument, 404, "WORK_ORDER_NOT_FOUND"},
		{"ação fora de ordem", &fakeCustomerService{failWith: apperr.Conflict(domain.CodeInvalidTransition, "não aceita esta ação")}, "7", customerDocument, 409, "INVALID_STATUS_TRANSITION"},
		{"estoque insuficiente", &fakeCustomerService{failWith: apperr.Conflict("INSUFFICIENT_STOCK", "estoque insuficiente para a peça Filtro")}, "7", customerDocument, 409, "INSUFFICIENT_STOCK"},
		{"erro interno", &fakeCustomerService{failWith: errors.New("banco caiu")}, "7", customerDocument, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := callPublic(setupPublic(tt.svc), http.MethodPost, "/public/work-orders/"+tt.number+"/approve", tt.document)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

// A rota pública não passa pelo middleware de autenticação: só o documento a protege.
func TestPublicRoutesDoNotRequireAJWT(t *testing.T) {
	rec := callPublic(setupPublic(&fakeCustomerService{}), http.MethodGet, "/public/work-orders/7", customerDocument)

	if rec.Code == http.StatusUnauthorized {
		t.Error("as rotas públicas não devem exigir JWT")
	}
}
