package app_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oficina-api/internal/platform/testdb"
)

// public faz uma chamada como o cliente: sem login, só com o CPF/CNPJ no cabeçalho.
func (a *api) public(method, path, document string) (int, resp, string) {
	a.t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if document != "" {
		req.Header.Set("X-Customer-Document", document)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)

	out := resp{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			a.t.Fatalf("resposta não é JSON (%d): %s", rec.Code, rec.Body)
		}
	}
	return rec.Code, out, rec.Body.String()
}

func (a *api) mustPublic(want int, method, path, document string) resp {
	a.t.Helper()
	status, out, _ := a.public(method, path, document)
	if status != want {
		a.t.Fatalf("%s %s: status %d, esperado %d: %v", method, path, status, want, out)
	}
	return out
}

func orderPath(order resp) string {
	return fmt.Sprintf("/public/work-orders/%d", int(order["number"].(float64)))
}

func TestCustomerFollowsAndApprovesTheOrder(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	order := a.newOrder(w, part(w.filterID, 3))
	id := a.toAwaitingApproval(order)
	path := orderPath(order)

	seen := a.mustPublic(200, "GET", path, w.document)
	if seen["status"] != "AWAITING_APPROVAL" || seen["status_label"] != "aguardando aprovação" || seen["total_cents"] != float64(22770) {
		t.Errorf("o cliente deveria ver o orçamento aguardando aprovação: %v", seen)
	}
	if items := seen["items"].([]any); len(items) != 2 || items[1].(map[string]any)["name"] != "Filtro de óleo" {
		t.Errorf("itens do orçamento: %v", seen["items"])
	}

	approved := a.mustPublic(200, "POST", path+"/approve", w.document)
	if approved["status"] != "IN_EXECUTION" || approved["started_at"] == nil {
		t.Errorf("aprovação do cliente: %v", approved)
	}
	if got := a.stock(w.filterID); got != 7 {
		t.Errorf("estoque = %d, esperado 7: a aprovação do cliente também dá baixa", got)
	}

	// o atendente conclui o serviço e o cliente acompanha
	a.must(200, "POST", "/admin/work-orders/"+id+"/complete", nil)
	if got := a.mustPublic(200, "GET", path, w.document)["status"]; got != "COMPLETED" {
		t.Errorf("o cliente deveria ver COMPLETED, viu %v", got)
	}

	a.must(200, "POST", "/admin/work-orders/"+id+"/deliver", nil)
	if got := a.mustPublic(200, "GET", path, w.document)["status_label"]; got != "entregue" {
		t.Errorf("o cliente deveria ver 'entregue', viu %v", got)
	}
}

func TestCustomerRejectsTheQuote(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	order := a.newOrder(w, part(w.filterID, 3))
	id := a.toAwaitingApproval(order)

	rejected := a.mustPublic(200, "POST", orderPath(order)+"/reject", w.document)

	if rejected["status"] != "CANCELLED" || a.status(id) != "CANCELLED" {
		t.Errorf("recusa: %v", rejected)
	}
	if a.stock(w.filterID) != 10 {
		t.Error("recusar não pode mexer no estoque")
	}
}

func TestCustomerApprovalStillRequiresStock(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(1, 10)
	order := a.newOrder(w, part(w.filterID, 5))
	id := a.toAwaitingApproval(order)

	status, out, _ := a.public("POST", orderPath(order)+"/approve", w.document)

	if status != 409 || errorCode(out) != "INSUFFICIENT_STOCK" {
		t.Fatalf("%d %v, esperado 409 INSUFFICIENT_STOCK", status, out)
	}
	if a.status(id) != "AWAITING_APPROVAL" || a.stock(w.filterID) != 1 {
		t.Error("sem estoque, a OS deve continuar aguardando e o saldo ficar intacto")
	}
}

func TestCustomerOnlySeesAndActsOnTheirOwnOrders(t *testing.T) {
	a := newAPI(t)
	mine := a.newWorld(10, 10)
	other := a.newWorld(10, 10)

	order := a.newOrder(mine, part(mine.filterID, 3))
	id := a.toAwaitingApproval(order)
	path := orderPath(order)

	// O mesmo erro para qualquer tentativa de identificação que falhe.
	_, _, ghostBody := a.public("GET", "/public/work-orders/999999999", mine.document)
	attempts := map[string]string{
		"documento de outro cliente":      other.document,
		"documento válido e sem cadastro": testdb.RandomCPF(),
		"documento inválido":              "123",
		"documento só com formatação":     "000.000.000-00",
	}
	for name, document := range attempts {
		t.Run(name, func(t *testing.T) {
			status, _, body := a.public("GET", path, document)

			if status != 404 {
				t.Errorf("status = %d, esperado 404", status)
			}
			if body != ghostBody {
				t.Errorf("a resposta deveria ser idêntica à de uma OS que não existe:\n%s\n%s", body, ghostBody)
			}
		})
	}

	t.Run("outro cliente não aprova nem recusa", func(t *testing.T) {
		for _, action := range []string{"approve", "reject"} {
			if status, _, _ := a.public("POST", path+"/"+action, other.document); status != 404 {
				t.Errorf("%s por outro cliente: status %d", action, status)
			}
		}
		if a.status(id) != "AWAITING_APPROVAL" || a.stock(mine.filterID) != 10 {
			t.Error("a OS de um cliente não pode ser alterada por outro")
		}
	})

	t.Run("sem o cabeçalho do documento", func(t *testing.T) {
		status, out, _ := a.public("GET", path, "")
		if status != 400 || errorCode(out) != "DOCUMENT_REQUIRED" {
			t.Errorf("%d %v", status, out)
		}
	})

	t.Run("número inválido", func(t *testing.T) {
		if status, _, _ := a.public("GET", "/public/work-orders/abc", mine.document); status != 400 {
			t.Errorf("status = %d", status)
		}
	})

	t.Run("o documento também vale com pontuação", func(t *testing.T) {
		formatted := mine.document[0:3] + "." + mine.document[3:6] + "." + mine.document[6:9] + "-" + mine.document[9:]
		a.mustPublic(200, "GET", path, formatted)
	})
}

func TestPublicResponseHasNoInternalIdentifiers(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	order := a.newOrder(w, part(w.filterID, 1))

	_, _, body := a.public("GET", orderPath(order), w.document)

	for name, internal := range map[string]string{
		"id da OS": order["id"].(string), "id do cliente": w.customerID, "id do veículo": w.vehicleID,
		"id do serviço": w.serviceID, "id da peça": w.filterID, "CPF": w.document,
	} {
		if strings.Contains(body, internal) {
			t.Errorf("a resposta pública vazou o %s", name)
		}
	}
}

func TestPublicApprovalIsSafeUnderConcurrency(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	order := a.newOrder(w, part(w.filterID, 3))
	a.toAwaitingApproval(order)
	path := orderPath(order) + "/approve"

	results := make(chan int, 6)
	for i := 0; i < cap(results); i++ {
		go func() {
			status, _, _ := a.public("POST", path, w.document)
			results <- status
		}()
	}

	counts := map[int]int{}
	for i := 0; i < cap(results); i++ {
		counts[<-results]++
	}

	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != cap(results)-1 {
		t.Errorf("respostas = %v; esperado uma 200 e as demais 409", counts)
	}
	if got := a.stock(w.filterID); got != 7 {
		t.Errorf("estoque = %d, esperado 7: a baixa só pode acontecer uma vez", got)
	}
}

func TestExecutionTimeMetric(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	id := a.toAwaitingApproval(a.newOrder(w, part(w.filterID, 1)))
	a.must(200, "POST", "/admin/work-orders/"+id+"/approve", nil)
	a.must(200, "POST", "/admin/work-orders/"+id+"/complete", nil)

	today := time.Now().UTC().Format(time.DateOnly)
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format(time.DateOnly)

	t.Run("inclui a OS finalizada hoje", func(t *testing.T) {
		got := a.must(200, "GET", "/admin/metrics/execution-time?from="+today+"&to="+tomorrow, nil)

		if got["completed_orders"].(float64) < 1 {
			t.Errorf("deveria contar a OS finalizada: %v", got)
		}
		if _, ok := got["average_execution_minutes"].(float64); !ok {
			t.Errorf("faltou a média: %v", got)
		}
	})

	t.Run("período no passado não inclui", func(t *testing.T) {
		got := a.must(200, "GET", "/admin/metrics/execution-time?from=2001-01-01&to=2001-02-01", nil)

		if got["completed_orders"] != float64(0) || got["average_execution_minutes"] != float64(0) {
			t.Errorf("esperado zero: %v", got)
		}
	})

	t.Run("exige autenticação", func(t *testing.T) {
		if status, _ := a.request("GET", "/admin/metrics/execution-time", nil, ""); status != 401 {
			t.Errorf("status = %d", status)
		}
	})

	t.Run("período inválido", func(t *testing.T) {
		status, out := a.do("GET", "/admin/metrics/execution-time?from=amanhã", nil)
		if status != 400 || errorCode(out) != "INVALID_PERIOD" {
			t.Errorf("%d %v", status, out)
		}
	})
}
