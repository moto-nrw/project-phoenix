package postgres

import (
	context "context"
	fmt "fmt"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

func DatabaseRuntime(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("facilities postgres: tenant is required: %w", err)
		}
		Transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			return nil, 0, fmt.Errorf("facilities postgres: transaction is required")
		}
		switch tx := Transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
			return nil, 0, fmt.Errorf("facilities postgres: transaction is required")
		default:
			return nil, 0, fmt.Errorf("facilities postgres: unsupported transaction %T", Transaction)
		}
	}
}
