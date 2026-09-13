package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewOfferingBookings binds the owner to the ambient UnitOfWork, never to an
// unscoped pool. It can therefore participate in atomic Enrollment workflows.
func NewOfferingBookings() *careplan.OfferingBookings {
	return newOfferingBookings(observeOfferingBookings())
}

func newOfferingBookings(observe func(careplan.OfferingBookingObservation)) *careplan.OfferingBookings {
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
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
	})
	return careplan.NewOfferingBookings(store, tenant.NewTransactionRunner(), observe)
}
