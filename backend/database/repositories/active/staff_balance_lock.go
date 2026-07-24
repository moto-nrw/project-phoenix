package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/uptrace/bun"
)

// lockStaffBalanceWrites serializes every write that can change a staff
// member's Stundenkonto inputs. Adjustment, work-session, absence, and work
// schedule repositories must all use the base helper's exact key.
func lockStaffBalanceWrites(ctx context.Context, db *bun.DB, staffID int64) error {
	return base.AcquireStaffBalanceLock(ctx, db, staffID)
}
