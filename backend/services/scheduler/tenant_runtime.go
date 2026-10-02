package scheduler

import (
	"context"
	"errors"
	"time"
)

// TenantRuntime is the transaction runtime the Worker runs its jobs in
// (#2746). The root binds it to the tenant runtime through
// modules/schedulerruntime/jobruntime; the scheduler owns cadence, batching
// and observations, the runtime owns tenant scoping and transactions.
type TenantRuntime interface {
	// Attach binds the runtime to ctx, so the owners a job calls open their
	// transactions in it. observe, when set, receives every unit-of-work
	// event: its kind, its result, its duration and its retries.
	Attach(ctx context.Context, observe func(kind, result string, duration time.Duration, retries int)) context.Context
	// WithinTenant runs fn in the transaction of one school.
	WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
	// WithinTenantRetry runs a retry-safe fn in the transaction of one school
	// and replays it after a deadlock or serialization failure.
	WithinTenantRetry(ctx context.Context, tenantID int64, fn func(context.Context) error) error
	// WithinAdmin runs fn in the cross-tenant administrative transaction.
	WithinAdmin(ctx context.Context, fn func(context.Context) error) error
	// WithCommitGuard makes every transaction opened under ctx call guard as
	// its last statement before commit; a guard error rolls it back.
	WithCommitGuard(ctx context.Context, guard func(context.Context) error) context.Context
	// RegisterAfterCommit runs fn after the surrounding transaction commits,
	// and at once outside a transaction.
	RegisterAfterCommit(ctx context.Context, fn func())
}

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
