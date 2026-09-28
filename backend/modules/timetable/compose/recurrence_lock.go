package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// RecurrenceLockDependencies wires the tenant recurrence gate
// (timetable.RecurrenceWriteLock). The gate takes transaction-scoped
// advisory locks on the caller's transaction over DB. GradeTransitionLockKey
// is School Structure's grade-transition gate key
// (education.TenantTransitionsLockKey), which this gate takes second.
type RecurrenceLockDependencies struct {
	DB                     *bun.DB
	GradeTransitionLockKey func(tenantID int64) string
}

type recurrenceWriteLock struct {
	store                  *postgres.Store
	gradeTransitionLockKey func(tenantID int64) string
}

// NewRecurrenceWriteLock composes the tenant recurrence gate.
func NewRecurrenceWriteLock(deps RecurrenceLockDependencies) (timetable.RecurrenceWriteLock, error) {
	if deps.DB == nil || deps.GradeTransitionLockKey == nil {
		return nil, errors.New("timetable recurrence lock: required dependency is nil")
	}
	return recurrenceWriteLock{
		store:                  postgres.New(databaseRuntime(deps.DB)),
		gradeTransitionLockKey: deps.GradeTransitionLockKey,
	}, nil
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
func (l recurrenceWriteLock) LockRecurrenceWrites(ctx context.Context) error {
	tenantID, err := lockTenant(ctx, "template recurrence lock requires a transaction")
	if err != nil {
		return err
	}
	if err := l.store.AcquireTransactionLock(ctx, schedule.TenantRecurrenceLockKey(tenantID)); err != nil {
		return fmt.Errorf("lock template recurrence writes: %w", err)
	}
	return nil
}

// LockRecurrenceWritesThenGradeTransitions takes the recurrence gate and then
// the gate School Structure holds for the whole of a grade transition's apply
// or revert, so a roster-writing pass and a grade transition never run
// concurrently for one school (#405 review). The order is fixed project-wide:
// recurrence first, transitions second.
func (l recurrenceWriteLock) LockRecurrenceWritesThenGradeTransitions(ctx context.Context) error {
	if err := l.LockRecurrenceWrites(ctx); err != nil {
		return err
	}
	tenantID, err := lockTenant(ctx, "grade transition lock requires a transaction")
	if err != nil {
		return err
	}
	if err := l.store.AcquireTransactionLock(ctx, l.gradeTransitionLockKey(tenantID)); err != nil {
		return fmt.Errorf("lock tenant grade transitions: %w", err)
	}
	return nil
}

func lockTenant(ctx context.Context, missingTransaction string) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, errors.New("tenant id is required")
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return 0, errors.New(missingTransaction)
	}
	return tenantID, nil
}
