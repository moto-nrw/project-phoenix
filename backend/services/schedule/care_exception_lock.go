package schedule

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LockCareExceptionDay serializes pickup and arrival exception writes for one
// child-day. The parent portal treats staff ownership as day-level state, while
// the data lives in two tables, so every writer must take the same lock before
// checking or mutating either leg.
func LockCareExceptionDay(ctx context.Context, db *bun.DB, studentID int64, date timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	key := fmt.Sprintf("care-exception-day:%d:%d:%s", tenantID, studentID, date.String())
	if err := base.AcquireXactLock(ctx, db, key); err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
	}
	return nil
}
