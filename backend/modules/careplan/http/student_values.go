package careplanhttp

import (
	context "context"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// WeekdayPickupNoteReplacer owns the one atomic write that replaces the
// recurring day notes for a child. It deliberately excludes dated notes.
type WeekdayPickupNoteReplacer interface {
	ReplaceWeekdayPickupNotes(context.Context, int64, int64, map[int]string) error
}

func (rs *Resource) todayDate() calendar.Date {
	return calendar.DateFromTime(rs.Now())
}

// withinTenant runs fn in the transaction of tenantID. Inside a route that
// already runs under the tenant transaction middleware it joins that
// transaction, so an error returned from fn rolls the whole request back.
func withinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		tenant.ObserveMissingTenant(ctx, err)
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}
