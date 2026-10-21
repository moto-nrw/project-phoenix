package ports

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareBoundReader projects the last care day of a set of children. Narrow on
// purpose — the materializer needs one DATE column, not student rows (#2487).
type CareBoundReader interface {
	FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]calendar.Date, error)
}

// MaterializationDependencies wires the materializer. The repositories are
// required. DB and RecurrenceLock go together: production wiring provides
// both so direct calls get an RLS-aware transaction and the recurrence gate;
// a nil DB is reserved for pure repository-double unit tests. CareBounds is
// the per-date care filter (#2487); nil means no child has an end of care.
// NonWorkingDays answers holidays and closing days (#3594); nil skips none.
// Staffing announces created instances to the staffing caches (#1844); nil
// announces nothing.
type MaterializationDependencies struct {
	GroupRepo      timetable.ActivityGroupQuery
	ScheduleRepo   timetable.ScheduleQuery
	EnrollmentRepo ActivityEnrollmentRecords
	SupervisorRepo PlannedSupervisorRecords
	PeriodRepo     CalendarPeriodRecords
	InstanceRepo   ActivityInstanceRecords
	StaffRepo      InstanceStaffRecords
	StudentRepo    InstanceStudentRecords
	ExceptionRepo  timetable.ActivityExceptionQuery
	TimeframeRepo  timetable.TimeframeQuery
	CareBounds     CareBoundReader
	NonWorkingDays timetable.NonWorkingDayCalendar
	RecurrenceLock timetable.RecurrenceWriteLock
	Staffing       StaffingAnnouncer
	Transactions   TenantTransactions
	Logger         *slog.Logger
}
