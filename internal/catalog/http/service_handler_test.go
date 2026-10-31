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

	"oficina-api/internal/catalog/domain"
	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
)

var (
	knownID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testTime = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
)

type fakeCatalog struct {
	listCalls [][2]int
	failWith  error
}

func (f *fakeCatalog) service() domain.Service {
	s, _ := domain.NewService(knownID, domain.ServiceData{Name: "Troca de óleo", PriceCents: 12000, EstimatedMinutes: 45}, testTime)
	return s
}

func (f *fakeCatalog) Create(_ context.Context, data domain.ServiceData) (domain.Service, error) {
	if f.failWith != nil {
		return domain.Service{}, f.failWith
	}
	return domain.NewService(knownID, data, testTime)
}

func (f *fakeCatalog) Get(_ context.Context, id uuid.UUID) (domain.Service, error) {
	if id != knownID {
		return domain.Service{}, apperr.NotFound(domain.CodeServiceNotFound, "serviço não encontrado")
	}
	return f.service(), nil
}

func (f *fakeCatalog) List(_ context.Context, limit, offset int) ([]domain.Service, error) {
	f.listCalls = append(f.listCalls, [2]int{limit, offset})
	if f.failWith != nil {
		return nil, f.failWith
	}
	return []domain.Service{f.service()}, nil
}

func (f *fakeCatalog) Update(ctx context.Context, id uuid.UUID, data domain.ServiceData) (domain.Service, error) {
	current, err := f.Get(ctx, id)
	if err != nil {
		return domain.Service{}, err
	}
	return current.Update(data, testTime.Add(time.Hour))
}

func (f *fakeCatalog) Delete(_ context.Context, id uuid.UUID) error {
	if id != knownID {
		return apperr.NotFound(domain.CodeServiceNotFound, "serviço não encontrado")
	}
	return nil
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

func setup(catalog *fakeCatalog) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewServiceHandler(catalog, logger)
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

const serviceBody = `{"name":"Troca de óleo","description":"Óleo e filtro","price_cents":12000,"estimated_minutes":45}`

func TestAllRoutesRequireAuthentication(t *testing.T) {
	h := setup(&fakeCatalog{})
	path := "/admin/services/" + knownID.String()

	routes := []struct{ method, path string }{
		{http.MethodPost, "/admin/services"},
		{http.MethodGet, "/admin/services"},
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
	rec := call(setup(&fakeCatalog{}), http.MethodPost, "/admin/services", serviceBody, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/admin/services/"+knownID.String() {
		t.Errorf("Location = %q", got)
	}
	assertBody(t, rec, `"price_cents":12000`, `"estimated_minutes":45`, `"name":"Troca de óleo"`)
}

func TestCreateErrors(t *testing.T) {
	tests := []struct {
		name       string
		catalog    *fakeCatalog
		body       string
		wantStatus int
		wantCode   string
	}{
		{"JSON inválido", &fakeCatalog{}, `{`, 400, "INVALID_REQUEST"},
		{"campo desconhecido", &fakeCatalog{}, `{"name":"x","cost":1}`, 400, "INVALID_REQUEST"},
		{"preço como texto", &fakeCatalog{}, `{"name":"Troca","price_cents":"12,00"}`, 400, "INVALID_REQUEST"},
		{"preço omitido", &fakeCatalog{}, `{"name":"Troca de óleo","estimated_minutes":45}`, 400, "INVALID_PRICE"},
		{"nome duplicado", &fakeCatalog{failWith: apperr.Conflict(domain.CodeServiceNameTaken, "nome já cadastrado")}, serviceBody, 409, "SERVICE_NAME_ALREADY_REGISTERED"},
		{"erro interno", &fakeCatalog{failWith: errors.New("banco caiu")}, serviceBody, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(tt.catalog), http.MethodPost, "/admin/services", tt.body, true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

func TestList(t *testing.T) {
	t.Run("paginação padrão", func(t *testing.T) {
		catalog := &fakeCatalog{}
		rec := call(setup(catalog), http.MethodGet, "/admin/services", "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"items":[{`, `"limit":50`, `"offset":0`)
		if len(catalog.listCalls) != 1 || catalog.listCalls[0] != [2]int{50, 0} {
			t.Errorf("chamadas = %v", catalog.listCalls)
		}
	})

	t.Run("paginação informada", func(t *testing.T) {
		catalog := &fakeCatalog{}
		call(setup(catalog), http.MethodGet, "/admin/services?limit=5&offset=10", "", true)

		if len(catalog.listCalls) != 1 || catalog.listCalls[0] != [2]int{5, 10} {
			t.Errorf("chamadas = %v", catalog.listCalls)
		}
	})

	t.Run("paginação inválida", func(t *testing.T) {
		rec := call(setup(&fakeCatalog{}), http.MethodGet, "/admin/services?limit=0", "", true)

		if rec.Code != 400 {
			t.Errorf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"code":"INVALID_PAGINATION"`)
	})

	t.Run("erro do serviço", func(t *testing.T) {
		rec := call(setup(&fakeCatalog{failWith: errors.New("x")}), http.MethodGet, "/admin/services", "", true)

		if rec.Code != 500 {
			t.Errorf("status = %d", rec.Code)
		}
	})
}

func TestGet(t *testing.T) {
	h := setup(&fakeCatalog{})

	if rec := call(h, http.MethodGet, "/admin/services/"+knownID.String(), "", true); rec.Code != 200 {
		t.Errorf("existente: status = %d", rec.Code)
	}

	rec := call(h, http.MethodGet, "/admin/services/"+uuid.NewString(), "", true)
	if rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	assertBody(t, rec, `"code":"SERVICE_NOT_FOUND"`)

	if rec := call(h, http.MethodGet, "/admin/services/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestUpdate(t *testing.T) {
	h := setup(&fakeCatalog{})
	path := "/admin/services/" + knownID.String()

	rec := call(h, http.MethodPut, path, `{"name":"Alinhamento","price_cents":8000,"estimated_minutes":30}`, true)
	if rec.Code != 200 {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	assertBody(t, rec, `"name":"Alinhamento"`, `"updated_at":"2026-03-01T11:00:00Z"`)

	if rec := call(h, http.MethodPut, path, `{"name":"x"}`, true); rec.Code != 400 {
		t.Errorf("dados inválidos: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, path, `{`, true); rec.Code != 400 {
		t.Errorf("JSON inválido: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/services/"+uuid.NewString(), serviceBody, true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/services/abc", serviceBody, true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestDelete(t *testing.T) {
	h := setup(&fakeCatalog{})

	rec := call(h, http.MethodDelete, "/admin/services/"+knownID.String(), "", true)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("status = %d, corpo = %q", rec.Code, rec.Body)
	}
	if rec := call(h, http.MethodDelete, "/admin/services/"+uuid.NewString(), "", true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, "/admin/services/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}
