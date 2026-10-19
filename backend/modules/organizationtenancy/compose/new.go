package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

func New(dependencies Dependencies) (*organizationtenancy.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("organization tenancy compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewDatabase())
	service := application.New(store, transaction{}, application.NewObserver(dependencies.Observe))
	return organizationtenancy.NewModule(application.NewEngine(service)), nil
}

type transaction struct{}

func (transaction) RunAdmin(ctx context.Context, callback func(context.Context) error) error {
	return postgres.TransactionRunAdminBinding(ctx, callback, tenant.WithinAdmin)
}

func (transaction) RunTenant(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	return postgres.TransactionRunTenantBinding(ctx, tenantID, callback, tenant.WithinTenant)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	return postgres.TransactionRunReadBinding(ctx, callback, tenant.WithinAdmin)
}
