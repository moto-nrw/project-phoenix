// Package jobruntime binds the Worker's jobs to the tenant runtime (#2746).
// The scheduler names its transactions through its own TenantRuntime port in
// plain values; this adapter maps them onto the tenant package, so the
// scheduler imports neither the runtime nor its event vocabulary.
package jobruntime

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// Runtime runs the Worker's units of work in the composed tenant runtime.
type Runtime struct {
	unitOfWork tenant.UnitOfWork
}

// New binds a composed tenant runtime. A runtime without transaction
// functions is a wiring error and fails here instead of on the first tick.
func New(unitOfWork tenant.UnitOfWork) (Runtime, error) {
	if unitOfWork.IsZero() {
		return Runtime{}, tenant.ErrRuntimeRequired
	}
	return Runtime{unitOfWork: unitOfWork}, nil
}

// Attach binds the runtime to ctx, so the owners a job calls open their
// transactions in it. observe, when set, receives every unit-of-work event
// the runtime reports: its kind, its result, its duration and its retries.
func (r Runtime) Attach(ctx context.Context, observe func(kind, result string, duration time.Duration, retries int)) context.Context {
	ctx = tenant.WithUnitOfWork(ctx, r.unitOfWork)
	if observe == nil {
		return ctx
	}
	return tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		observe(string(event.Kind), string(event.Result), event.Duration, event.Retries)
	})
}

// bind makes this runtime the one the transaction opens in, so the
// transaction methods need no prior Attach; observers on ctx stay.
func (r Runtime) bind(ctx context.Context) context.Context {
	return tenant.WithUnitOfWork(ctx, r.unitOfWork)
}

// WithinTenant runs fn in the transaction of the school tenantID names.
func (r Runtime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(r.bind(ctx), id, fn)
}

// WithinTenantRetry runs a retry-safe fn in the school's transaction and
// replays it after a deadlock or serialization failure.
func (r Runtime) WithinTenantRetry(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenantRetry(r.bind(ctx), id, fn)
}

// WithinAdmin runs fn in the cross-tenant administrative transaction.
func (r Runtime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(r.bind(ctx), fn)
}

// WithCommitGuard makes every transaction opened under ctx call guard as its
// last statement before commit.
func (r Runtime) WithCommitGuard(ctx context.Context, guard func(context.Context) error) context.Context {
	return tenant.WithCommitGuard(ctx, guard)
}

// RegisterAfterCommit runs fn after the surrounding transaction commits, and
// at once outside a transaction.
func (r Runtime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}
