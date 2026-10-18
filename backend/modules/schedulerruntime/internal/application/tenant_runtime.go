package scheduler

import (
	"context"
	"errors"
)

// The unit-of-work event kinds the tenant batches count.
const (
	unitOfWorkTransaction = "transaction"
	unitOfWorkPoolWait    = "pool_wait"
)

var (
	errTenantRuntimeRequired = errors.New("tenant: runtime is required")
	errInvalidTenantID       = errors.New("tenant ID must be positive")
)

// afterCommit runs fn once the surrounding tenant transaction commits.
// Without a runtime there is no transaction to wait for.
func (s *Scheduler) afterCommit(ctx context.Context, fn func()) {
	if s.tenantRuntime == nil {
		fn()
		return
	}
	s.tenantRuntime.RegisterAfterCommit(ctx, fn)
}
