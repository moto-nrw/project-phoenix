// Package compose builds the Student Presence owner over the tenant runtime.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

func New(deps Dependencies) (*studentpresence.Module, error) {
	if deps.DB == nil || deps.Observe == nil {
		return nil, errors.New("student presence compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(deps.DB))
	return studentpresence.NewModule(engine{Service: application.New(store, transaction{}, deps.Observe)}), nil
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		value, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db, id.Int64(), nil
		}
		switch tx := value.(type) {
		case bun.Tx:
			return tx, id.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, id.Int64(), nil
			}
		}
		return nil, 0, fmt.Errorf("student presence: unsupported transaction %T", value)
	}
}

type transaction struct{}

func (transaction) Run(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

func (transaction) Require(ctx context.Context) error {
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("student presence: transaction is required")
	}
	return nil
}
