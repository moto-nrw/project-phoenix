package postgres

import (
	context "context"
	errors "errors"
	fmt "fmt"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

// RetentionDatabase is the Transaction of the school in context; without one
// the retention refuses to run instead of falling back to a pool.
func RetentionDatabase(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("file storage postgres: tenant is required: %w", err)
	}
	Transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("file storage postgres: transaction is required")
	}
	tx, ok := Transaction.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("file storage postgres: unsupported transaction %T", Transaction)
	}
	return tx, tenantID.Int64(), nil
}
