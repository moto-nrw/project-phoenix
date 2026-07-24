package active

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// lockStaffBalanceWrites serializes every write that can change a staff
// member's Stundenkonto inputs. Adjustment, work-session, and absence
// repositories must all use this exact key.
func lockStaffBalanceWrites(ctx context.Context, db *bun.DB, staffID int64) error {
	if staffID <= 0 {
		return errors.New("staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	key := fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID)
	if err := base.AcquireXactLock(ctx, db, key); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
}
