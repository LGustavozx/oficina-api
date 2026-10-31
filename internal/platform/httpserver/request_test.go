package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oficina-api/internal/shared/apperr"
)

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != code || appErr.Kind != apperr.KindValidation {
		t.Fatalf("erro = %v, esperado validação %s", err, code)
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{"válido", `{"name":"x"}`, "x", false},
		{"campo desconhecido", `{"name":"x","admin":true}`, "", true},
		{"malformado", `{"name":`, "", true},
		{"vazio", ``, "", true},
		{"tipo errado", `{"name":1}`, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			var got payload

			err := DecodeJSON(httptest.NewRecorder(), req, &got)

			if tt.wantErr {
				wantCode(t, err, CodeInvalidRequest)
				return
			}
			if err != nil || got.Name != tt.want {
				t.Errorf("got %+v, err %v", got, err)
			}
		})
	}
}

func TestDecodeJSONRejectsBodyThatIsNotUTF8(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	// "Pe\xe7as" é "Peças" em Windows-1252: o byte 0xE7 sozinho não é UTF-8 válido.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{\"name\":\"Pe\xe7as\"}"))
	var got payload

	err := DecodeJSON(httptest.NewRecorder(), req, &got)

	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != CodeInvalidEncoding || appErr.Kind != apperr.KindValidation {
		t.Fatalf("erro = %v, esperado %s", err, CodeInvalidEncoding)
	}
	if !strings.Contains(appErr.Message, "UTF-8") {
		t.Errorf("a mensagem deveria citar UTF-8: %q", appErr.Message)
	}
	if got.Name != "" {
		t.Errorf("nada deveria ser decodificado: %+v", got)
	}
}

func TestDecodeJSONAcceptsValidUTF8WithAccents(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Troca de óleo ção"}`))
	var got payload

	if err := DecodeJSON(httptest.NewRecorder(), req, &got); err != nil || got.Name != "Troca de óleo ção" {
		t.Errorf("got %+v, err %v", got, err)
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	huge := `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(huge))

	err := DecodeJSON(httptest.NewRecorder(), req, &struct{ Name string }{})

	wantCode(t, err, CodeInvalidRequest)
}

func TestPathUUID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	if id, err := PathUUID(req, "id"); err != nil || id.String() != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("id = %v, err = %v", id, err)
	}

	req.SetPathValue("id", "abc")
	_, err := PathUUID(req, "id")
	wantCode(t, err, CodeInvalidID)
}

func TestPagination(t *testing.T) {
	tests := []struct {
		query      string
		wantLimit  int
		wantOffset int
		wantErr    bool
	}{
		{"", 50, 0, false},
		{"limit=10&offset=20", 10, 20, false},
		{"limit=200", 200, 0, false},
		{"limit=0", 0, 0, true},
		{"limit=201", 0, 0, true},
		{"limit=abc", 0, 0, true},
		{"offset=-1", 0, 0, true},
		{"offset=x", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)

			limit, offset, err := Pagination(req)

			if tt.wantErr {
				wantCode(t, err, CodeInvalidPaging)
				return
			}
			if err != nil || limit != tt.wantLimit || offset != tt.wantOffset {
				t.Errorf("limit=%d offset=%d err=%v", limit, offset, err)
			}
		})
	}
}
