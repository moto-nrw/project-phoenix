package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewDatabase() func(ctx context.Context) (bun.IDB, TenantScope, error) {
	return func(ctx context.Context) (bun.IDB, TenantScope, error) {
		id, scopeErr := tenant.TenantFromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)
		return ResolveDatabase(raw, ok, id.Int64(), scopeErr, tenant.IsAdminTx(ctx))
	}
}
