package schedule

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LockStaffShiftWrites serializes shift writes for one staff member. The
// overlap check is a read-before-write in the service, and the table's unique
// constraint only blocks identical start times — without this lock two
// concurrent requests can both pass the check and commit overlapping shifts.
// The key is staff-level (not per-date) so an update that moves a shift to
// another date never needs two lock keys.
func LockStaffShiftWrites(ctx context.Context, db *bun.DB, staffID int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	key := fmt.Sprintf("staff-shift:%d:%d", tenantID, staffID)
	if err := base.AcquireXactLock(ctx, db, key); err != nil {
		return fmt.Errorf("lock staff shift writes: %w", err)
	}
	return nil
}
