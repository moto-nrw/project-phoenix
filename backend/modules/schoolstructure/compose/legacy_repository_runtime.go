package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LegacyRepositoryRuntime binds the retained School Structure repositories
// (database/repositories/education) and services (services/education) to the
// shared tenant runtime, so those packages read neither the tenant context nor
// the transaction protocol themselves (#2742).
type LegacyRepositoryRuntime struct{ db *bun.DB }

// NewLegacyRepositoryRuntime returns the runtime over db, the pool used
// outside a tenant transaction.
func NewLegacyRepositoryRuntime(db *bun.DB) LegacyRepositoryRuntime {
	if db == nil {
		panic("school structure compose: database is required")
	}
	return LegacyRepositoryRuntime{db: db}
}

// DB is the ambient tenant transaction, or the pool outside one.
func (r LegacyRepositoryRuntime) DB(ctx context.Context) bun.IDB {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return r.db
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx
	case *bun.Tx:
		if tx != nil {
			return tx
		}
	}
	panic(fmt.Sprintf("school structure compose: unsupported transaction type %T", transaction))
}

// TenantID is the school the caller acts for, 0 when none.
func (LegacyRepositoryRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// RunInTx joins the ambient tenant transaction or opens one for the
// context's school. The retained School Structure services run their writes
// through it.
func (LegacyRepositoryRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// MarkRollback asks the request's tenant transaction to roll back.
func (LegacyRepositoryRuntime) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }
