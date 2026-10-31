// Package application do contexto workorder: casos de uso da ordem de serviço.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
	"oficina-api/internal/shared/money"
	"oficina-api/internal/workorder/domain"
)

const codeInsufficientStock = "INSUFFICIENT_STOCK"

// ItemInfo é o que a ordem de serviço guarda de um serviço ou peça: nome e preço do momento.
type ItemInfo struct {
	Name      string
	UnitPrice money.Money
}

// Os contextos de cadastro, catálogo e estoque entram por estas interfaces
// pequenas; quem as implementa é a montagem da aplicação (internal/app).

type VehicleDirectory interface {
	// CustomerOf devolve o dono do veículo, ou um erro not_found se ele não existe.
	CustomerOf(ctx context.Context, vehicleID uuid.UUID) (uuid.UUID, error)
}

type ItemCatalog interface {
	Service(ctx context.Context, id uuid.UUID) (ItemInfo, error)
	Part(ctx context.Context, id uuid.UUID) (ItemInfo, error)
}

type StockConsumer interface {
	// Consume dá baixa no estoque; participa da transação que estiver no contexto.
	Consume(ctx context.Context, partID uuid.UUID, quantity int, reason string) error
}

type Transactor interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

type PartInput struct {
	PartID   uuid.UUID
	Quantity int
}

type ItemsInput struct {
	ServiceIDs []uuid.UUID
	Parts      []PartInput
}

type WorkOrderService struct {
	repo     domain.Repository
	tx       Transactor
	vehicles VehicleDirectory
	catalog  ItemCatalog
	stock    StockConsumer
	clock    clock.Clock
}

func NewWorkOrderService(repo domain.Repository, tx Transactor, vehicles VehicleDirectory, catalog ItemCatalog, stock StockConsumer, clk clock.Clock) *WorkOrderService {
	return &WorkOrderService{repo: repo, tx: tx, vehicles: vehicles, catalog: catalog, stock: stock, clock: clk}
}

// Create abre a ordem de serviço como RECEIVED. O cliente é o dono do veículo.
func (s *WorkOrderService) Create(ctx context.Context, vehicleID uuid.UUID, description string, in ItemsInput) (domain.WorkOrder, error) {
	customerID, err := s.vehicles.CustomerOf(ctx, vehicleID)
	if err != nil {
		return domain.WorkOrder{}, asReference(err)
	}

	items, err := s.buildItems(ctx, in)
	if err != nil {
		return domain.WorkOrder{}, err
	}

	order, err := domain.NewWorkOrder(uuid.New(), customerID, vehicleID, description, items, s.clock.Now())
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return s.repo.Create(ctx, order)
}

func (s *WorkOrderService) Get(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return s.repo.FindByID(ctx, id)
}

// List filtra por status quando rawStatus não é vazio.
func (s *WorkOrderService) List(ctx context.Context, rawStatus string, limit, offset int) ([]domain.WorkOrder, error) {
	var status domain.Status
	if rawStatus != "" {
		parsed, err := domain.ParseStatus(rawStatus)
		if err != nil {
			return nil, err
		}
		status = parsed
	}
	return s.repo.List(ctx, status, limit, offset)
}

// ReplaceItems troca os serviços e peças; só é possível antes de o orçamento ser enviado.
func (s *WorkOrderService) ReplaceItems(ctx context.Context, id uuid.UUID, in ItemsInput) (domain.WorkOrder, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.WorkOrder{}, err
	}

	items, err := s.buildItems(ctx, in)
	if err != nil {
		return domain.WorkOrder{}, err
	}

	next, err := current.ReplaceItems(items, s.clock.Now())
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return s.save(ctx, current, next)
}

func (s *WorkOrderService) StartDiagnosis(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return s.transition(ctx, id, domain.WorkOrder.StartDiagnosis)
}

func (s *WorkOrderService) SendQuote(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return s.transition(ctx, id, domain.WorkOrder.SendQuote)
}

func (s *WorkOrderService) Reject(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return s.transition(ctx, id, domain.WorkOrder.Reject)
}

func (s *WorkOrderService) Complete(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return s.transition(ctx, id, domain.WorkOrder.Complete)
}

func (s *WorkOrderService) Deliver(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	return s.transition(ctx, id, domain.WorkOrder.Deliver)
}

// Approve inicia a execução e dá baixa no estoque das peças. Tudo acontece numa
// única transação: se qualquer peça não tiver saldo, a aprovação inteira é
// desfeita e a ordem de serviço continua aguardando aprovação.
func (s *WorkOrderService) Approve(ctx context.Context, id uuid.UUID) (domain.WorkOrder, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.WorkOrder{}, err
	}

	next, err := current.Approve(s.clock.Now())
	if err != nil {
		return domain.WorkOrder{}, err
	}

	err = s.tx.Run(ctx, func(ctx context.Context) error {
		// Gravar o status primeiro trava a linha: duas aprovações simultâneas
		// não chegam a consumir estoque duas vezes.
		if err := s.repo.Save(ctx, next, current.Status); err != nil {
			return err
		}
		return s.consumeParts(ctx, next)
	})
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return next, nil
}

func (s *WorkOrderService) consumeParts(ctx context.Context, order domain.WorkOrder) error {
	reason := fmt.Sprintf("OS #%d", order.Number)

	for _, item := range order.Items {
		if item.Kind != domain.ItemPart {
			continue
		}

		err := s.stock.Consume(ctx, item.RefID, item.Quantity, reason)
		if appErr, ok := apperr.As(err); ok && appErr.Code == codeInsufficientStock {
			return apperr.Conflict(codeInsufficientStock, fmt.Sprintf("estoque insuficiente para a peça %s", item.Name))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *WorkOrderService) transition(ctx context.Context, id uuid.UUID, apply func(domain.WorkOrder, time.Time) (domain.WorkOrder, error)) (domain.WorkOrder, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.WorkOrder{}, err
	}

	next, err := apply(current, s.clock.Now())
	if err != nil {
		return domain.WorkOrder{}, err
	}
	return s.save(ctx, current, next)
}

func (s *WorkOrderService) save(ctx context.Context, current, next domain.WorkOrder) (domain.WorkOrder, error) {
	if err := s.repo.Save(ctx, next, current.Status); err != nil {
		return domain.WorkOrder{}, err
	}
	return next, nil
}

// buildItems consulta nome e preço atuais no catálogo e no estoque.
func (s *WorkOrderService) buildItems(ctx context.Context, in ItemsInput) ([]domain.Item, error) {
	items := make([]domain.Item, 0, len(in.ServiceIDs)+len(in.Parts))

	for _, id := range in.ServiceIDs {
		info, err := s.catalog.Service(ctx, id)
		if err != nil {
			return nil, asReference(err)
		}
		items = append(items, domain.Item{Kind: domain.ItemService, RefID: id, Name: info.Name, UnitPrice: info.UnitPrice, Quantity: 1})
	}

	for _, p := range in.Parts {
		info, err := s.catalog.Part(ctx, p.PartID)
		if err != nil {
			return nil, asReference(err)
		}
		items = append(items, domain.Item{Kind: domain.ItemPart, RefID: p.PartID, Name: info.Name, UnitPrice: info.UnitPrice, Quantity: p.Quantity})
	}
	return items, nil
}

// asReference transforma "não encontrado" em erro de validação: quando o item
// citado no pedido não existe, o problema é o pedido, e não a rota.
func asReference(err error) error {
	if appErr, ok := apperr.As(err); ok && appErr.Kind == apperr.KindNotFound {
		return apperr.Validation(appErr.Code, appErr.Message)
	}
	return err
}
