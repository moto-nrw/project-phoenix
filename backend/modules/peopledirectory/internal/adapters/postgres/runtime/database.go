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
	transaction func(context.Context) (any, bool)
}

func NewDatabase(contextRuntime ContextRuntime, db *bun.DB, transaction func(context.Context) (any, bool)) Database {
	if db == nil {
		panic("people directory compose: database is required")
	}
	return Database{ContextRuntime: contextRuntime, transaction: transaction}
}

// DB resolves only the caller's existing unit of work.
func (r Database) DB(ctx context.Context) (bun.IDB, error) {
	if _, err := r.RequireTenantID(ctx); err != nil && !r.AcrossSchools(ctx) {
		return nil, err
	}
	raw, ok := r.transaction(ctx)
	if !ok {
		return nil, fmt.Errorf("people directory repository: transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("people directory repository: unsupported transaction %T", raw)
}
