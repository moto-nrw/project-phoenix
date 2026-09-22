package scheduler

import (
	"context"
	"time"
)

// TenantDirectory lists the schools the worker iterates. Organisation &
// Tenancy owns the school rows; the root binds this port to its capability.
// Both lists are ascending by school name, the order the tenant batches have
// always used.
type TenantDirectory interface {
	// ListActiveTenantIDs returns every active, non-deleted school.
	ListActiveTenantIDs(ctx context.Context) ([]int64, error)
	// ListNonDeletedTenantIDs returns every non-deleted school, active or not.
	ListNonDeletedTenantIDs(ctx context.Context) ([]int64, error)
	// RecordDueBillingKeyDates captures the billing key-date counts of every
	// school once the month's key date is due (#2791). It is idempotent,
	// opens its own administrative transaction and returns the rows written.
	RecordDueBillingKeyDates(ctx context.Context, now time.Time) (int, error)
}
