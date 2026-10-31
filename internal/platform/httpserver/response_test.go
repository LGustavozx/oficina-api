package httpserver

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oficina-api/internal/shared/apperr"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validação", apperr.Validation("INVALID_X", "x inválido"), http.StatusBadRequest, "INVALID_X"},
		{"não encontrado", apperr.NotFound("NOT_FOUND_X", "x"), http.StatusNotFound, "NOT_FOUND_X"},
		{"conflito", apperr.Conflict("CONFLICT_X", "x"), http.StatusConflict, "CONFLICT_X"},
		{"não autenticado", apperr.Unauthorized("UNAUTH_X", "x"), http.StatusUnauthorized, "UNAUTH_X"},
		{"embrulhado em outro erro", errors.Join(errors.New("ctx"), apperr.NotFound("NOT_FOUND_X", "x")), http.StatusNotFound, "NOT_FOUND_X"},
		{"erro interno da aplicação", apperr.New(apperr.KindInternal, "BOOM", "detalhe"), http.StatusInternalServerError, "INTERNAL_ERROR"},
		{"erro comum", errors.New("senha do banco: 123"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			WriteError(rec, discardLogger(), tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), `"code":"`+tt.wantCode+`"`) {
				t.Errorf("corpo = %s, esperado código %s", rec.Body.String(), tt.wantCode)
			}
			if strings.Contains(rec.Body.String(), "senha do banco") || strings.Contains(rec.Body.String(), "detalhe") {
				t.Errorf("detalhe interno vazou para o cliente: %s", rec.Body.String())
			}
		})
	}
}

func TestWriteErrorUnauthorizedSetsChallenge(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, discardLogger(), apperr.Unauthorized("X", "x"))

	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q", got)
	}
}

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := NewHandler(fakePinger{}, logger, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /teste", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/teste?token=segredo", nil)
	req.Header.Set("Authorization", "Bearer segredo")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, want := range []string{`"method":"GET"`, `"path":"/teste"`, `"status":418`, `"duration_ms"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %s não contém %s", out, want)
		}
	}
	if strings.Contains(out, "segredo") {
		t.Errorf("log vazou dado sensível: %s", out)
	}
}

func TestRequestLoggerDefaultsTo200(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(fakePinger{}, slog.New(slog.NewJSONHandler(&buf, nil)))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if !strings.Contains(buf.String(), `"status":200`) {
		t.Errorf("log = %s, esperado status 200", buf.String())
	}
}
