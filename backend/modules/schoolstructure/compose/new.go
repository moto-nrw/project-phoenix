package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres"
	application "github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the School Structure module. Reads run on the caller's
// ambient tenant transaction when one exists (tenant middleware, scheduler
// loops) and otherwise on the shared connection, exactly like the legacy
// repositories' base.GetDB resolution, so RLS visibility is unchanged.
func New(dependencies Dependencies) (*schoolstructure.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school structure compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return dependencies.DB, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return tx, nil
			}
			return dependencies.DB, nil
		default:
			return nil, fmt.Errorf("school structure postgres: unsupported transaction %T", transaction)
		}
	})
	service := application.New(store, func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	})
	return schoolstructure.NewModule(application.NewEngine(service), application.NewTransitionRetention(store)), nil
}
