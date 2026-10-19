package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// newStore resolves every statement against the caller's ambient transaction.
func newStore(db *bun.DB) *postgres.Store {
	scope := func(ctx context.Context) postgres.TenantScope {
		id, err := tenant.TenantFromContext(ctx)
		admin := tenant.IsAdminTx(ctx)
		if err != nil {
			return postgres.TenantScope{AdminTransaction: admin, AcrossSchools: admin}
		}
		return postgres.TenantScope{TenantID: id.Int64(), AdminTransaction: admin}
	}
	return postgres.New(func(ctx context.Context) (bun.IDB, error) {
		if err := scope(ctx).Validate(); err != nil {
			return nil, err
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("identity access postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return *tx, nil
			}
			return nil, errors.New("identity access postgres: transaction is required")
		default:
			return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", transaction)
		}
	}, scope)
}
