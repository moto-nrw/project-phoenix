package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// lockScheduleDays is the one acquisition of the day-wide staffing locks
// (timetable.SubstituteDayLockKey, #1840) for every Timetable path and for
// the shift-plan-sync cascade (#6232). It takes the days once each,
// ascending, and admits each through the unit of work's lock order
// (tenant.OrderLock): a staff-shift gate or the recurrence gate after a day,
// or an earlier day after a later one, is refused before it can wait, so the
// path fails instead of closing a deadlock cycle. acquire takes one
// transaction-scoped advisory lock on the hashed key.
func lockScheduleDays(ctx context.Context, acquire func(context.Context, string) error, tenantID int64, days ...calendar.Date) error {
	for _, day := range ascendingDays(days) {
		if err := tenant.OrderLock(ctx, tenant.LockScheduleDay, day.String()); err != nil {
			return err
		}
		if err := acquire(ctx, timetable.SubstituteDayLockKey(tenantID, day)); err != nil {
			return err
		}
	}
	return nil
}
