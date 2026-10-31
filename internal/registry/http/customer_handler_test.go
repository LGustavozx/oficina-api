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
	"oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/apperr"
)

var (
	knownID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testTime = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
)

type fakeService struct {
	listCalls []struct{ limit, offset int }
	failWith  error
}

func (f *fakeService) customer(t time.Time) domain.Customer {
	c, _ := domain.NewCustomer(knownID, domain.CustomerData{
		Name: "Maria Souza", Document: "529.982.247-25", Email: "maria@email.com", Phone: "11987654321",
	}, t)
	return c
}

func (f *fakeService) Create(_ context.Context, data domain.CustomerData) (domain.Customer, error) {
	if f.failWith != nil {
		return domain.Customer{}, f.failWith
	}
	return domain.NewCustomer(knownID, data, testTime)
}

func (f *fakeService) Get(_ context.Context, id uuid.UUID) (domain.Customer, error) {
	if id != knownID {
		return domain.Customer{}, apperr.NotFound(domain.CodeCustomerNotFound, "cliente não encontrado")
	}
	return f.customer(testTime), nil
}

func (f *fakeService) FindByDocument(_ context.Context, raw string) (domain.Customer, error) {
	doc, err := domain.NewDocument(raw)
	if err != nil {
		return domain.Customer{}, err
	}
	if doc.String() != "52998224725" {
		return domain.Customer{}, apperr.NotFound(domain.CodeCustomerNotFound, "cliente não encontrado")
	}
	return f.customer(testTime), nil
}

func (f *fakeService) List(_ context.Context, limit, offset int) ([]domain.Customer, error) {
	f.listCalls = append(f.listCalls, struct{ limit, offset int }{limit, offset})
	if f.failWith != nil {
		return nil, f.failWith
	}
	return []domain.Customer{f.customer(testTime)}, nil
}

func (f *fakeService) Update(ctx context.Context, id uuid.UUID, data domain.CustomerData) (domain.Customer, error) {
	current, err := f.Get(ctx, id)
	if err != nil {
		return domain.Customer{}, err
	}
	return current.Update(data, testTime.Add(time.Hour))
}

func (f *fakeService) Delete(_ context.Context, id uuid.UUID) error {
	if id != knownID {
		return apperr.NotFound(domain.CodeCustomerNotFound, "cliente não encontrado")
	}
	return nil
}

type nopPinger struct{}

func (nopPinger) PingContext(context.Context) error { return nil }

// requireHeader simula o middleware de autenticação: exige o cabeçalho Authorization.
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
	handler := NewCustomerHandler(svc, logger)
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

func TestAllRoutesRequireAuthentication(t *testing.T) {
	h := setup(&fakeService{})
	path := "/admin/customers/" + knownID.String()

	routes := []struct{ method, path string }{
		{http.MethodPost, "/admin/customers"},
		{http.MethodGet, "/admin/customers"},
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

func TestCreate(t *testing.T) {
	h := setup(&fakeService{})
	body := `{"name":"Maria Souza","document":"529.982.247-25","email":"maria@email.com","phone":"(11) 98765-4321"}`

	rec := call(h, http.MethodPost, "/admin/customers", body, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/admin/customers/"+knownID.String() {
		t.Errorf("Location = %q", got)
	}
	assertBody(t, rec, `"document":"52998224725"`, `"document_type":"CPF"`, `"phone":"11987654321"`, `"created_at":"2026-03-01T10:00:00Z"`)
}

func TestCreateErrors(t *testing.T) {
	tests := []struct {
		name       string
		svc        *fakeService
		body       string
		wantStatus int
		wantCode   string
	}{
		{"JSON inválido", &fakeService{}, `{`, 400, "INVALID_REQUEST"},
		{"campo desconhecido", &fakeService{}, `{"name":"x","role":"admin"}`, 400, "INVALID_REQUEST"},
		{"dados inválidos", &fakeService{}, `{"name":"x","document":"123"}`, 400, "INVALID_NAME"},
		{"documento duplicado", &fakeService{failWith: apperr.Conflict(domain.CodeDocumentTaken, "documento já cadastrado")}, `{"name":"Maria","document":"529.982.247-25"}`, 409, "DOCUMENT_ALREADY_REGISTERED"},
		{"erro interno", &fakeService{failWith: errors.New("banco caiu")}, `{"name":"Maria","document":"529.982.247-25"}`, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(tt.svc), http.MethodPost, "/admin/customers", tt.body, true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

func TestList(t *testing.T) {
	t.Run("paginação padrão", func(t *testing.T) {
		svc := &fakeService{}
		rec := call(setup(svc), http.MethodGet, "/admin/customers", "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"items":[{`, `"limit":50`, `"offset":0`)
		if len(svc.listCalls) != 1 || svc.listCalls[0].limit != 50 {
			t.Errorf("chamadas = %+v", svc.listCalls)
		}
	})

	t.Run("paginação informada", func(t *testing.T) {
		svc := &fakeService{}
		call(setup(svc), http.MethodGet, "/admin/customers?limit=5&offset=10", "", true)

		if len(svc.listCalls) != 1 || svc.listCalls[0].limit != 5 || svc.listCalls[0].offset != 10 {
			t.Errorf("chamadas = %+v", svc.listCalls)
		}
	})

	t.Run("paginação inválida", func(t *testing.T) {
		rec := call(setup(&fakeService{}), http.MethodGet, "/admin/customers?limit=0", "", true)

		if rec.Code != 400 {
			t.Errorf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"code":"INVALID_PAGINATION"`)
	})

	t.Run("erro do serviço", func(t *testing.T) {
		rec := call(setup(&fakeService{failWith: errors.New("x")}), http.MethodGet, "/admin/customers", "", true)

		if rec.Code != 500 {
			t.Errorf("status = %d", rec.Code)
		}
	})
}

func TestListByDocument(t *testing.T) {
	h := setup(&fakeService{})

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantBody   string
	}{
		{"encontrado", "?document=529.982.247-25", 200, `"name":"Maria Souza"`},
		{"sem formatação", "?document=52998224725", 200, `"name":"Maria Souza"`},
		{"não cadastrado devolve lista vazia", "?document=11.222.333/0001-81", 200, `"items":[]`},
		{"documento inválido", "?document=123", 400, `"code":"INVALID_DOCUMENT"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(h, http.MethodGet, "/admin/customers"+tt.query, "", true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, tt.wantBody)
		})
	}
}

func TestGet(t *testing.T) {
	h := setup(&fakeService{})

	rec := call(h, http.MethodGet, "/admin/customers/"+knownID.String(), "", true)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	assertBody(t, rec, `"id":"`+knownID.String()+`"`)

	rec = call(h, http.MethodGet, "/admin/customers/"+uuid.NewString(), "", true)
	if rec.Code != 404 {
		t.Errorf("status = %d, esperado 404", rec.Code)
	}
	assertBody(t, rec, `"code":"CUSTOMER_NOT_FOUND"`)

	rec = call(h, http.MethodGet, "/admin/customers/abc", "", true)
	if rec.Code != 400 {
		t.Errorf("status = %d, esperado 400", rec.Code)
	}
	assertBody(t, rec, `"code":"INVALID_ID"`)
}

func TestUpdate(t *testing.T) {
	h := setup(&fakeService{})
	path := "/admin/customers/" + knownID.String()

	rec := call(h, http.MethodPut, path, `{"name":"Maria Silva","document":"529.982.247-25"}`, true)
	if rec.Code != 200 {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	assertBody(t, rec, `"name":"Maria Silva"`, `"updated_at":"2026-03-01T11:00:00Z"`)

	if rec := call(h, http.MethodPut, path, `{"name":"x"}`, true); rec.Code != 400 {
		t.Errorf("dados inválidos: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, path, `{`, true); rec.Code != 400 {
		t.Errorf("JSON inválido: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/customers/"+uuid.NewString(), `{"name":"Maria","document":"529.982.247-25"}`, true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/customers/abc", `{}`, true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestDelete(t *testing.T) {
	h := setup(&fakeService{})

	rec := call(h, http.MethodDelete, "/admin/customers/"+knownID.String(), "", true)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("status = %d, corpo = %q", rec.Code, rec.Body)
	}

	if rec := call(h, http.MethodDelete, "/admin/customers/"+uuid.NewString(), "", true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, "/admin/customers/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}
