package audit

import (
	"context"

	"github.com/uptrace/bun"
)

// NewRuntime adapts a root database plus a caller-owned tenant resolver. It is
// intended for read-only command roots and focused adapter tests; production
// append commands use an ambient-transaction resolver.
func NewRuntime(db *bun.DB, tenantID func(context.Context) int64) Runtime {
	if db == nil || tenantID == nil {
		panic("audit repository: database and tenant resolver are required")
	}
	return func(ctx context.Context) (bun.IDB, int64) { return db, tenantID(ctx) }
}
