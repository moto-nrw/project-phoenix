package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// OperatorDatabase supplies the existing administrative runtime to root-bound projections.
func OperatorDatabase() func(context.Context) (bun.IDB, error) {
	return func(ctx context.Context) (bun.IDB, error) {
		value, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("organization tenancy operator dashboard: transaction is required")
		}
		tx, ok := value.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("organization tenancy operator dashboard: unsupported transaction %T", value)
		}
		return tx, nil
	}
}
