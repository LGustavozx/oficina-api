package http

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

const metricPath = "/admin/metrics/execution-time"

func TestExecutionTimeMetric(t *testing.T) {
	t.Run("sem período", func(t *testing.T) {
		svc := &fakeService{}
		rec := call(setup(svc), http.MethodGet, metricPath, "", true)

		if rec.Code != 200 {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
		}
		// 93 min 20 s = 93,33 min, arredondado para uma casa
		assertBody(t, rec, `"completed_orders":4`, `"average_execution_minutes":93.3`)
		if !svc.statsArgs[0].IsZero() || !svc.statsArgs[1].IsZero() {
			t.Errorf("sem período, nada deveria ser filtrado: %v", svc.statsArgs)
		}
	})

	t.Run("com período", func(t *testing.T) {
		svc := &fakeService{}
		call(setup(svc), http.MethodGet, metricPath+"?from=2026-01-01&to=2026-02-01", "", true)

		wantFrom, wantTo := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		if !svc.statsArgs[0].Equal(wantFrom) || !svc.statsArgs[1].Equal(wantTo) {
			t.Errorf("período = %v", svc.statsArgs)
		}
	})

	t.Run("só um dos lados", func(t *testing.T) {
		svc := &fakeService{}
		call(setup(svc), http.MethodGet, metricPath+"?from=2026-01-01", "", true)

		if svc.statsArgs[0].IsZero() || !svc.statsArgs[1].IsZero() {
			t.Errorf("período = %v", svc.statsArgs)
		}
	})

	invalid := map[string]string{
		"data mal formatada": "?from=01/01/2026",
		"to mal formatado":   "?to=ontem",
		"from igual a to":    "?from=2026-01-01&to=2026-01-01",
		"from depois de to":  "?from=2026-03-01&to=2026-01-01",
	}
	for name, query := range invalid {
		t.Run(name, func(t *testing.T) {
			rec := call(setup(&fakeService{}), http.MethodGet, metricPath+query, "", true)

			if rec.Code != 400 {
				t.Errorf("status = %d, esperado 400", rec.Code)
			}
			assertBody(t, rec, `"code":"INVALID_PERIOD"`)
		})
	}

	t.Run("exige autenticação", func(t *testing.T) {
		if rec := call(setup(&fakeService{}), http.MethodGet, metricPath, "", false); rec.Code != 401 {
			t.Errorf("status = %d", rec.Code)
		}
	})

	t.Run("erro do serviço", func(t *testing.T) {
		if rec := call(setup(&fakeService{failWith: errors.New("x")}), http.MethodGet, metricPath, "", true); rec.Code != 500 {
			t.Errorf("status = %d", rec.Code)
		}
	})
}
