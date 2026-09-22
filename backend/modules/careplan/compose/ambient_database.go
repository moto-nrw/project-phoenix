package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantAmbientDatabase resolves the transaction the tenant unit of work
// attached to the caller's context, and falls back to db outside one. The
// unit of work attaches its repository transaction and the tenant
// transaction together, so this joins the same transaction the retained
// repositories resolve. A transaction it cannot join panics instead of
// running the statement outside the caller's unit of work.
func TenantAmbientDatabase(db *bun.DB) AmbientDatabase {
	if db == nil {
		panic("care plan compose: database is required")
	}
	return func(ctx context.Context) bun.IDB {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx
		case *bun.Tx:
			if tx != nil {
				return tx
			}
		}
		panic(fmt.Sprintf("care plan compose: unsupported transaction type %T", transaction))
	}
}
