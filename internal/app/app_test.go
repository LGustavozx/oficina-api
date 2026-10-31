package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"oficina-api/internal/app"
	identitydomain "oficina-api/internal/identity/domain"
	identityinfra "oficina-api/internal/identity/infrastructure"
	"oficina-api/internal/platform/config"
	"oficina-api/internal/platform/testdb"
	registrydomain "oficina-api/internal/registry/domain"
	"oficina-api/internal/shared/clock"
)

// Estes testes sobem a aplicação inteira (handlers, serviços, repositórios e
// banco real) e a usam só por HTTP, como um cliente da API faria.

type api struct {
	t       *testing.T
	db      *sql.DB
	handler http.Handler
	token   string
}

type resp map[string]any

func newAPI(t *testing.T) *api {
	t.Helper()
	db := testdb.Open(t)
	ctx := context.Background()

	email, err := identitydomain.NewEmail("fluxo-" + uuid.NewString() + "@oficina.com")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := identityinfra.NewBcryptHasher(bcrypt.MinCost).Hash("senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	user := identitydomain.User{ID: uuid.New(), Email: email, PasswordHash: hash, CreatedAt: time.Now().UTC()}
	if err := identityinfra.NewUserRepository(db).Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM usuarios WHERE id = $1`, user.ID) })

	cfg := config.Config{JWTSecret: "0123456789abcdef0123456789abcdef", JWTTTL: time.Hour}
	handler, err := app.NewHandler(ctx, db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), clock.System{})
	if err != nil {
		t.Fatal(err)
	}

	a := &api{t: t, db: db, handler: handler}
	status, body := a.do(http.MethodPost, "/auth/login", map[string]any{"email": email.String(), "password": "senha-forte-123"})
	if status != http.StatusOK {
		t.Fatalf("login falhou: %d %v", status, body)
	}
	a.token = body["token"].(string)
	return a
}

func (a *api) do(method, path string, body any) (int, resp) {
	return a.request(method, path, body, a.token)
}

func (a *api) request(method, path string, body any, token string) (int, resp) {
	a.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)

	out := resp{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			a.t.Fatalf("resposta não é JSON (%d): %s", rec.Code, rec.Body)
		}
	}
	return rec.Code, out
}

// must faz a chamada e exige o status esperado.
func (a *api) must(want int, method, path string, body any) resp {
	a.t.Helper()
	status, out := a.do(method, path, body)
	if status != want {
		a.t.Fatalf("%s %s: status %d, esperado %d: %v", method, path, status, want, out)
	}
	return out
}

func (a *api) cleanup(query string, args ...any) {
	a.t.Cleanup(func() { _, _ = a.db.Exec(query, args...) })
}

func randomCPF() string {
	for {
		digits := make([]int, 11)
		for i := 0; i < 9; i++ {
			digits[i] = rand.IntN(10)
		}
		digit := func(base []int, first int) int {
			sum := 0
			for i, d := range base {
				sum += d * (first - i)
			}
			if rest := sum % 11; rest >= 2 {
				return 11 - rest
			}
			return 0
		}
		digits[9], digits[10] = digit(digits[:9], 10), 0
		digits[10] = digit(digits[:10], 11)

		var b strings.Builder
		for _, d := range digits {
			fmt.Fprint(&b, d)
		}
		if _, err := registrydomain.NewDocument(b.String()); err == nil {
			return b.String()
		}
	}
}

func randomPlate() string {
	return fmt.Sprintf("%c%c%c%04d", 'A'+rand.IntN(26), 'A'+rand.IntN(26), 'A'+rand.IntN(26), rand.IntN(10000))
}

// world é um cliente com veículo, um serviço e duas peças com estoque.
type world struct {
	customerID, vehicleID, serviceID, filterID, sparkID string
}

func (a *api) newWorld(filterStock, sparkStock int) world {
	a.t.Helper()
	tag := uuid.NewString()[:8]
	var w world

	w.customerID = a.must(201, "POST", "/admin/customers", map[string]any{"name": "Maria Souza", "document": randomCPF()})["id"].(string)
	a.cleanup(`DELETE FROM clientes WHERE id = $1`, w.customerID)

	w.vehicleID = a.must(201, "POST", "/admin/vehicles", map[string]any{
		"customer_id": w.customerID, "plate": randomPlate(), "brand": "Fiat", "model": "Uno", "year": 2010,
	})["id"].(string)
	a.cleanup(`DELETE FROM veiculos WHERE id = $1`, w.vehicleID)

	w.serviceID = a.must(201, "POST", "/admin/services", map[string]any{
		"name": "Troca de óleo " + tag, "price_cents": 12000, "estimated_minutes": 45,
	})["id"].(string)
	a.cleanup(`DELETE FROM servicos WHERE id = $1`, w.serviceID)

	w.filterID = a.newPart("FLT-"+strings.ToUpper(tag), "Filtro de óleo", 3590, filterStock)
	w.sparkID = a.newPart("VLA-"+strings.ToUpper(tag), "Vela de ignição", 2500, sparkStock)
	return w
}

func (a *api) newPart(sku, name string, priceCents, stock int) string {
	a.t.Helper()
	id := a.must(201, "POST", "/admin/parts", map[string]any{"sku": sku, "name": name, "unit_price_cents": priceCents})["id"].(string)
	a.cleanup(`DELETE FROM pecas WHERE id = $1`, id)
	a.cleanup(`DELETE FROM movimentacoes_estoque WHERE peca_id = $1`, id)
	if stock > 0 {
		a.must(201, "POST", "/admin/parts/"+id+"/stock-movements", map[string]any{"type": "IN", "quantity": stock, "reason": "estoque inicial"})
	}
	return id
}

func (a *api) newOrder(w world, parts ...map[string]any) resp {
	a.t.Helper()
	body := map[string]any{"vehicle_id": w.vehicleID, "description": "Barulho no motor", "services": []string{w.serviceID}, "parts": parts}
	order := a.must(201, "POST", "/admin/work-orders", body)
	a.cleanup(`DELETE FROM ordens_servico WHERE id = $1`, order["id"])
	return order
}

// newEmptyOrder abre uma OS sem nenhum serviço ou peça.
func (a *api) newEmptyOrder(w world) resp {
	a.t.Helper()
	order := a.must(201, "POST", "/admin/work-orders", map[string]any{"vehicle_id": w.vehicleID, "description": "Revisão geral"})
	a.cleanup(`DELETE FROM ordens_servico WHERE id = $1`, order["id"])
	return order
}

func (a *api) toAwaitingApproval(order resp) string {
	a.t.Helper()
	id := order["id"].(string)
	a.must(200, "POST", "/admin/work-orders/"+id+"/start-diagnosis", nil)
	a.must(200, "POST", "/admin/work-orders/"+id+"/send-quote", nil)
	return id
}

func (a *api) stock(partID string) int {
	a.t.Helper()
	return int(a.must(200, "GET", "/admin/parts/"+partID, nil)["stock_quantity"].(float64))
}

func (a *api) status(orderID string) string {
	a.t.Helper()
	return a.must(200, "GET", "/admin/work-orders/"+orderID, nil)["status"].(string)
}

func part(id string, qty int) map[string]any { return map[string]any{"part_id": id, "quantity": qty} }

func errorCode(out resp) string {
	code, _ := out["code"].(string)
	return code
}

// O caminho feliz completo, do cadastro à entrega.
func TestMainFlow(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)

	order := a.newOrder(w, part(w.filterID, 3))
	id := order["id"].(string)

	if order["status"] != "RECEIVED" || order["customer_id"] != w.customerID {
		t.Fatalf("a OS deveria nascer RECEIVED e com o cliente dono do veículo: %v", order)
	}
	// orçamento automático: 120,00 + 3 × 35,90 = 227,70
	if order["total_cents"] != float64(22770) {
		t.Errorf("total = %v, esperado 22770", order["total_cents"])
	}
	if order["number"].(float64) < 1 {
		t.Errorf("a OS deveria ter número: %v", order["number"])
	}

	diagnosis := a.must(200, "POST", "/admin/work-orders/"+id+"/start-diagnosis", nil)
	if diagnosis["status"] != "IN_DIAGNOSIS" {
		t.Errorf("status = %v", diagnosis["status"])
	}

	quote := a.must(200, "POST", "/admin/work-orders/"+id+"/send-quote", nil)
	if quote["status"] != "AWAITING_APPROVAL" {
		t.Errorf("status = %v", quote["status"])
	}
	if a.stock(w.filterID) != 10 {
		t.Error("o estoque não pode mudar antes da aprovação")
	}

	approved := a.must(200, "POST", "/admin/work-orders/"+id+"/approve", nil)
	if approved["status"] != "IN_EXECUTION" || approved["started_at"] == nil {
		t.Errorf("aprovação: %v", approved)
	}
	if got := a.stock(w.filterID); got != 7 {
		t.Errorf("estoque do filtro = %d, esperado 7 (10 - 3)", got)
	}
	if got := a.stock(w.sparkID); got != 10 {
		t.Errorf("a vela não está na OS e não pode ter baixa: %d", got)
	}

	history := a.must(200, "GET", "/admin/parts/"+w.filterID+"/stock-movements", nil)["items"].([]any)
	last := history[0].(map[string]any)
	wantReason := fmt.Sprintf("OS #%d", int(order["number"].(float64)))
	if last["type"] != "OUT" || last["quantity"] != float64(3) || last["reason"] != wantReason {
		t.Errorf("a baixa deveria ficar no histórico: %v (esperado motivo %q)", last, wantReason)
	}

	completed := a.must(200, "POST", "/admin/work-orders/"+id+"/complete", nil)
	if completed["status"] != "COMPLETED" || completed["completed_at"] == nil {
		t.Errorf("finalização: %v", completed)
	}

	delivered := a.must(200, "POST", "/admin/work-orders/"+id+"/deliver", nil)
	if delivered["status"] != "DELIVERED" || delivered["delivered_at"] == nil {
		t.Errorf("entrega: %v", delivered)
	}

	listed := a.must(200, "GET", "/admin/work-orders?status=DELIVERED&limit=200", nil)["items"].([]any)
	found := false
	for _, item := range listed {
		if item.(map[string]any)["id"] == id {
			found = true
		}
	}
	if !found {
		t.Error("a OS entregue deveria aparecer na listagem filtrada por status")
	}
}

func TestItemsCanBeReplacedBeforeTheQuoteIsSent(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	order := a.newOrder(w, part(w.filterID, 1))
	id := order["id"].(string)

	a.must(200, "POST", "/admin/work-orders/"+id+"/start-diagnosis", nil)
	updated := a.must(200, "PUT", "/admin/work-orders/"+id+"/items", map[string]any{
		"services": []string{w.serviceID},
		"parts":    []map[string]any{part(w.filterID, 2), part(w.sparkID, 4)},
	})
	// 120,00 + 2 × 35,90 + 4 × 25,00 = 291,80
	if updated["total_cents"] != float64(29180) || len(updated["items"].([]any)) != 3 {
		t.Errorf("itens não foram trocados: %v", updated)
	}

	a.must(200, "POST", "/admin/work-orders/"+id+"/send-quote", nil)
	status, out := a.do("PUT", "/admin/work-orders/"+id+"/items", map[string]any{})
	if status != 409 || errorCode(out) != "INVALID_STATUS_TRANSITION" {
		t.Errorf("depois do orçamento enviado, trocar itens deveria dar 409: %d %v", status, out)
	}
}

func TestRejectedQuoteCancelsWithoutTouchingStock(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	id := a.toAwaitingApproval(a.newOrder(w, part(w.filterID, 3)))

	rejected := a.must(200, "POST", "/admin/work-orders/"+id+"/reject", nil)

	if rejected["status"] != "CANCELLED" || rejected["started_at"] != nil {
		t.Errorf("recusa: %v", rejected)
	}
	if a.stock(w.filterID) != 10 {
		t.Error("recusar o orçamento não pode mexer no estoque")
	}
	if status, out := a.do("POST", "/admin/work-orders/"+id+"/approve", nil); status != 409 || errorCode(out) != "INVALID_STATUS_TRANSITION" {
		t.Errorf("uma OS cancelada não pode ser aprovada: %d %v", status, out)
	}
}

// Sem saldo para QUALQUER peça, nada muda: nem o status, nem a primeira peça.
func TestApprovalIsAllOrNothing(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 1)
	id := a.toAwaitingApproval(a.newOrder(w, part(w.filterID, 3), part(w.sparkID, 4)))

	status, out := a.do("POST", "/admin/work-orders/"+id+"/approve", nil)

	if status != 409 || errorCode(out) != "INSUFFICIENT_STOCK" {
		t.Fatalf("esperado 409 INSUFFICIENT_STOCK, veio %d %v", status, out)
	}
	if !strings.Contains(out["message"].(string), "Vela de ignição") {
		t.Errorf("a mensagem deveria citar a peça sem saldo: %v", out["message"])
	}
	if got := a.status(id); got != "AWAITING_APPROVAL" {
		t.Errorf("status = %s, a OS deveria continuar aguardando aprovação", got)
	}
	if a.stock(w.filterID) != 10 || a.stock(w.sparkID) != 1 {
		t.Errorf("a baixa da primeira peça deveria ter sido desfeita: filtro %d, vela %d", a.stock(w.filterID), a.stock(w.sparkID))
	}
	history := a.must(200, "GET", "/admin/parts/"+w.filterID+"/stock-movements", nil)["items"].([]any)
	if len(history) != 1 { // só a entrada inicial
		t.Errorf("o histórico não pode ter a baixa desfeita: %d registros", len(history))
	}

	a.must(201, "POST", "/admin/parts/"+w.sparkID+"/stock-movements", map[string]any{"type": "IN", "quantity": 5, "reason": "reposição"})
	a.must(200, "POST", "/admin/work-orders/"+id+"/approve", nil)
	if a.stock(w.filterID) != 7 || a.stock(w.sparkID) != 2 {
		t.Errorf("depois de repor, a aprovação deveria dar baixa nas duas: filtro %d, vela %d", a.stock(w.filterID), a.stock(w.sparkID))
	}
}

// Várias aprovações simultâneas da mesma OS: o estoque só pode ser consumido uma vez.
func TestConcurrentApprovalsConsumeStockOnce(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	id := a.toAwaitingApproval(a.newOrder(w, part(w.filterID, 3)))

	const attempts = 8
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		statuses = map[int]int{}
	)
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, _ := a.do("POST", "/admin/work-orders/"+id+"/approve", nil)
			mu.Lock()
			statuses[status]++
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()

	if statuses[200] != 1 || statuses[409] != attempts-1 {
		t.Errorf("respostas = %v; esperado uma 200 e %d 409", statuses, attempts-1)
	}
	if got := a.stock(w.filterID); got != 7 {
		t.Errorf("estoque = %d, esperado 7: a baixa de 3 unidades só pode acontecer uma vez", got)
	}
}

func TestReferencedRecordsCannotBeDeleted(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	a.newOrder(w, part(w.filterID, 1))

	tests := []struct {
		name, path, wantCode string
	}{
		{"cliente com OS", "/admin/customers/" + w.customerID, "CUSTOMER_IN_USE"},
		{"veículo com OS", "/admin/vehicles/" + w.vehicleID, "VEHICLE_IN_USE"},
		{"serviço em uso numa OS", "/admin/services/" + w.serviceID, "SERVICE_IN_USE"},
		{"peça em uso numa OS", "/admin/parts/" + w.filterID, "PART_IN_USE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, out := a.do("DELETE", tt.path, nil)

			if status != 409 || errorCode(out) != tt.wantCode {
				t.Errorf("%d %v, esperado 409 %s", status, out, tt.wantCode)
			}
		})
	}
}

func TestWorkOrderValidationAndAccess(t *testing.T) {
	a := newAPI(t)
	w := a.newWorld(10, 10)
	emptyID := a.newEmptyOrder(w)["id"].(string)

	t.Run("exige autenticação", func(t *testing.T) {
		for _, path := range []string{"/admin/work-orders", "/admin/work-orders/" + emptyID} {
			if status, _ := a.request("GET", path, nil, ""); status != 401 {
				t.Errorf("GET %s sem token: %d", path, status)
			}
		}
	})

	t.Run("veículo inexistente", func(t *testing.T) {
		status, out := a.do("POST", "/admin/work-orders", map[string]any{"vehicle_id": uuid.NewString(), "description": "x"})
		if status != 400 || errorCode(out) != "VEHICLE_NOT_FOUND" {
			t.Errorf("%d %v", status, out)
		}
	})

	t.Run("serviço e peça inexistentes", func(t *testing.T) {
		status, out := a.do("POST", "/admin/work-orders", map[string]any{"vehicle_id": w.vehicleID, "description": "x", "services": []string{uuid.NewString()}})
		if status != 400 || errorCode(out) != "SERVICE_NOT_FOUND" {
			t.Errorf("serviço: %d %v", status, out)
		}
		status, out = a.do("POST", "/admin/work-orders", map[string]any{"vehicle_id": w.vehicleID, "description": "x", "parts": []map[string]any{part(uuid.NewString(), 1)}})
		if status != 400 || errorCode(out) != "PART_NOT_FOUND" {
			t.Errorf("peça: %d %v", status, out)
		}
	})

	t.Run("orçamento sem itens", func(t *testing.T) {
		a.must(200, "POST", "/admin/work-orders/"+emptyID+"/start-diagnosis", nil)
		status, out := a.do("POST", "/admin/work-orders/"+emptyID+"/send-quote", nil)
		if status != 400 || errorCode(out) != "QUOTE_WITHOUT_ITEMS" {
			t.Errorf("%d %v", status, out)
		}
	})

	t.Run("ação fora de ordem", func(t *testing.T) {
		fresh := a.newEmptyOrder(w)["id"].(string)
		for _, action := range []string{"send-quote", "approve", "reject", "complete", "deliver"} {
			if status, out := a.do("POST", "/admin/work-orders/"+fresh+"/"+action, nil); status != 409 || errorCode(out) != "INVALID_STATUS_TRANSITION" {
				t.Errorf("%s numa OS recebida: %d %v", action, status, out)
			}
		}
	})

	t.Run("ordem inexistente", func(t *testing.T) {
		status, out := a.do("GET", "/admin/work-orders/"+uuid.NewString(), nil)
		if status != 404 || errorCode(out) != "WORK_ORDER_NOT_FOUND" {
			t.Errorf("%d %v", status, out)
		}
	})

	t.Run("status inválido no filtro", func(t *testing.T) {
		status, out := a.do("GET", "/admin/work-orders?status=XYZ", nil)
		if status != 400 || errorCode(out) != "INVALID_STATUS" {
			t.Errorf("%d %v", status, out)
		}
	})

	t.Run("preço congelado no momento da criação", func(t *testing.T) {
		id := a.newOrder(w, part(w.filterID, 2))["id"].(string)
		a.must(200, "PUT", "/admin/parts/"+w.filterID, map[string]any{"sku": "FLT-" + strings.ToUpper(uuid.NewString()[:8]), "name": "Filtro reajustado", "unit_price_cents": 99999})

		got := a.must(200, "GET", "/admin/work-orders/"+id, nil)

		// 120,00 + 2 × 35,90: o reajuste do catálogo não altera a OS existente
		if got["total_cents"] != float64(19180) {
			t.Errorf("total = %v, esperado 19180", got["total_cents"])
		}
	})
}
