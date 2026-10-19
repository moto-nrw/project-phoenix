package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// newStore resolves every statement against the caller's ambient transaction.
func NewStore(db *bun.DB) *Store {
	scope := func(ctx context.Context) TenantScope {
		return ScopeFor(tenant.FromContext(ctx), tenant.IsAdminTx(ctx))
	}
	return New(func(ctx context.Context) (bun.IDB, error) {
		raw, ok := tenant.TransactionFromContext(ctx)
		return ResolveDatabase(raw, ok, scope(ctx))
	}, scope)
}
