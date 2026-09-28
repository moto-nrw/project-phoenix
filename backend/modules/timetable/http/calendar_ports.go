package timetablehttp

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// CalendarPeriods is the slice of the School Calendar the period editor
// drives: the reads plus the administrative write path that enforces the
// school's rules (name uniqueness, the same-type overlap invariant, the
// recurrence gate and the care-offering guard).
type CalendarPeriods interface {
	FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error)
	ListCalendarPeriods(context.Context, schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
	schoolcalendar.CalendarPeriodAdministration
}

// ClosingDays is the slice of the School Calendar the closing-day editor
// drives (#1418 3b).
type ClosingDays interface {
	FindClosingDay(context.Context, int64) (schoolcalendar.ClosingDay, error)
	ListClosingDays(context.Context, schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error)
	CreateClosingDay(context.Context, schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error)
	UpdateClosingDay(context.Context, schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error)
	DeleteClosingDay(context.Context, int64) error
}

// CalendarPeriodUsageCounts reports how many rows reference one calendar
// period through nullable calendar_period_id FKs; the counts drive the list
// display and the delete warnings.
type CalendarPeriodUsageCounts struct {
	EnrollmentPhases   int
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// CalendarPeriodUsage answers the reference counts per period of the
// current tenant; the planning owners answer, not the calendar (#3124).
type CalendarPeriodUsage interface {
	UsageCounts(context.Context) (map[int64]CalendarPeriodUsageCounts, error)
}

// germanCalendarDate renders a DateLayout day as dd.mm.yyyy for user-facing
// text; an unparseable value is shown as is.
func germanCalendarDate(value string) string {
	day, err := time.Parse(schoolcalendar.DateLayout, value)
	if err != nil {
		return value
	}
	return day.Format(germanDateLayout)
}
