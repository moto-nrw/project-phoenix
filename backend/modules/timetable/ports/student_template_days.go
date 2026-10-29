package ports

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentTemplateDaysDependencies are the owner rows the student template
// days read and write (#5102): the templates with their targets and
// schedules, the roster rows, the occurrences with their participants, and
// the tenant recurrence gate the removal takes. The timeframes give a
// Regeltermin's end, which decides whether a booking carries it (#5342).
type StudentTemplateDaysDependencies struct {
	Groups         timetable.ActivityGroupQuery
	Schedules      timetable.ScheduleQuery
	Timeframes     timetable.TimeframeQuery
	Enrollments    timetable.StudentEnrollmentCapability
	Instances      LegacyInstanceReads
	Participants   ParticipantRows
	RecurrenceLock timetable.RecurrenceWriteLock
	// Today bounds the removal: it never rewrites a past day.
	Today  func() calendar.Date
	Logger *slog.Logger
}
