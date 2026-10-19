package postgres

import (
	context "context"
	errors "errors"

	devicefleet "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

// DatabaseRuntime requires an ambient school or explicit administrative unit.
// Pre-school authentication and public-token discovery use named capabilities.
func DatabaseRuntime(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, TenantScope, error) {
		id, scopeErr := tenant.TenantFromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)
		if scopeErr != nil && !tenant.IsAdminTx(ctx) {
			return nil, TenantScope{}, errors.Join(devicefleet.ErrTenantRequired, scopeErr)
		}
		return ResolveDatabase(raw, ok, id.Int64(), scopeErr, tenant.IsAdminTx(ctx))
	}

}
