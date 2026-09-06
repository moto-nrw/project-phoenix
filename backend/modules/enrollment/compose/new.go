package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// New binds Enrollment to the ambient transaction. It never falls back to an
// unscoped pool when a caller forgets to establish its transaction.
func New() *enrollment.Module {
	return enrollment.NewModule(postgres.NewStore(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("enrollment postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("enrollment postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	}, func(ctx context.Context) (int64, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return 0, err
		}
		return id.Int64(), nil
	}), tenant.NewTransactionRunner())
}
