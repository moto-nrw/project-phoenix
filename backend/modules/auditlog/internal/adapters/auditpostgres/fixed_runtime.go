package audit

import (
	"context"

	"github.com/uptrace/bun"
)

// NewRuntime binds the caller-owned tenant resolver to the ambient transaction.
func NewRuntime(db *bun.DB, tenantID func(context.Context) int64) Runtime {
	if db == nil || tenantID == nil {
		panic("audit repository: database and tenant resolver are required")
	}
	ambient := NewAmbientRuntime(db)
	return func(ctx context.Context) (bun.IDB, int64) { tx, _ := ambient(ctx); return tx, tenantID(ctx) }
}
