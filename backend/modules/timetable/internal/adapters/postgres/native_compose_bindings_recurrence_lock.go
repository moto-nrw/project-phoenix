package postgres

import (
	context "context"
	errors "errors"
	fmt "fmt"

	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

type RecurrenceWriteLock struct {
	Store                  *Store
	GradeTransitionLockKey func(tenantID int64) string
}

// LockRecurrenceWrites serializes recurrence mutation, re-planning, and
// materialization inside one tenant. The lock must be acquired before reading
// schedule bounds: otherwise a concurrent split/end can commit valid_until
// after another operation read NULL, allowing stale schedules or instances to
// be written across the new boundary.
//
// A tenant-wide key is intentional. Split holds the gate and then invokes a
// materialization pass over every template; per-template locks would accumulate
// in different orders and deadlock against another full materialization. Schools
// remain independent. The PostgreSQL advisory lock is transaction-scoped;
// requiring an existing transaction prevents an implicit one-statement
// transaction from releasing it before the protected operation finishes.
func (l RecurrenceWriteLock) LockRecurrenceWrites(ctx context.Context) error {
	tenantID, err := LockTenant(ctx, "template recurrence lock requires a transaction")
	if err != nil {
		return err
	}
	if err := l.Store.AcquireTransactionLock(ctx, schedule.TenantRecurrenceLockKey(tenantID)); err != nil {
		return fmt.Errorf("lock template recurrence writes: %w", err)
	}
	return nil
}

// LockRecurrenceWritesThenGradeTransitions takes the recurrence gate and then
// the gate School Structure holds for the whole of a grade transition's apply
// or revert, so a roster-writing pass and a grade transition never run
// concurrently for one school (#405 review). The order is fixed project-wide:
// recurrence first, transitions second.
func (l RecurrenceWriteLock) LockRecurrenceWritesThenGradeTransitions(ctx context.Context) error {
	if err := l.LockRecurrenceWrites(ctx); err != nil {
		return err
	}
	tenantID, err := LockTenant(ctx, "grade transition lock requires a transaction")
	if err != nil {
		return err
	}
	if err := l.Store.AcquireTransactionLock(ctx, l.GradeTransitionLockKey(tenantID)); err != nil {
		return fmt.Errorf("lock tenant grade transitions: %w", err)
	}
	return nil
}

func LockTenant(ctx context.Context, missingTransaction string) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, errors.New("tenant id is required")
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return 0, errors.New(missingTransaction)
	}
	return tenantID, nil
}
