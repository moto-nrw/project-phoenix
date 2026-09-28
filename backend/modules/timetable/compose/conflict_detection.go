package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Conflict detection and staffing (#3424 slice S4, #3550): the start and
// planning conflict checks (WP-B9, #2139), the exception-conflict warnings
// (WP-B13), the staff availability pool (#1884) and the shift-coverage probe
// (#1873) behind the public timetable.ConflictDetectionCapability. The
// detection still reads the retained repository rows, whose status carries
// the Student Presence session state the owner's own rows do not, so the
// readers below are the retained repository shapes the composition root
// binds.

// ConflictInstances is the slice of the retained activity instance
// repository the detection reads. The start check also lists the instances
// bridged to running sessions through the retained List(options) read, which
// the repository serves beside this interface.
type ConflictInstances interface {
	FindByID(ctx context.Context, id any) (*scheduleModels.ActivityInstance, error)
	FindByTenantAndDateRange(ctx context.Context, from, to scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error)
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error)
}

// ConflictInstanceStaff reads the staff rows of blocks.
type ConflictInstanceStaff interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStaff, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStaff, error)
}

// ConflictInstanceStudents reads the student rows of blocks.
type ConflictInstanceStudents interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error)
	FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
}

// ConflictExceptions reads activity exceptions by window.
type ConflictExceptions interface {
	FindByDateRange(ctx context.Context, from, to scheduleModels.Date) ([]*scheduleModels.ActivityException, error)
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to scheduleModels.Date) ([]*scheduleModels.ActivityException, error)
}

// ConflictSchedules reads the template schedules of activity groups.
type ConflictSchedules interface {
	FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.Schedule, error)
	FindTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModels.TemplateStartTime, error)
}

// ConflictShifts reads the Dienstplan rows of Workforce.
type ConflictShifts interface {
	FindByDateRange(ctx context.Context, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error)
	FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []scheduleModels.Date) ([]*scheduleModels.StaffShift, error)
	FindUsedCalendarWeeks(ctx context.Context, start, end scheduleModels.Date) ([]scheduleModels.Date, error)
}

// ConflictStaffDirectory reads staff members with their person rows.
type ConflictStaffDirectory interface {
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error)
	ListAllWithPerson(ctx context.Context) ([]*usersModels.Staff, error)
}

// ConflictCalendarPeriods reads one calendar period; a missing period is a
// not-found error.
type ConflictCalendarPeriods interface {
	FindByID(ctx context.Context, id any) (*scheduleModels.CalendarPeriod, error)
}

// ConflictArrivalExceptions reads the dated arrival exceptions of students.
type ConflictArrivalExceptions interface {
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date scheduleModels.Date) ([]*scheduleModels.StudentArrivalException, error)
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
	ProjectArrivals(ctx context.Context, studentIDs []int64, from, to timezone.Date) (ArrivalBaselines, error)
}

// ArrivalBaselines answers the regular arrival time of a student on a date;
// ok is false when no recurring arrival applies.
type ArrivalBaselines interface {
	ExpectedArrival(studentID int64, date timezone.Date) (arrival time.Time, ok bool)
}

// ConflictDetectionDependencies are the readers the composition root binds.
// ContentHash is Security Runtime's content fingerprint: the hex SHA-256 of
// the content. The conflict fingerprints persisted in per-user
// acknowledgements are its first 32 hex characters. ArrivalBaselines is
// optional; without it the exception-conflict read fails once it needs a
// recurring arrival.
type ConflictDetectionDependencies struct {
	Instances         ConflictInstances
	InstanceStaff     ConflictInstanceStaff
	InstanceStudents  ConflictInstanceStudents
	Exceptions        ConflictExceptions
	Schedules         ConflictSchedules
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

type conflictDetection struct {
	deps   ConflictDetectionDependencies
	logger *slog.Logger
}

// NewConflictDetection composes the conflict detection and staffing
// capability. Every reader except ArrivalBaselines is required: the checks
// cannot decide whether to warn without them.
func NewConflictDetection(deps ConflictDetectionDependencies) (timetable.ConflictDetectionCapability, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil || deps.InstanceStudents == nil || deps.Exceptions == nil ||
		deps.Schedules == nil || deps.Shifts == nil || deps.Staff == nil || deps.CalendarPeriods == nil ||
		deps.ArrivalExceptions == nil || deps.Presence == nil || deps.Sessions == nil || deps.ContentHash == nil {
		return nil, errors.New("timetable conflict detection: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &conflictDetection{deps: deps, logger: logger}, nil
}

// germanDateLayout renders calendar dates as dd.mm.yyyy in user-facing copy.
const germanDateLayout = "02.01.2006"

// isoWeekday returns the ISO weekday (Monday=1 … Sunday=7) of the date.
func isoWeekday(date timezone.Date) int {
	weekday := date.Weekday()
	if weekday == time.Sunday {
		return 7
	}
	return int(weekday)
}

// clockWindowsOverlap reports whether two wall-clock windows intersect;
// touching boundaries do not overlap (same rule as StaffShift.Overlaps).
func clockWindowsOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	as, ae := timezone.NormalizeWallClock(aStart), timezone.NormalizeWallClock(aEnd)
	bs, be := timezone.NormalizeWallClock(bStart), timezone.NormalizeWallClock(bEnd)
	return as.Before(be) && bs.Before(ae)
}

// isPlannableInstance keeps the blocks that still occupy their people:
// planned or running ones.
func isPlannableInstance(instance *scheduleModels.ActivityInstance) bool {
	return instance.Status == scheduleModels.InstanceStatusPlanned ||
		instance.Status == scheduleModels.InstanceStatusActive
}

// shiftWindowsOf maps Dienstplan rows onto the public interval vocabulary.
func shiftWindowsOf(shifts []*scheduleModels.StaffShift) []timetable.ShiftWindow {
	windows := make([]timetable.ShiftWindow, 0, len(shifts))
	for _, shift := range shifts {
		if shift != nil {
			windows = append(windows, timetable.ShiftWindow{StartTime: shift.StartTime, EndTime: shift.EndTime, Cancelled: shift.Cancelled})
		}
	}
	return windows
}
