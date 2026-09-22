package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewStaffShiftLock returns the per-staff shift write lock. The overlap check
// is a read-before-write in the planning service, and the table's unique
// constraint only blocks identical start times — without this lock two
// concurrent requests can both pass the check and commit overlapping shifts.
// The key is staff-level (not per-date) so an update that moves a shift to
// another date never needs two lock keys. Callers outside the module bind it
// too: the sick cascade writes shift rows of its own and must serialize
// against the planning routes on the same advisory key.
func NewStaffShiftLock(db *bun.DB) func(ctx context.Context, staffID int64) error {
	return func(ctx context.Context, staffID int64) error {
		return lockStaffShiftWrites(ctx, db, staffID)
	}
}

func lockStaffShiftWrites(ctx context.Context, db *bun.DB, staffID int64) error {
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
