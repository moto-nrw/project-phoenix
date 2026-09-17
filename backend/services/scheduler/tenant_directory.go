package scheduler

import "context"

// TenantDirectory lists the schools the worker iterates. Organisation &
// Tenancy owns the school rows; the root binds this port to its capability.
// Both lists are ascending by school name, the order the tenant batches have
// always used.
type TenantDirectory interface {
	// ListActiveTenantIDs returns every active, non-deleted school.
	ListActiveTenantIDs(ctx context.Context) ([]int64, error)
	// ListNonDeletedTenantIDs returns every non-deleted school, active or not.
	ListNonDeletedTenantIDs(ctx context.Context) ([]int64, error)
}
