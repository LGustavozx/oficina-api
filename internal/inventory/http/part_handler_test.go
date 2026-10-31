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

	"oficina-api/internal/inventory/domain"
	"oficina-api/internal/platform/httpserver"
	"oficina-api/internal/shared/apperr"
)

var (
	knownID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testTime = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
)

type fakeService struct {
	listCalls [][2]int
	failWith  error
}

func (f *fakeService) part() domain.Part {
	p, _ := domain.NewPart(knownID, domain.PartData{SKU: "FLT-01", Name: "Filtro", UnitPriceCents: 3590}, testTime)
	p.Stock = 8
	return p
}

func missing() error { return apperr.NotFound(domain.CodePartNotFound, "peça não encontrada") }

func (f *fakeService) Create(_ context.Context, data domain.PartData) (domain.Part, error) {
	if f.failWith != nil {
		return domain.Part{}, f.failWith
	}
	return domain.NewPart(knownID, data, testTime)
}

func (f *fakeService) Get(_ context.Context, id uuid.UUID) (domain.Part, error) {
	if id != knownID {
		return domain.Part{}, missing()
	}
	return f.part(), nil
}

func (f *fakeService) FindBySKU(_ context.Context, sku string) (domain.Part, error) {
	if f.failWith != nil {
		return domain.Part{}, f.failWith
	}
	if strings.ToUpper(sku) != "FLT-01" {
		return domain.Part{}, missing()
	}
	return f.part(), nil
}

func (f *fakeService) List(_ context.Context, limit, offset int) ([]domain.Part, error) {
	f.listCalls = append(f.listCalls, [2]int{limit, offset})
	if f.failWith != nil {
		return nil, f.failWith
	}
	return []domain.Part{f.part()}, nil
}

func (f *fakeService) Update(ctx context.Context, id uuid.UUID, data domain.PartData) (domain.Part, error) {
	current, err := f.Get(ctx, id)
	if err != nil {
		return domain.Part{}, err
	}
	return current.Update(data, testTime.Add(time.Hour))
}

func (f *fakeService) Delete(_ context.Context, id uuid.UUID) error {
	if id != knownID {
		return missing()
	}
	return nil
}

func (f *fakeService) RecordMovement(ctx context.Context, partID uuid.UUID, data domain.MovementData) (domain.Part, domain.StockMovement, error) {
	if f.failWith != nil {
		return domain.Part{}, domain.StockMovement{}, f.failWith
	}
	part, err := f.Get(ctx, partID)
	if err != nil {
		return domain.Part{}, domain.StockMovement{}, err
	}
	movement, err := domain.NewStockMovement(uuid.MustParse("33333333-3333-3333-3333-333333333333"), partID, data, testTime)
	if err != nil {
		return domain.Part{}, domain.StockMovement{}, err
	}
	if movement.Type == domain.MovementOut {
		part.Stock -= movement.Quantity
	} else {
		part.Stock += movement.Quantity
	}
	return part, movement, nil
}

func (f *fakeService) ListMovements(ctx context.Context, partID uuid.UUID, limit, offset int) ([]domain.StockMovement, error) {
	f.listCalls = append(f.listCalls, [2]int{limit, offset})
	if _, err := f.Get(ctx, partID); err != nil {
		return nil, err
	}
	m, _ := domain.NewStockMovement(uuid.New(), partID, domain.MovementData{Type: "IN", Quantity: 5, Reason: "compra"}, testTime)
	return []domain.StockMovement{m}, nil
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
	handler := NewPartHandler(svc, logger)
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

const partBody = `{"sku":"flt-01","name":"Filtro de óleo","description":"Motor 1.0","unit_price_cents":3590}`

func TestAllRoutesRequireAuthentication(t *testing.T) {
	h := setup(&fakeService{})
	path := "/admin/parts/" + knownID.String()

	routes := []struct{ method, path string }{
		{http.MethodPost, "/admin/parts"},
		{http.MethodGet, "/admin/parts"},
		{http.MethodGet, path},
		{http.MethodPut, path},
		{http.MethodDelete, path},
		{http.MethodPost, path + "/stock-movements"},
		{http.MethodGet, path + "/stock-movements"},
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
	rec := call(setup(&fakeService{}), http.MethodPost, "/admin/parts", partBody, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/admin/parts/"+knownID.String() {
		t.Errorf("Location = %q", got)
	}
	assertBody(t, rec, `"sku":"FLT-01"`, `"unit_price_cents":3590`, `"stock_quantity":0`)
}

func TestCreateRejectsStockInBody(t *testing.T) {
	body := `{"sku":"FLT-01","name":"Filtro","unit_price_cents":100,"stock_quantity":999}`

	rec := call(setup(&fakeService{}), http.MethodPost, "/admin/parts", body, true)

	if rec.Code != 400 {
		t.Errorf("status = %d, esperado 400: o saldo não pode ser definido pelo cadastro", rec.Code)
	}
	assertBody(t, rec, `"code":"INVALID_REQUEST"`)
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
		{"preço omitido", &fakeService{}, `{"sku":"FLT-01","name":"Filtro"}`, 400, "INVALID_PRICE"},
		{"SKU inválido", &fakeService{}, `{"sku":"a b","name":"Filtro","unit_price_cents":1}`, 400, "INVALID_SKU"},
		{"SKU duplicado", &fakeService{failWith: apperr.Conflict(domain.CodeSKUTaken, "código já cadastrado")}, partBody, 409, "SKU_ALREADY_REGISTERED"},
		{"erro interno", &fakeService{failWith: errors.New("banco caiu")}, partBody, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(tt.svc), http.MethodPost, "/admin/parts", tt.body, true)

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
		rec := call(setup(svc), http.MethodGet, "/admin/parts", "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"items":[{`, `"limit":50`, `"stock_quantity":8`)
		if len(svc.listCalls) != 1 || svc.listCalls[0] != [2]int{50, 0} {
			t.Errorf("chamadas = %v", svc.listCalls)
		}
	})

	t.Run("paginação inválida", func(t *testing.T) {
		rec := call(setup(&fakeService{}), http.MethodGet, "/admin/parts?limit=0", "", true)

		if rec.Code != 400 {
			t.Errorf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"code":"INVALID_PAGINATION"`)
	})

	t.Run("erro do serviço", func(t *testing.T) {
		if rec := call(setup(&fakeService{failWith: errors.New("x")}), http.MethodGet, "/admin/parts", "", true); rec.Code != 500 {
			t.Errorf("status = %d", rec.Code)
		}
	})
}

func TestListBySKU(t *testing.T) {
	h := setup(&fakeService{})

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantBody   string
	}{
		{"encontrada", "?sku=FLT-01", 200, `"sku":"FLT-01"`},
		{"minúsculas", "?sku=flt-01", 200, `"sku":"FLT-01"`},
		{"não cadastrada devolve lista vazia", "?sku=XYZ", 200, `"items":[]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(h, http.MethodGet, "/admin/parts"+tt.query, "", true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, tt.wantBody)
		})
	}

	rec := call(setup(&fakeService{failWith: errors.New("x")}), http.MethodGet, "/admin/parts?sku=FLT-01", "", true)
	if rec.Code != 500 {
		t.Errorf("erro do serviço na busca por SKU: status = %d", rec.Code)
	}
}

func TestGet(t *testing.T) {
	h := setup(&fakeService{})

	if rec := call(h, http.MethodGet, "/admin/parts/"+knownID.String(), "", true); rec.Code != 200 {
		t.Errorf("existente: status = %d", rec.Code)
	}
	rec := call(h, http.MethodGet, "/admin/parts/"+uuid.NewString(), "", true)
	if rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	assertBody(t, rec, `"code":"PART_NOT_FOUND"`)

	if rec := call(h, http.MethodGet, "/admin/parts/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestUpdate(t *testing.T) {
	h := setup(&fakeService{})
	path := "/admin/parts/" + knownID.String()

	rec := call(h, http.MethodPut, path, `{"sku":"VELA-01","name":"Vela","unit_price_cents":2500}`, true)
	if rec.Code != 200 {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
	}
	assertBody(t, rec, `"sku":"VELA-01"`, `"stock_quantity":8`, `"updated_at":"2026-03-01T11:00:00Z"`)

	if rec := call(h, http.MethodPut, path, `{"name":"x"}`, true); rec.Code != 400 {
		t.Errorf("dados inválidos: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, path, `{`, true); rec.Code != 400 {
		t.Errorf("JSON inválido: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/parts/"+uuid.NewString(), partBody, true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/admin/parts/abc", partBody, true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestDelete(t *testing.T) {
	h := setup(&fakeService{})

	rec := call(h, http.MethodDelete, "/admin/parts/"+knownID.String(), "", true)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("status = %d, corpo = %q", rec.Code, rec.Body)
	}
	if rec := call(h, http.MethodDelete, "/admin/parts/"+uuid.NewString(), "", true); rec.Code != 404 {
		t.Errorf("inexistente: status = %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, "/admin/parts/abc", "", true); rec.Code != 400 {
		t.Errorf("id inválido: status = %d", rec.Code)
	}
}

func TestRecordMovement(t *testing.T) {
	h := setup(&fakeService{})
	path := "/admin/parts/" + knownID.String() + "/stock-movements"

	in := call(h, http.MethodPost, path, `{"type":"IN","quantity":5,"reason":"Compra NF 10"}`, true)
	if in.Code != http.StatusCreated {
		t.Fatalf("entrada: status = %d (%s)", in.Code, in.Body)
	}
	assertBody(t, in, `"type":"IN"`, `"quantity":5`, `"reason":"Compra NF 10"`, `"part_id":"`+knownID.String()+`"`, `"stock_quantity":13`)

	out := call(h, http.MethodPost, path, `{"type":"OUT","quantity":3,"reason":"Uso na OS"}`, true)
	if out.Code != http.StatusCreated {
		t.Fatalf("saída: status = %d", out.Code)
	}
	assertBody(t, out, `"stock_quantity":5`)
}

func TestRecordMovementErrors(t *testing.T) {
	path := "/admin/parts/" + knownID.String() + "/stock-movements"

	tests := []struct {
		name       string
		svc        *fakeService
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"id inválido", &fakeService{}, "/admin/parts/abc/stock-movements", `{}`, 400, "INVALID_ID"},
		{"JSON inválido", &fakeService{}, path, `{`, 400, "INVALID_REQUEST"},
		{"tipo inválido", &fakeService{}, path, `{"type":"X","quantity":1,"reason":"r"}`, 400, "INVALID_MOVEMENT_TYPE"},
		{"quantidade zero", &fakeService{}, path, `{"type":"IN","quantity":0,"reason":"r"}`, 400, "INVALID_QUANTITY"},
		{"sem motivo", &fakeService{}, path, `{"type":"IN","quantity":1}`, 400, "INVALID_REASON"},
		{"peça inexistente", &fakeService{}, "/admin/parts/" + uuid.NewString() + "/stock-movements", `{"type":"IN","quantity":1,"reason":"r"}`, 404, "PART_NOT_FOUND"},
		{"estoque insuficiente", &fakeService{failWith: apperr.Conflict(domain.CodeInsufficientStock, "estoque insuficiente")}, path, `{"type":"OUT","quantity":99,"reason":"r"}`, 409, "INSUFFICIENT_STOCK"},
		{"erro interno", &fakeService{failWith: errors.New("x")}, path, `{"type":"IN","quantity":1,"reason":"r"}`, 500, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(tt.svc), http.MethodPost, tt.path, tt.body, true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}

func TestListMovements(t *testing.T) {
	path := "/admin/parts/" + knownID.String() + "/stock-movements"

	t.Run("lista", func(t *testing.T) {
		svc := &fakeService{}
		rec := call(setup(svc), http.MethodGet, path+"?limit=5&offset=2", "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		assertBody(t, rec, `"items":[{`, `"type":"IN"`, `"limit":5`, `"offset":2`)
		if len(svc.listCalls) != 1 || svc.listCalls[0] != [2]int{5, 2} {
			t.Errorf("chamadas = %v", svc.listCalls)
		}
	})

	errorCases := []struct {
		name, path string
		wantStatus int
		wantCode   string
	}{
		{"id inválido", "/admin/parts/abc/stock-movements", 400, "INVALID_ID"},
		{"paginação inválida", path + "?limit=0", 400, "INVALID_PAGINATION"},
		{"peça inexistente", "/admin/parts/" + uuid.NewString() + "/stock-movements", 404, "PART_NOT_FOUND"},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := call(setup(&fakeService{}), http.MethodGet, tt.path, "", true)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			assertBody(t, rec, `"code":"`+tt.wantCode+`"`)
		})
	}
}
