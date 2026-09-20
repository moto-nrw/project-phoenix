package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// newStore resolves every statement against the caller's ambient transaction.
func newStore(db *bun.DB) *postgres.Store {
	scope := func(ctx context.Context) postgres.TenantScope {
		return postgres.TenantScope{TenantID: tenant.FromContext(ctx), AdminTransaction: tenant.IsAdminTx(ctx)}
	}
	return postgres.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return *tx, nil
			}
			return db, nil
		default:
			return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", transaction)
		}
	}, scope)
}
