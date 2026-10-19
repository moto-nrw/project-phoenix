package runtime

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Database resolves the ambient tenant transaction and the owner's pool.
type Database struct {
	db          *bun.DB
	transaction func(context.Context) (any, bool)
}

func NewDatabase(db *bun.DB, transaction func(context.Context) (any, bool)) Database {
	if db == nil {
		panic("school structure compose: database is required")
	}
	return Database{db: db, transaction: transaction}
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
