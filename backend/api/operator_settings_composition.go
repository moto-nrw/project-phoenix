package api

import (
	"context"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The operator's school settings (modules/settings/compose, #2736) take two
// seams from the retained services: the tenant settings broadcast and the
// presence-mode guard's open attendance check.

// OpenAttendanceChecker answers the presence-mode guard from Student
// Presence. Without the presence service it reports no open attendance,
// which disables the guard.
type OpenAttendanceChecker struct{ Service studentpresence.Presence }

func (c OpenAttendanceChecker) HasOpenAttendanceOn(ctx context.Context, day configModel.CalendarDate) (bool, error) {
	if c.Service == nil {
		return false, nil
	}
	value := day.UTCMidnight()
	return c.Service.HasOpenAttendanceOn(ctx, timezone.NewDate(value.Year(), value.Month(), value.Day()))
}
