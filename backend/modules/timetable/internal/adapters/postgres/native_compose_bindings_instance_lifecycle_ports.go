package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SubstituteDayLock takes the day-wide staffing lock (#1840) on the caller's
// transaction, for compositions that serialize with the owner's staffing
// writes (the #1843 sick cascade).
func SubstituteDayLock(db *bun.DB) func(context.Context, calendar.Date) error {
	store := New(DatabaseRuntime(db))
	return func(ctx context.Context, date calendar.Date) error {
		return store.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenant.FromContext(ctx), date))
	}
}
