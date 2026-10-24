package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// timetableAdvisoryLocks takes the planner's transaction-scoped advisory
// locks through the tenant runtime's unit of work; they release at commit or
// rollback.
type timetableAdvisoryLocks struct{}

func (timetableAdvisoryLocks) AcquireXactLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}
