package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

func TestError_MapsCategoriesToStatus(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"validação", apperr.Validation("V", "m"), http.StatusUnprocessableEntity},
		{"não encontrado", apperr.NotFound("N", "m"), http.StatusNotFound},
		{"conflito", apperr.Conflict("C", "m"), http.StatusConflict},
		{"não autorizado", apperr.Unauthorized("U", "m"), http.StatusUnauthorized},
		{"categoria desconhecida", &apperr.Error{Category: errors.New("x"), Code: "X", Message: "m"}, http.StatusInternalServerError},
		{"erro inesperado", errors.New("detalhe interno secreto"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Error(rec, c.err)
			if rec.Code != c.status {
				t.Errorf("status = %d, esperado %d", rec.Code, c.status)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("Content-Type = %q", got)
			}
		})
	}
}

func TestError_DoesNotLeakUnexpectedDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, errors.New("senha do banco: 123"))
	if strings.Contains(rec.Body.String(), "senha do banco") {
		t.Errorf("corpo vazou detalhe interno: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ERRO_INTERNO") {
		t.Errorf("corpo = %s", rec.Body.String())
	}
}

func TestDecode(t *testing.T) {
	type payload struct {
		Name string `json:"nome"`
	}
	cases := []struct {
		name string
		body string
		code string
	}{
		{"válido", `{"nome":"x"}`, ""},
		{"campo desconhecido", `{"nome":"x","admin":true}`, "JSON_INVALIDO"},
		{"json malformado", `{"nome":`, "JSON_INVALIDO"},
		{"conteúdo extra", `{"nome":"x"}{"nome":"y"}`, "JSON_INVALIDO"},
		{"vazio", ``, "JSON_INVALIDO"},
		{"grande demais", `{"nome":"` + strings.Repeat("a", maxBodyBytes) + `"}`, "CORPO_MUITO_GRANDE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body))
			var p payload
			err := Decode(httptest.NewRecorder(), req, &p)
			if got := apperr.CodeOf(err); got != c.code {
				t.Errorf("código = %q, esperado %q (erro: %v)", got, c.code, err)
			}
		})
	}
}
