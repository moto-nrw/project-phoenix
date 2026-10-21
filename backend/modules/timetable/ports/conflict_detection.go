package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ConflictShifts reads the Dienstplan rows of Workforce.
type ConflictShifts interface {
	FindByDateRange(ctx context.Context, start, end calendar.Date) ([]*scheduleModels.StaffShift, error)
	FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []calendar.Date) ([]*scheduleModels.StaffShift, error)
	FindUsedCalendarWeeks(ctx context.Context, start, end calendar.Date) ([]calendar.Date, error)
}

// ConflictStaffDirectory reads staff members with their person rows.
type ConflictStaffDirectory interface {
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error)
	ListAllWithPerson(ctx context.Context) ([]*timetable.StaffMember, error)
}

// ConflictCalendarPeriods reads one calendar period; a missing period is a
// not-found error.
type ConflictCalendarPeriods interface {
	FindByID(ctx context.Context, id int64) (*scheduleModels.CalendarPeriod, error)
}

// ConflictArrivalExceptions reads the dated arrival exceptions of students.
type ConflictArrivalExceptions interface {
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*scheduleModels.StudentArrivalException, error)
}

// ConflictPresence supplies the current visits and staff supervisions of
// Student Presence.
type ConflictPresence interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

// ConflictSessions reads running Student Presence sessions with their room.
type ConflictSessions interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*studentpresence.SessionDetail, error)
}

// ArrivalBaselineProjector is the consumer-owned port to Care Plan's
// regular arrival plan (#2414, ADR 0005): the class timetable supplies the
// time and, with enrollment.bookings_authoritative on, the approved bookings
// supply the care days. One projection covers the inclusive window.
type ArrivalBaselineProjector interface {
	ProjectArrivals(ctx context.Context, studentIDs []int64, from, to calendar.Date) (ArrivalBaselines, error)
}

// ArrivalBaselines answers the regular arrival time of a student on a date;
// ok is false when no recurring arrival applies.
type ArrivalBaselines interface {
	ExpectedArrival(studentID int64, date calendar.Date) (arrival time.Time, ok bool)
}

// ConflictDetectionDependencies are the readers the composition root binds.
// ContentHash is Security Runtime's content fingerprint: the hex SHA-256 of
// the content. The conflict fingerprints persisted in per-user
// acknowledgements are its first 32 hex characters. ArrivalBaselines is
// optional; without it the exception-conflict read fails once it needs a
// recurring arrival.
type ConflictDetectionDependencies struct {
	RunInTx           func(context.Context, func(context.Context) error) error
	Instances         LegacyInstanceReads
	InstanceStaff     timetable.InstanceStaffQuery
	InstanceStudents  ParticipantRows
	Exceptions        timetable.ActivityExceptionQuery
	Schedules         timetable.ScheduleQuery
	Shifts            ConflictShifts
	Staff             ConflictStaffDirectory
	CalendarPeriods   ConflictCalendarPeriods
	ArrivalExceptions ConflictArrivalExceptions
	ArrivalBaselines  ArrivalBaselineProjector
	Presence          ConflictPresence
	Sessions          ConflictSessions
	ContentHash       func([]byte) string
	Logger            *slog.Logger
}
