package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"
)

// LockHolidayClosurePreparation holds current school memberships and then
// calendars in ID order through commit, before locking a command or Enrollment.
func (s *HolidayClosureEventStore) LockHolidayClosurePreparation(ctx context.Context, organizationID int64, schoolIDs []int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(lockHolidayClosureCalendarsSQL, pgdialect.Array(schoolIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("school calendar postgres: lock the notice memberships and calendars: %w", err)
	}
	return nil
}
