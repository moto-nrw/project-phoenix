package postgres

import (
	context "context"
	fmt "fmt"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

func DatabaseBinding(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, fmt.Errorf("appointments postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return nil, 0, fmt.Errorf("appointments postgres: transaction is required")
		default:
			return nil, 0, fmt.Errorf("appointments postgres: unsupported transaction %T", transaction)
		}
	}
}
