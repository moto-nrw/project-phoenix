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
}

type Database struct {
	ContextRuntime
	db          *bun.DB
	transaction func(context.Context) (any, bool)
}

func NewDatabase(contextRuntime ContextRuntime, db *bun.DB, transaction func(context.Context) (any, bool)) Database {
	if db == nil {
		panic("school structure compose: database is required")
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
	panic(fmt.Sprintf("school structure compose: unsupported transaction type %T", transaction))
}
