package schedule

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// lockTenantRecurrenceWrites serializes recurrence mutation, re-planning, and
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
func lockTenantRecurrenceWrites(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if _, ok := modelBase.TxFromContext(ctx); !ok {
		return errors.New("template recurrence lock requires a transaction")
	}
	key := fmt.Sprintf("template-recurrence:%d", tenantID)
	if err := base.AcquireXactLock(ctx, db, key); err != nil {
		return fmt.Errorf("lock template recurrence writes: %w", err)
	}
	return nil
}

// lockTenantGradeTransitions takes the tenant-wide gate that
// education.GradeTransitionRepository.LockTenantTransitions holds for the whole
// of an apply/revert, so a materialization pass and a grade transition can never
// run concurrently for one school. See education.TenantTransitionsLockKey for
// the race this closes and for the required lock ordering: the recurrence gate
// is always taken FIRST, this one second.
func lockTenantGradeTransitions(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if _, ok := modelBase.TxFromContext(ctx); !ok {
		return errors.New("grade transition lock requires a transaction")
	}
	if err := base.AcquireXactLock(ctx, db, education.TenantTransitionsLockKey(tenantID)); err != nil {
		return fmt.Errorf("lock tenant grade transitions: %w", err)
	}
	return nil
}

// LockTenantRecurrenceWrites exposes the shared tenant-wide recurrence gate to
// other services that mutate recurrence-derived state (currently enrollment
// care-offering rosters). Callers must already be inside the tenant request
// transaction so the advisory lock covers their complete mutation.
func LockTenantRecurrenceWrites(ctx context.Context, db *bun.DB) error {
	return lockTenantRecurrenceWrites(ctx, db)
}
