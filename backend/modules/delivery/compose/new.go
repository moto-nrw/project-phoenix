package compose

import (
	"context"
	"errors"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/uptrace/bun"
)

// ModuleDependencies are the enqueue and state side of Delivery, which the
// producers and Identity & Access need before the mail renderers exist.
type ModuleDependencies struct {
	DB      *bun.DB
	People  application.GuardianDisplayResolver
	Observe func(deliveryContract.Observation)
}

type Runtime application.Runtime
type Provider interface{ application.Provider }
type GuardianDisplayResolver interface {
	application.GuardianDisplayResolver
}

func NewWorker(runtime *Runtime, provider Provider) (*delivery.Worker, error) {
	return application.NewWorkerBinding((*application.Runtime)(runtime), provider)
}
func NewPushDispatcher(runtime *Runtime, provider Provider) *delivery.PushDispatcher {
	return application.NewPushDispatcher((*application.Runtime)(runtime), provider)
}
func (r *Runtime) EmailStatus(ctx context.Context, tenantID, id int64) (deliveryContract.State, bool, error) {
	return (*application.Runtime)(r).EmailStatus(ctx, tenantID, id)
}

// NewModule composes the enqueue facade and the e-mail state without the
// worker; NewWorker adds the worker once the mail renderers exist.
func NewModule(dependencies ModuleDependencies) (*Runtime, error) {
	if dependencies.DB == nil || dependencies.People == nil || dependencies.Observe == nil {
		return nil, errors.New("delivery compose: all dependencies are required")
	}
	raw := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	store := application.NewWorkerCommands(raw, NewTenantTransactions(dependencies.DB).WithinSystem)
	observe := application.NewObservationAdapter(dependencies.Observe)
	service := application.NewService(store, application.GuardianDirectoryAdapter{Resolver: dependencies.People}, observe)
	return &Runtime{Module: delivery.NewModule(application.ModuleEngine{Service: service, Store: store}), Store: store, Observe: observe}, nil
}
