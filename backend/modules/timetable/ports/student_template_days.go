package ports

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentTemplateDaysDependencies are the owner rows the student template
// days read and write (#5102): the templates with their targets and
// schedules, the roster rows, the occurrences with their participants, and
// the tenant recurrence gate the removal takes. The timeframes give a
// Regeltermin's end, which decides whether a booking carries it, and the
// active periods tell which schedules still plan from a date on (#5342).
type StudentTemplateDaysDependencies struct {
	Groups         timetable.ActivityGroupQuery
	Schedules      timetable.ScheduleQuery
	Timeframes     timetable.TimeframeQuery
	Periods        ActiveCalendarPeriods
	Enrollments    timetable.StudentEnrollmentCapability
	Instances      LegacyInstanceReads
	Participants   ParticipantRows
	RecurrenceLock timetable.RecurrenceWriteLock
	// Staffing wakes the planning caches after a removal or a „Bleibt so“
	// commits (#5102); nil wakes none.
	Staffing StaffingAnnouncer
	// Today bounds the removal: it never rewrites a past day.
	Today  func() calendar.Date
	Logger *slog.Logger
}

// ActiveCalendarPeriods reads the tenant's active calendar periods, which
// materialization selects a schedule's period from.
type ActiveCalendarPeriods interface {
	FindActiveByTenantID(ctx context.Context) ([]*schedulerecords.CalendarPeriod, error)
}
