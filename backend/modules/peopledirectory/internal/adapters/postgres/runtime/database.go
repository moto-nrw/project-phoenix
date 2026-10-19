package runtime

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Database resolves the ambient tenant transaction and the owner's pool.
// ContextRuntime supplies the ambient context operations without a driver dependency.
type ContextRuntime interface {
	TenantID(context.Context) int64
	RunInTx(context.Context, func(context.Context) error) error
	MarkRollback(context.Context)
	RequireTenantID(context.Context) (int64, error)
	InTransaction(context.Context) bool
	WithSavepoint(context.Context, func(context.Context) error) error
	WithinCurrentTenant(context.Context, func(context.Context) error) error
}

type Database struct {
	ContextRuntime
	db          *bun.DB
	transaction func(context.Context) (any, bool)
	tenantID    func(context.Context) int64
	inTenantTx  func(context.Context, *bun.DB, int64, func(context.Context, bun.Tx) error) error
}

func NewDatabase(contextRuntime ContextRuntime, db *bun.DB, transaction func(context.Context) (any, bool), tenantID func(context.Context) int64, inTenantTx func(context.Context, *bun.DB, int64, func(context.Context, bun.Tx) error) error) Database {
	if db == nil {
		panic("people directory compose: database is required")
	}
	return Database{ContextRuntime: contextRuntime, db: db, transaction: transaction, tenantID: tenantID, inTenantTx: inTenantTx}
}

// DB is the ambient tenant transaction, or the pool outside one.
func (r Database) DB(ctx context.Context) bun.IDB {
	transaction, ok := r.transaction(ctx)
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
	panic(fmt.Sprintf("people directory compose: unsupported transaction type %T", transaction))
}

// InTenantTx runs fn in the tenant transaction of the context's school,
// joining the request transaction when one is open, so a failure rolls the
// whole request back.
func (r Database) InTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return r.inTenantTx(ctx, r.db, r.tenantID(ctx), func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}
