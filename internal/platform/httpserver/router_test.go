package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingerFake struct{ err error }

func (p pingerFake) Ping(context.Context) error { return p.err }

func executar(t *testing.T, db Pinger, caminho string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	rec := httptest.NewRecorder()
	NewRouter(logger, db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, caminho, nil))
	return rec
}

func TestHealth(t *testing.T) {
	rec := executar(t, pingerFake{}, "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", rec.Code)
	}
}

func TestReady(t *testing.T) {
	if rec := executar(t, pingerFake{}, "/ready"); rec.Code != http.StatusOK {
		t.Errorf("banco ok: status = %d, esperado 200", rec.Code)
	}
	if rec := executar(t, pingerFake{err: errors.New("fora")}, "/ready"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("banco fora: status = %d, esperado 503", rec.Code)
	}
}
