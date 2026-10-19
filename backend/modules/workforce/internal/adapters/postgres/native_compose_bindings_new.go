package postgres

import (
	context "context"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

func DatabaseRuntime(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, TenantScope, error) {
		id, scopeErr := tenant.TenantFromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)

		return ResolveDatabase(raw, ok, id.Int64(), scopeErr, tenant.IsAdminTx(ctx))
	}

}
