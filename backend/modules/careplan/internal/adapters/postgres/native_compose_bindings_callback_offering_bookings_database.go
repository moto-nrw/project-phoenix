package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewOfferingBookingsDatabase() func(ctx context.Context) (bun.IDB, int64, error) {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, fmt.Errorf("care offering bookings: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("care offering bookings: unsupported transaction %T", transaction)
		}
		return tx, id.Int64(), nil
	}
}
