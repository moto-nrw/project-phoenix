package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewCleanupRetention composes the staff cleanup-job retention of the nightly
// cleanup (#4046). It runs on the tenant transaction of the school in
// context and needs no object store.
func NewCleanupRetention() workforce.CleanupRetention {
	return application.NewCleanupRetention(postgres.New(retentionDatabase))
}

// retentionDatabase is the transaction of the school in context; without one
// the retention refuses to run instead of falling back to a pool.
func retentionDatabase(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("workforce postgres: tenant is required: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("workforce postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("workforce postgres: unsupported transaction %T", transaction)
	}
	return tx, tenantID.Int64(), nil
}
