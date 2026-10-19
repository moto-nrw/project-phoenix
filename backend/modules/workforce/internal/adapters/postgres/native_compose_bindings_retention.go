package postgres

import (
	context "context"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

// retentionDatabase is the transaction of the school in context; without one
// the retention refuses to run instead of falling back to a pool.
func RetentionDatabase(ctx context.Context) (bun.IDB, TenantScope, error) {
	id, err := tenant.TenantFromContext(ctx)
	raw, ok := tenant.TransactionFromContext(ctx)
	return ResolveDatabase(raw, ok, id.Int64(), err, false)
}
