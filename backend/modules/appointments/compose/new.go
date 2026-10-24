// Package compose wires the Appointments module over the shared tenant
// UnitOfWork and Bun retainedDatabase.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the Appointments service, which is the whole Owner surface.
func New(dependencies Dependencies) (appointments.Owner, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("appointments compose: all dependencies are required")
	}
	store := postgres.New(postgres.DatabaseBinding(dependencies.DB))
	return application.New(store, application.NewRetention(store), dependencies.Observe, withinTenant), nil
}

func withinTenant(ctx context.Context, command func(context.Context) error) error {
	return postgres.WithinTenantBinding(ctx, command, tenant.WithinTenant)
}
