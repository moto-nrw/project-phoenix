package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// newStore resolves every statement against the caller's ambient transaction.
func newStore(db *bun.DB) *postgres.Store {
	scope := func(ctx context.Context) postgres.TenantScope {
		return postgres.ScopeFor(tenant.FromContext(ctx), tenant.IsAdminTx(ctx))
	}
	return postgres.New(func(ctx context.Context) (bun.IDB, error) {
		raw, ok := tenant.TransactionFromContext(ctx)
		return postgres.ResolveDatabase(raw, ok, scope(ctx))
	}, scope)
}
