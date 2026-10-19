// Package compose wires the Appointments module over the shared tenant
// UnitOfWork and Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

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

func New(dependencies Dependencies) (*appointments.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("appointments compose: all dependencies are required")
	}
	store := postgres.New(database(dependencies.DB))
	service := application.New(store, func(observation Observation) {
		observation.Err = application.MapPublicError(observation.Err)
		dependencies.Observe(observation)
	}, withinTenant)
	return application.NewPublicModule(service), nil
}

func database(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, fmt.Errorf("appointments postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return nil, 0, fmt.Errorf("appointments postgres: transaction is required")
		default:
			return nil, 0, fmt.Errorf("appointments postgres: unsupported transaction %T", transaction)
		}
	}
}

func withinTenant(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return errors.New("appointments: tenant is required for commands")
	}
	return tenant.WithinTenant(ctx, tenantID, command)
}
