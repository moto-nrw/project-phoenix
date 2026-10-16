package base

import (
	"context"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

func tenantIDFromContext(ctx context.Context) int64 {
	return modelBase.RepositoryTenantID(ctx)
}

// GetDB returns the transaction from context if available, otherwise the base DB.
// The transaction-runtime adapter installs either bun.Tx or *bun.Tx through
// the dependency-free context protocol above.
func GetDB(ctx context.Context, db bun.IDB) bun.IDB {
	raw, ok := modelBase.RepositoryTransaction(ctx)
	if !ok {
		return db
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx
	case *bun.Tx:
		if tx != nil {
			return tx
		}
	}
	panic(fmt.Sprintf("base repository: unsupported transaction type %T", raw))
}
