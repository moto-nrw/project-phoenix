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
	InTenantTx(context.Context, func(context.Context) error) error
	MarkRollback(context.Context)
	RequireTenantID(context.Context) (int64, error)
	InTransaction(context.Context) bool
	WithSavepoint(context.Context, func(context.Context) error) error
	AcrossSchools(context.Context) bool
}

type Database struct {
	ContextRuntime
	db          *bun.DB
	transaction func(context.Context) (any, bool)
}

func NewDatabase(contextRuntime ContextRuntime, db *bun.DB, transaction func(context.Context) (any, bool)) Database {
	if db == nil {
		panic("people directory compose: database is required")
	}
	return Database{ContextRuntime: contextRuntime, db: db, transaction: transaction}
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
