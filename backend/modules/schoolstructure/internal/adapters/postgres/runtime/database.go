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
	transaction func(context.Context) (any, bool)
}

func NewDatabase(contextRuntime ContextRuntime, db *bun.DB, transaction func(context.Context) (any, bool)) Database {
	if db == nil {
		panic("school structure compose: database is required")
	}
	return Database{ContextRuntime: contextRuntime, transaction: transaction}
}

// DB resolves only the caller's existing unit of work.
func (r Database) DB(ctx context.Context) (bun.IDB, error) {
	if r.TenantID(ctx) <= 0 {
		return nil, fmt.Errorf("school structure repository: tenant is required")
	}
	raw, ok := r.transaction(ctx)
	if !ok {
		return nil, fmt.Errorf("school structure repository: transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("school structure repository: unsupported transaction %T", raw)
}
