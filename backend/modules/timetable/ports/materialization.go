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

// SickReportReader lists the sick reports in force that overlap [from, to]
// (#6292). Reports that never take a plan effect (half-day reports, declined
// or cancelled entries) are left out. Workforce owns them; the composition
// root binds this port.
type SickReportReader interface {
	OpenSickReports(ctx context.Context, from, to calendar.Date) ([]timetable.SickReport, error)
}

// MaterializationDependencies wires the materializer. The repositories are
// required. DB and RecurrenceLock go together: production wiring provides
// both so direct calls get an RLS-aware transaction and the recurrence gate;
// a nil DB is reserved for pure repository-double unit tests. CareBounds is
// the per-date care filter (#2487); nil means no child has an end of care.
// NonWorkingDays answers holidays and closing days (#3594); nil skips none.
// Staffing announces created instances to the staffing caches (#1844); nil
// announces nothing. SickReports marks a newly planned supervisor absent on
// the full days of an open sick report (#6292); nil applies none. With a
// database it needs DayLocks: the run takes the substitute day lock of every
// window day so a sick report committing meanwhile is either seen by the run
// or sees the rows it inserted.
type MaterializationDependencies struct {
	GroupRepo      timetable.ActivityGroupQuery
	ScheduleRepo   timetable.ScheduleQuery
	EnrollmentRepo timetable.StudentEnrollmentQuery
	SupervisorRepo timetable.PlannedSupervisorQuery
	// Directories name the enrolled children with their alumni state and
	// the planned supervisors' persons.
	Directories    RepositoryDirectories
	PeriodRepo     CalendarPeriodRecords
	InstanceRepo   InstanceRows
	StaffRepo      timetable.InstanceStaffCapability
	StudentRepo    ParticipantRows
	ExceptionRepo  timetable.ActivityExceptionQuery
	TimeframeRepo  timetable.TimeframeQuery
	CareBounds     CareBoundReader
	SickReports    SickReportReader
	DayLocks       TransactionLocks
	NonWorkingDays timetable.NonWorkingDayCalendar
	RecurrenceLock timetable.RecurrenceWriteLock
	Staffing       StaffingAnnouncer
	Transactions   TenantTransactions
	Logger         *slog.Logger
}
