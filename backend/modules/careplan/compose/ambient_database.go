package compose

import (
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantAmbientDatabase resolves the transaction the tenant unit of work
// attached to the caller's context, failing outside a unit of work. The
// unit of work attaches its repository transaction and the tenant
// transaction together, so this joins the same transaction the retained
// repositories resolve. A transaction it cannot join fails instead of
// running the statement outside the caller's unit of work.
func TenantAmbientDatabase(db *bun.DB) AmbientDatabase {
	if db == nil {
		panic("care plan compose: database is required")
	}
	return tenant.CurrentTransaction[bun.IDB]
}
