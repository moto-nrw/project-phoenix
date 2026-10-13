package common

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantRuntimeContext is another owner's view of the tenant and the
// transaction the runtime opens; the runtime attaches both to every unit of
// work and detaches the transaction where it detaches its own.
type TenantRuntimeContext struct {
	WithTenant      func(context.Context, int64) context.Context
	WithTransaction func(context.Context, any) context.Context
}

// BindTenantRuntime binds the PostgreSQL tenant and administrative
// transactions as the unit of work tenant work runs under. A savepoint
// controller that also takes advisory locks, detaches its transaction or
// adapts the context hands those abilities to the runtime too; attached
// names the context the transactions are handed to besides.
func BindTenantRuntime(
	withinTenant func(context.Context, int64, func(context.Context, any) error) error,
	withinAdmin func(context.Context, func(context.Context, any) error) error,
	savepoints tenant.SavepointController,
	retryable func(error) bool,
	attached TenantRuntimeContext,
) (TenantRuntime, error) {
	runtime, err := tenant.NewUnitOfWork(
		withinTenant,
		withinAdmin,
		tenant.SavepointFunc(savepoints),
		retryable,
		savepointLocks(savepoints),
	)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	runtime = withSavepointContext(runtime, savepoints)
	runtime = runtime.WithTransactionDetacher(func(ctx context.Context) context.Context {
		return attached.WithTransaction(ctx, nil)
	})
	return runtime.WithContextAdapters(attached.WithTenant, attached.WithTransaction), nil
}

func savepointLocks(savepoints tenant.SavepointController) func(context.Context, string, bool) error {
	if locks, ok := savepoints.(interface {
		AcquireLock(context.Context, string, bool) error
	}); ok {
		return locks.AcquireLock
	}
	return nil
}

func withSavepointContext(runtime tenant.UnitOfWork, savepoints tenant.SavepointController) tenant.UnitOfWork {
	if detacher, ok := savepoints.(interface {
		ContextWithoutTransaction(context.Context) context.Context
	}); ok {
		runtime = runtime.WithTransactionDetacher(detacher.ContextWithoutTransaction)
	}
	if adapter, ok := savepoints.(interface {
		ContextWithTenant(context.Context, int64) context.Context
		ContextWithTransaction(context.Context, any) context.Context
	}); ok {
		runtime = runtime.WithContextAdapters(adapter.ContextWithTenant, adapter.ContextWithTransaction)
	}
	return runtime
}

// ObserveUnitOfWorkPoolWait bridges the PostgreSQL adapter's pool wait to the
// transaction observer.
func ObserveUnitOfWorkPoolWait(ctx context.Context, duration time.Duration) {
	tenant.ObservePoolWait(ctx, duration)
}

// ObserveUnitOfWorkLockWait bridges the PostgreSQL adapter's lock wait to the
// transaction observer.
func ObserveUnitOfWorkLockWait(ctx context.Context, duration time.Duration) {
	tenant.ObserveLockWait(ctx, duration)
}
