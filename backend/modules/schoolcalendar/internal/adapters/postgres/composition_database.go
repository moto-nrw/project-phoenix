package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewCompositionDatabase() Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		id, scopeErr := tenant.TenantFromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)
		return ResolveDatabase(raw, ok, id.Int64(), scopeErr)
	}
}
