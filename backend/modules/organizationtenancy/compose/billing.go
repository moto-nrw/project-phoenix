package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// BillingCounts are the owners' live figures the billing report captures:
// School Membership's active students and Device Fleet's active terminals.
type BillingCounts = ports.BillingCounts

// BillingDependencies are the collaborators of the billing report (#2791).
// Counts is required; Now defaults to time.Now.
type BillingDependencies struct {
	Counts BillingCounts
	Now    func() time.Time
	Logger *slog.Logger
}

// NewBilling composes the operator billing report over the ambient
// administrative transaction.
func NewBilling(dependencies BillingDependencies) (organizationtenancy.BillingReport, error) {
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	// The key date and the capture hour are read on the school calendar's
	// clock; a missing zone database fails the composition, not a capture.
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, fmt.Errorf("organization tenancy billing: load Europe/Berlin: %w", err)
	}
	store := postgres.NewBillingStore(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("organization tenancy billing: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("organization tenancy billing: unsupported transaction %T", transaction)
		}
		return tx, nil
	})
	billing, err := application.NewBilling(application.BillingDependencies{
		Transaction: transaction{}, Store: store, Counts: dependencies.Counts,
		Clock: ports.BillingClock(now), Location: berlin, Logger: dependencies.Logger,
	})
	if err != nil {
		return nil, err
	}
	return billing, nil
}
