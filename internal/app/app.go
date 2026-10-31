// Package app monta a aplicação: liga cada contexto às suas dependências e
// registra as rotas. É o único lugar que conhece todos os contextos ao mesmo tempo.
package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	catalogapp "oficina-api/internal/catalog/application"
	cataloghttp "oficina-api/internal/catalog/http"
	cataloginfra "oficina-api/internal/catalog/infrastructure"
	identityapp "oficina-api/internal/identity/application"
	identityhttp "oficina-api/internal/identity/http"
	identityinfra "oficina-api/internal/identity/infrastructure"
	inventoryapp "oficina-api/internal/inventory/application"
	inventorydomain "oficina-api/internal/inventory/domain"
	inventoryhttp "oficina-api/internal/inventory/http"
	inventoryinfra "oficina-api/internal/inventory/infrastructure"
	"oficina-api/internal/platform/config"
	"oficina-api/internal/platform/database"
	"oficina-api/internal/platform/httpserver"
	registryapp "oficina-api/internal/registry/application"
	registryhttp "oficina-api/internal/registry/http"
	registryinfra "oficina-api/internal/registry/infrastructure"
	"oficina-api/internal/shared/clock"
	workorderapp "oficina-api/internal/workorder/application"
	workorderhttp "oficina-api/internal/workorder/http"
	workorderinfra "oficina-api/internal/workorder/infrastructure"
)

// NewHandler devolve o roteador completo da API. Também cria o administrador
// inicial, quando configurado e ainda não existir nenhum usuário.
func NewHandler(ctx context.Context, db *sql.DB, cfg config.Config, logger *slog.Logger, clk clock.Clock) (http.Handler, error) {
	auth, err := identityapp.NewAuthService(
		identityinfra.NewUserRepository(db),
		identityinfra.NewBcryptHasher(bcrypt.DefaultCost),
		identityinfra.NewJWTManager(cfg.JWTSecret, cfg.JWTTTL, clk),
		clk,
	)
	if err != nil {
		return nil, err
	}

	if cfg.AdminEmail != "" {
		created, err := auth.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword)
		if err != nil {
			return nil, err
		}
		if created {
			logger.Info("administrador inicial criado", "email", cfg.AdminEmail)
		}
	}

	customers := registryapp.NewCustomerService(registryinfra.NewCustomerRepository(db), clk)
	vehicles := registryapp.NewVehicleService(registryinfra.NewVehicleRepository(db), clk)
	services := catalogapp.NewServiceCatalog(cataloginfra.NewServiceRepository(db), clk)
	inventory := inventoryapp.NewInventoryService(inventoryinfra.NewPartRepository(db), clk)

	workOrders := workorderapp.NewWorkOrderService(
		workorderinfra.NewWorkOrderRepository(db),
		database.Transactor{DB: db},
		vehicleDirectory{vehicles},
		customerDirectory{customers},
		itemCatalog{services: services, parts: inventory},
		stockConsumer{inventory},
		clk,
	)

	identityHandler := identityhttp.NewHandler(auth, logger)
	protect := identityHandler.RequireAuth

	return httpserver.NewHandler(db, logger,
		identityHandler.Register,
		func(mux *http.ServeMux) { registryhttp.NewCustomerHandler(customers, logger).Register(mux, protect) },
		func(mux *http.ServeMux) { registryhttp.NewVehicleHandler(vehicles, logger).Register(mux, protect) },
		func(mux *http.ServeMux) { cataloghttp.NewServiceHandler(services, logger).Register(mux, protect) },
		func(mux *http.ServeMux) { inventoryhttp.NewPartHandler(inventory, logger).Register(mux, protect) },
		func(mux *http.ServeMux) { workorderhttp.NewHandler(workOrders, logger).Register(mux, protect) },
		workorderhttp.NewPublicHandler(workOrders, logger).Register,
	), nil
}

// Os tipos abaixo adaptam os contextos de cadastro, catálogo e estoque às
// interfaces que a ordem de serviço declarou, sem que ela os importe.

type vehicleDirectory struct {
	vehicles *registryapp.VehicleService
}

func (d vehicleDirectory) CustomerOf(ctx context.Context, vehicleID uuid.UUID) (uuid.UUID, error) {
	vehicle, err := d.vehicles.Get(ctx, vehicleID)
	return vehicle.CustomerID, err
}

type customerDirectory struct {
	customers *registryapp.CustomerService
}

func (d customerDirectory) CustomerIDByDocument(ctx context.Context, rawDocument string) (uuid.UUID, error) {
	customer, err := d.customers.FindByDocument(ctx, rawDocument)
	return customer.ID, err
}

type itemCatalog struct {
	services *catalogapp.ServiceCatalog
	parts    *inventoryapp.InventoryService
}

func (c itemCatalog) Service(ctx context.Context, id uuid.UUID) (workorderapp.ItemInfo, error) {
	service, err := c.services.Get(ctx, id)
	return workorderapp.ItemInfo{Name: service.Name, UnitPrice: service.Price}, err
}

func (c itemCatalog) Part(ctx context.Context, id uuid.UUID) (workorderapp.ItemInfo, error) {
	part, err := c.parts.Get(ctx, id)
	return workorderapp.ItemInfo{Name: part.Name, UnitPrice: part.UnitPrice}, err
}

type stockConsumer struct {
	inventory *inventoryapp.InventoryService
}

func (s stockConsumer) Consume(ctx context.Context, partID uuid.UUID, quantity int, reason string) error {
	_, _, err := s.inventory.RecordMovement(ctx, partID, inventorydomain.MovementData{
		Type: string(inventorydomain.MovementOut), Quantity: quantity, Reason: reason,
	})
	return err
}
