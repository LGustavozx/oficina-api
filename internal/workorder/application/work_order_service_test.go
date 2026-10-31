package application

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/workorder/domain"
)

type fakeRepo struct {
	orders  map[uuid.UUID]domain.WorkOrder
	nextNum int64
	saveErr error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{orders: map[uuid.UUID]domain.WorkOrder{}, nextNum: 1} }

func (r *fakeRepo) Create(_ context.Context, o domain.WorkOrder) (domain.WorkOrder, error) {
	o.Number = r.nextNum
	r.nextNum++
	r.orders[o.ID] = o
	return o, nil
}

func (r *fakeRepo) Save(_ context.Context, o domain.WorkOrder, expected domain.Status) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	stored, ok := r.orders[o.ID]
	if !ok || stored.Status != expected {
		return apperr.Conflict(domain.CodeWorkOrderChanged, "alterada por outra operação")
	}
	o.Number = stored.Number
	r.orders[o.ID] = o
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	o, ok := r.orders[id]
	if !ok {
		return domain.WorkOrder{}, apperr.NotFound(domain.CodeWorkOrderNotFound, "ordem de serviço não encontrada")
	}
	return o, nil
}

func (r *fakeRepo) List(_ context.Context, status domain.Status, limit, offset int) ([]domain.WorkOrder, error) {
	var all []domain.WorkOrder
	for _, o := range r.orders {
		if status == "" || o.Status == status {
			all = append(all, o)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Number < all[j].Number })

	if offset >= len(all) {
		return nil, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

type fakeWorld struct {
	repo     *fakeRepo
	stock    map[uuid.UUID]int
	consumed []string // "partID:quantity:motivo", na ordem
	stockErr error

	lookupErr     error // erro inesperado ao buscar o veículo
	itemLookupErr error // erro inesperado ao buscar serviço ou peça

	vehicles map[uuid.UUID]uuid.UUID // veículo -> cliente
	services map[uuid.UUID]ItemInfo
	parts    map[uuid.UUID]ItemInfo
}

func newWorld() *fakeWorld {
	return &fakeWorld{
		repo:     newFakeRepo(),
		stock:    map[uuid.UUID]int{},
		vehicles: map[uuid.UUID]uuid.UUID{},
		services: map[uuid.UUID]ItemInfo{},
		parts:    map[uuid.UUID]ItemInfo{},
	}
}

func notFound(code string) error { return apperr.NotFound(code, "não encontrado") }

func (w *fakeWorld) CustomerOf(_ context.Context, vehicleID uuid.UUID) (uuid.UUID, error) {
	if w.lookupErr != nil {
		return uuid.Nil, w.lookupErr
	}
	c, ok := w.vehicles[vehicleID]
	if !ok {
		return uuid.Nil, notFound("VEHICLE_NOT_FOUND")
	}
	return c, nil
}

func (w *fakeWorld) Service(_ context.Context, id uuid.UUID) (ItemInfo, error) {
	if w.itemLookupErr != nil {
		return ItemInfo{}, w.itemLookupErr
	}
	info, ok := w.services[id]
	if !ok {
		return ItemInfo{}, notFound("SERVICE_NOT_FOUND")
	}
	return info, nil
}

func (w *fakeWorld) Part(_ context.Context, id uuid.UUID) (ItemInfo, error) {
	info, ok := w.parts[id]
	if !ok {
		return ItemInfo{}, notFound("PART_NOT_FOUND")
	}
	return info, nil
}

func (w *fakeWorld) Consume(_ context.Context, partID uuid.UUID, quantity int, reason string) error {
	if w.stockErr != nil {
		return w.stockErr
	}
	if w.stock[partID] < quantity {
		return apperr.Conflict(codeInsufficientStock, "estoque insuficiente para a saída")
	}
	w.stock[partID] -= quantity
	w.consumed = append(w.consumed, partID.String()+":"+string(rune('0'+quantity))+":"+reason)
	return nil
}

// Run simula a transação: se fn falhar, o estado de ordens e estoque volta ao que era.
func (w *fakeWorld) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	ordersBefore := make(map[uuid.UUID]domain.WorkOrder, len(w.repo.orders))
	for k, v := range w.repo.orders {
		ordersBefore[k] = v
	}
	stockBefore := make(map[uuid.UUID]int, len(w.stock))
	for k, v := range w.stock {
		stockBefore[k] = v
	}
	consumedBefore := len(w.consumed)

	err := fn(ctx)
	if err != nil {
		w.repo.orders, w.stock, w.consumed = ordersBefore, stockBefore, w.consumed[:consumedBefore]
	}
	return err
}

var now = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

type fixture struct {
	world       *fakeWorld
	svc         *WorkOrderService
	vehicleID   uuid.UUID
	customerID  uuid.UUID
	oilChangeID uuid.UUID
	filterID    uuid.UUID
	sparkID     uuid.UUID
}

func setup() fixture {
	w := newWorld()
	f := fixture{world: w, vehicleID: uuid.New(), customerID: uuid.New(), oilChangeID: uuid.New(), filterID: uuid.New(), sparkID: uuid.New()}

	w.vehicles[f.vehicleID] = f.customerID
	w.services[f.oilChangeID] = ItemInfo{Name: "Troca de óleo", UnitPrice: money.FromCents(12000)}
	w.parts[f.filterID] = ItemInfo{Name: "Filtro de óleo", UnitPrice: money.FromCents(3590)}
	w.parts[f.sparkID] = ItemInfo{Name: "Vela", UnitPrice: money.FromCents(2500)}
	w.stock[f.filterID] = 10
	w.stock[f.sparkID] = 10

	f.svc = NewWorkOrderService(w.repo, w, w, w, w, clock.Fixed{Time: now})
	return f
}

func (f fixture) order(t *testing.T) domain.WorkOrder {
	t.Helper()
	order, err := f.svc.Create(context.Background(), f.vehicleID, "Barulho no motor", ItemsInput{
		ServiceIDs: []uuid.UUID{f.oilChangeID},
		Parts:      []PartInput{{PartID: f.filterID, Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func (f fixture) awaitingApproval(t *testing.T) domain.WorkOrder {
	t.Helper()
	ctx := context.Background()
	order := f.order(t)
	for _, step := range []func(context.Context, uuid.UUID) (domain.WorkOrder, error){f.svc.StartDiagnosis, f.svc.SendQuote} {
		var err error
		if order, err = step(ctx, order.ID); err != nil {
			t.Fatal(err)
		}
	}
	return order
}

func wantErr(t *testing.T, err error, kind apperr.Kind, code string) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("erro = %v, esperado %s/%s", err, kind, code)
	}
}

func TestCreateDerivesCustomerAndSnapshotsPrices(t *testing.T) {
	f := setup()

	order := f.order(t)

	if order.CustomerID != f.customerID || order.VehicleID != f.vehicleID {
		t.Errorf("cliente/veículo errados: %+v", order)
	}
	if order.Number != 1 || order.Status != domain.StatusReceived || !order.CreatedAt.Equal(now) {
		t.Errorf("número/status/data errados: %+v", order)
	}
	if len(order.Items) != 2 || order.Items[0].Name != "Troca de óleo" || order.Items[1].Name != "Filtro de óleo" {
		t.Errorf("itens errados: %+v", order.Items)
	}
	// 120,00 + 2 × 35,90 = 191,80
	if order.Total().Cents() != 19180 {
		t.Errorf("total = %d, esperado 19180", order.Total().Cents())
	}

	// O preço fica congelado: mudar o catálogo depois não altera a ordem de serviço.
	f.world.parts[f.filterID] = ItemInfo{Name: "Filtro de óleo", UnitPrice: money.FromCents(99999)}
	got, _ := f.svc.Get(context.Background(), order.ID)
	if got.Total().Cents() != 19180 {
		t.Errorf("o orçamento mudou junto com o catálogo: %d", got.Total().Cents())
	}
}

func TestCreateWithoutItems(t *testing.T) {
	f := setup()

	order, err := f.svc.Create(context.Background(), f.vehicleID, "Revisão geral", ItemsInput{})

	if err != nil || len(order.Items) != 0 {
		t.Errorf("itens são opcionais na criação: %+v (%v)", order, err)
	}
}

func TestCreateErrors(t *testing.T) {
	f := setup()
	ctx := context.Background()

	tests := []struct {
		name     string
		vehicle  uuid.UUID
		desc     string
		in       ItemsInput
		wantKind apperr.Kind
		wantCode string
	}{
		{"veículo inexistente", uuid.New(), "x", ItemsInput{}, apperr.KindValidation, "VEHICLE_NOT_FOUND"},
		{"serviço inexistente", f.vehicleID, "x", ItemsInput{ServiceIDs: []uuid.UUID{uuid.New()}}, apperr.KindValidation, "SERVICE_NOT_FOUND"},
		{"peça inexistente", f.vehicleID, "x", ItemsInput{Parts: []PartInput{{PartID: uuid.New(), Quantity: 1}}}, apperr.KindValidation, "PART_NOT_FOUND"},
		{"descrição vazia", f.vehicleID, " ", ItemsInput{}, apperr.KindValidation, domain.CodeInvalidDescription},
		{"serviço repetido", f.vehicleID, "x", ItemsInput{ServiceIDs: []uuid.UUID{f.oilChangeID, f.oilChangeID}}, apperr.KindValidation, domain.CodeInvalidItems},
		{"quantidade zero", f.vehicleID, "x", ItemsInput{Parts: []PartInput{{PartID: f.filterID, Quantity: 0}}}, apperr.KindValidation, domain.CodeInvalidItems},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.svc.Create(ctx, tt.vehicle, tt.desc, tt.in)

			wantErr(t, err, tt.wantKind, tt.wantCode)
		})
	}
	if len(f.world.repo.orders) != 0 {
		t.Error("pedidos inválidos não devem gravar nada")
	}
}

// Só "não encontrado" vira erro de validação; falhas inesperadas chegam como estão.
func TestCreatePropagatesUnexpectedLookupErrors(t *testing.T) {
	boom := errors.New("banco fora do ar")

	f := setup()
	f.world.lookupErr = boom
	if _, err := f.svc.Create(context.Background(), f.vehicleID, "x", ItemsInput{}); !errors.Is(err, boom) {
		t.Errorf("erro do veículo = %v, esperado o original", err)
	}

	f = setup()
	f.world.itemLookupErr = boom
	_, err := f.svc.Create(context.Background(), f.vehicleID, "x", ItemsInput{ServiceIDs: []uuid.UUID{f.oilChangeID}})
	if !errors.Is(err, boom) {
		t.Errorf("erro do catálogo = %v, esperado o original", err)
	}
}

func TestGetAndList(t *testing.T) {
	f := setup()
	ctx := context.Background()
	first := f.order(t)
	second := f.order(t)
	if _, err := f.svc.StartDiagnosis(ctx, second.ID); err != nil {
		t.Fatal(err)
	}

	if got, err := f.svc.Get(ctx, first.ID); err != nil || got.ID != first.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	_, err := f.svc.Get(ctx, uuid.New())
	wantErr(t, err, apperr.KindNotFound, domain.CodeWorkOrderNotFound)

	all, err := f.svc.List(ctx, "", 10, 0)
	if err != nil || len(all) != 2 {
		t.Fatalf("sem filtro: %d (%v)", len(all), err)
	}
	filtered, err := f.svc.List(ctx, "IN_DIAGNOSIS", 10, 0)
	if err != nil || len(filtered) != 1 || filtered[0].ID != second.ID {
		t.Fatalf("filtrado: %+v (%v)", filtered, err)
	}
	_, err = f.svc.List(ctx, "ABC", 10, 0)
	wantErr(t, err, apperr.KindValidation, domain.CodeInvalidStatus)
}

func TestReplaceItems(t *testing.T) {
	f := setup()
	ctx := context.Background()
	order := f.order(t)

	updated, err := f.svc.ReplaceItems(ctx, order.ID, ItemsInput{Parts: []PartInput{{PartID: f.sparkID, Quantity: 4}}})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(updated.Items) != 1 || updated.Items[0].Name != "Vela" || updated.Total().Cents() != 10000 {
		t.Errorf("itens não foram trocados: %+v", updated.Items)
	}
	stored, _ := f.svc.Get(ctx, order.ID)
	if len(stored.Items) != 1 {
		t.Error("a troca não foi gravada")
	}

	_, err = f.svc.ReplaceItems(ctx, uuid.New(), ItemsInput{})
	wantErr(t, err, apperr.KindNotFound, domain.CodeWorkOrderNotFound)

	_, err = f.svc.ReplaceItems(ctx, order.ID, ItemsInput{ServiceIDs: []uuid.UUID{uuid.New()}})
	wantErr(t, err, apperr.KindValidation, "SERVICE_NOT_FOUND")

	_, err = f.svc.ReplaceItems(ctx, order.ID, ItemsInput{Parts: []PartInput{{PartID: f.sparkID, Quantity: 0}}})
	wantErr(t, err, apperr.KindValidation, domain.CodeInvalidItems)

	sent := f.awaitingApproval(t)
	_, err = f.svc.ReplaceItems(ctx, sent.ID, ItemsInput{})
	wantErr(t, err, apperr.KindConflict, domain.CodeInvalidTransition)
}

func TestTransitions(t *testing.T) {
	f := setup()
	ctx := context.Background()
	order := f.order(t)

	steps := []struct {
		name string
		run  func(context.Context, uuid.UUID) (domain.WorkOrder, error)
		want domain.Status
	}{
		{"StartDiagnosis", f.svc.StartDiagnosis, domain.StatusInDiagnosis},
		{"SendQuote", f.svc.SendQuote, domain.StatusAwaitingApproval},
		{"Approve", f.svc.Approve, domain.StatusInExecution},
		{"Complete", f.svc.Complete, domain.StatusCompleted},
		{"Deliver", f.svc.Deliver, domain.StatusDelivered},
	}

	for _, step := range steps {
		got, err := step.run(ctx, order.ID)
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got.Status != step.want {
			t.Fatalf("%s: status = %s, esperado %s", step.name, got.Status, step.want)
		}
		stored, _ := f.svc.Get(ctx, order.ID)
		if stored.Status != step.want {
			t.Fatalf("%s: o status não foi gravado (%s)", step.name, stored.Status)
		}
	}
}

func TestRejectCancelsWithoutTouchingStock(t *testing.T) {
	f := setup()
	order := f.awaitingApproval(t)

	got, err := f.svc.Reject(context.Background(), order.ID)

	if err != nil || got.Status != domain.StatusCancelled {
		t.Fatalf("Reject = %+v (%v)", got, err)
	}
	if f.world.stock[f.filterID] != 10 || len(f.world.consumed) != 0 {
		t.Error("recusar o orçamento não pode mexer no estoque")
	}
}

func TestTransitionErrors(t *testing.T) {
	f := setup()
	ctx := context.Background()
	order := f.order(t)

	actions := map[string]func(context.Context, uuid.UUID) (domain.WorkOrder, error){
		"StartDiagnosis": f.svc.StartDiagnosis, "SendQuote": f.svc.SendQuote, "Approve": f.svc.Approve,
		"Reject": f.svc.Reject, "Complete": f.svc.Complete, "Deliver": f.svc.Deliver,
	}

	for name, run := range actions {
		t.Run(name+" inexistente", func(t *testing.T) {
			_, err := run(ctx, uuid.New())
			wantErr(t, err, apperr.KindNotFound, domain.CodeWorkOrderNotFound)
		})
	}

	// Uma ordem RECEBIDA só aceita StartDiagnosis.
	for _, name := range []string{"SendQuote", "Approve", "Reject", "Complete", "Deliver"} {
		t.Run(name+" no status errado", func(t *testing.T) {
			_, err := actions[name](ctx, order.ID)
			wantErr(t, err, apperr.KindConflict, domain.CodeInvalidTransition)
		})
	}

	t.Run("conflito de gravação é propagado", func(t *testing.T) {
		f.world.repo.saveErr = apperr.Conflict(domain.CodeWorkOrderChanged, "alterada")
		defer func() { f.world.repo.saveErr = nil }()

		_, err := f.svc.StartDiagnosis(ctx, order.ID)
		wantErr(t, err, apperr.KindConflict, domain.CodeWorkOrderChanged)
	})
}

func TestApproveConsumesOnlyPartsWithOrderNumber(t *testing.T) {
	f := setup()
	ctx := context.Background()
	order, _ := f.svc.Create(ctx, f.vehicleID, "Revisão", ItemsInput{
		ServiceIDs: []uuid.UUID{f.oilChangeID},
		Parts:      []PartInput{{PartID: f.filterID, Quantity: 2}, {PartID: f.sparkID, Quantity: 4}},
	})
	_, _ = f.svc.StartDiagnosis(ctx, order.ID)
	_, _ = f.svc.SendQuote(ctx, order.ID)

	approved, err := f.svc.Approve(ctx, order.ID)

	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if approved.StartedAt == nil || !approved.StartedAt.Equal(now) {
		t.Errorf("início da execução não foi marcado: %+v", approved.StartedAt)
	}
	if f.world.stock[f.filterID] != 8 || f.world.stock[f.sparkID] != 6 {
		t.Errorf("estoque = filtro %d, vela %d; esperado 8 e 6", f.world.stock[f.filterID], f.world.stock[f.sparkID])
	}
	if len(f.world.consumed) != 2 {
		t.Fatalf("%d baixas, esperado 2 (o serviço não consome estoque)", len(f.world.consumed))
	}
	if want := f.filterID.String() + ":2:OS #1"; f.world.consumed[0] != want {
		t.Errorf("baixa = %q, esperado %q", f.world.consumed[0], want)
	}
}

// O ponto central da aprovação: sem saldo para QUALQUER peça, nada muda.
func TestApproveIsAtomicWhenStockIsInsufficient(t *testing.T) {
	f := setup()
	ctx := context.Background()
	f.world.stock[f.sparkID] = 1 // a segunda peça não tem saldo para 4 unidades
	order, _ := f.svc.Create(ctx, f.vehicleID, "Revisão", ItemsInput{
		Parts: []PartInput{{PartID: f.filterID, Quantity: 2}, {PartID: f.sparkID, Quantity: 4}},
	})
	_, _ = f.svc.StartDiagnosis(ctx, order.ID)
	_, _ = f.svc.SendQuote(ctx, order.ID)

	_, err := f.svc.Approve(ctx, order.ID)

	wantErr(t, err, apperr.KindConflict, codeInsufficientStock)
	if appErr, _ := apperr.As(err); appErr == nil || appErr.Message != "estoque insuficiente para a peça Vela" {
		t.Errorf("a mensagem deveria citar a peça: %v", err)
	}

	stored, _ := f.svc.Get(ctx, order.ID)
	if stored.Status != domain.StatusAwaitingApproval || stored.StartedAt != nil {
		t.Errorf("a ordem deveria continuar aguardando aprovação: %+v", stored)
	}
	if f.world.stock[f.filterID] != 10 || f.world.stock[f.sparkID] != 1 {
		t.Errorf("a baixa da primeira peça deveria ter sido desfeita: filtro %d, vela %d", f.world.stock[f.filterID], f.world.stock[f.sparkID])
	}
	if len(f.world.consumed) != 0 {
		t.Errorf("nenhuma baixa deveria sobrar: %v", f.world.consumed)
	}

	// Repondo o estoque, a mesma aprovação passa.
	f.world.stock[f.sparkID] = 4
	if _, err := f.svc.Approve(ctx, order.ID); err != nil {
		t.Errorf("com estoque, a aprovação deveria passar: %v", err)
	}
}

func TestApproveDoesNotTouchStockWhenTransitionIsInvalid(t *testing.T) {
	f := setup()
	order := f.order(t) // RECEIVED: aprovar não é permitido

	_, err := f.svc.Approve(context.Background(), order.ID)

	wantErr(t, err, apperr.KindConflict, domain.CodeInvalidTransition)
	if len(f.world.consumed) != 0 {
		t.Error("não deveria haver baixa de estoque")
	}

	_, err = f.svc.Approve(context.Background(), uuid.New())
	wantErr(t, err, apperr.KindNotFound, domain.CodeWorkOrderNotFound)
}

func TestApproveDoesNotConsumeStockWhenSaveConflicts(t *testing.T) {
	f := setup()
	order := f.awaitingApproval(t)
	f.world.repo.saveErr = apperr.Conflict(domain.CodeWorkOrderChanged, "alterada")

	_, err := f.svc.Approve(context.Background(), order.ID)

	wantErr(t, err, apperr.KindConflict, domain.CodeWorkOrderChanged)
	if len(f.world.consumed) != 0 || f.world.stock[f.filterID] != 10 {
		t.Error("com conflito de gravação, o estoque não pode ser consumido")
	}
}

func TestApprovePropagatesOtherStockErrors(t *testing.T) {
	f := setup()
	order := f.awaitingApproval(t)
	f.world.stockErr = errors.New("banco fora do ar")

	_, err := f.svc.Approve(context.Background(), order.ID)

	if err == nil || err.Error() != "banco fora do ar" {
		t.Errorf("erro = %v, esperado o erro original do estoque", err)
	}
	stored, _ := f.svc.Get(context.Background(), order.ID)
	if stored.Status != domain.StatusAwaitingApproval {
		t.Errorf("a ordem deveria ter voltado a aguardar aprovação: %s", stored.Status)
	}
}
