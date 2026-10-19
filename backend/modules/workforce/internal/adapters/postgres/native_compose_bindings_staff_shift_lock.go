package postgres

import (
	context "context"
	errors "errors"
	fmt "fmt"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

func LockStaffShiftWrites(ctx context.Context, db *bun.DB, staffID int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	key := fmt.Sprintf("staff-shift:%d:%d", tenantID, staffID)
	if err := AcquireXactLock(ctx, key, New(DatabaseRuntime(db)).AcquireXactLock); err != nil {
		return fmt.Errorf("lock staff shift writes: %w", err)
	}
	return nil
}

func AcquireXactLock(ctx context.Context, key string, lock func(context.Context, string) error) error {
	if _, inTransaction := tenant.TransactionFromContext(ctx); inTransaction {
		err := tenant.AcquireLock(ctx, key, false)
		if err == nil || !errors.Is(err, tenant.ErrRuntimeRequired) {
			return err
		}
	}
	return lock(ctx, key)
}
