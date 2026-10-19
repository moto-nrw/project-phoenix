package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// guardianPickupDatabase resolves the ambient transaction without requiring a
// tenant in context: the commands name their school, and the store holds a
// tenant transaction to that school.
func GuardianPickupDatabase(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		database, err := tenant.CurrentTransaction[bun.IDB](ctx)
		return database, tenant.FromContext(ctx), err
	}
}
