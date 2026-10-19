package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// retentionDatabase is the transaction of the school in context; without one
// the retention refuses to run instead of falling back to a pool.
func RetentionDatabase(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("care plan postgres: tenant is required: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("care plan postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("care plan postgres: unsupported transaction %T", transaction)
	}
	return tx, tenantID.Int64(), nil
}
