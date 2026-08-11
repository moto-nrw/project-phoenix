package careplanning

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LockExceptionDay serializes every plan exception for one child and calendar
// day, including full-day statuses and time-specific excusals.
func LockExceptionDay(ctx context.Context, db *bun.DB, studentID int64, date timezone.Date) error {
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
