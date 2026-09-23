package timetableplanning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// InstanceServiceDependencies aggregates wiring. All repo fields are required;
// Broadcaster is optional (nil → no SSE).
type InstanceServiceDependencies struct {
	InstanceRepo       scheduleModel.ActivityInstanceRepository
	IdempotencyRepo    scheduleModel.InstanceIdempotencyRepository
	InstanceStaffRepo  scheduleModel.InstanceStaffRepository
	InstanceStudents   scheduleModel.InstanceStudentRepository
	ExceptionRepo      scheduleModel.ActivityExceptionRepository
	ActiveGroupRepo    studentpresence.SessionRecords
	SupervisorRepo     studentpresence.SupervisionRecords
	Presence           InstancePresence
	RoomRepo           facilitiesModel.RoomRepository
	ActivityGroupRepo  activitiesModel.GroupRepository
	StaffRepo          usersModel.StaffRepository
	StudentRepo        usersModel.StudentRepository
	CalendarPeriodRepo scheduleModel.CalendarPeriodRepository
	ActiveService      ActiveSessionEnder
	// Materialization re-plans a window through the Timetable owner's
	// recurrence engine; RecurrenceLock is the owner's tenant recurrence gate
	// every template-derived write takes first (#3424 slice S2).
	Materialization timetable.MaterializationCapability
	RecurrenceLock  timetable.RecurrenceWriteLock
	// CareDayService decides which still-expected children may be stamped
	// absent when an instance ends (#1747) — required.
	CareDayService InstanceCareDays
	// DeviationEventRepo appends the Änderungsprotokoll (#1886) — required.
	DeviationEventRepo auditModel.DeviationEventRepository
	Broadcaster        realtime.Broadcaster
	DB                 *bun.DB
	Logger             *slog.Logger
	Settings           LifecycleSettings
	RecoveryRepo       scheduleModel.ActivityRecoveryRepository
	Now                func() time.Time
	EnforceTimePolicy  bool
	// GuardianNotices publishes the cancellation notice to families (#2601).
	// Optional: nil means a cancellation can never carry a notice. The
	// composition root passes a late-bound publisher because the announcement
	// service is built after this one.
	GuardianNotices announcement.CareCancellationPublisher
	// StartConflicts is the Timetable owner's start check (#2139, #3550) —
	// required for Start.
	StartConflicts timetable.StartConflictQuery
	// SubstituteConflicts is the Timetable owner's substitute time-overlap
	// advisory (#3424 slice S3) — required for the staff move.
	SubstituteConflicts timetable.SubstituteConflictQuery
}

// detectStartConflicts asks the owner's start check for the block and keeps
// the retained error classification: a failed read aborts the transition as
// a ScheduleError.
func detectStartConflicts(ctx context.Context, query timetable.StartConflictQuery, instance *scheduleModel.ActivityInstance) ([]timetable.InstanceConflictWarning, error) {
	if query == nil {
		return nil, &ScheduleError{Op: "detect start conflicts", Err: errors.New("start conflict detection is not configured")}
	}
	warnings, err := query.DetectStartConflicts(ctx, timetable.StartConflictSubject{InstanceID: instance.ID, RoomID: instance.RoomID})
	if err != nil {
		return nil, &ScheduleError{Op: "detect start conflicts", Err: err}
	}
	return warnings, nil
}

// staffingRows maps retained staff rows onto the input of the owner's
// staffing rule; nil rows are ignored.
func staffingRows(rows []*scheduleModel.InstanceStaff) []timetable.InstanceStaff {
	out := make([]timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, timetable.InstanceStaff{IsAbsent: row.IsAbsent, IsSubstitute: row.IsSubstitute})
		}
	}
	return out
}

// ScheduleError is the operation-wrapping error the retained lifecycle,
// template and deviation services share with Care Plan: one type, so
// errors.As matches either name (#3551 decision). It goes with this package.
type ScheduleError = careplan.ScheduleError

// StudentVisitReader reads the Student Presence visits.
type StudentVisitReader interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

// InstancePresence supplies authoritative presence state for lifecycle
// transitions: current visits, staff supervision facts and the session moves.
type InstancePresence interface {
	StudentVisitReader
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	TransferOpenVisits(context.Context, int64, int64) (int64, error)
	// EndGroup releases an absorbed unsupervised group after its visits moved
	// to the started instance's group (#2697).
	EndGroup(context.Context, int64, time.Time) error
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
}

// substituteDayLockKey is the Timetable owner's day-wide staffing lock key
// (timetable.SubstituteDayLockKey).
func substituteDayLockKey(tenantID int64, date timezone.Date) string {
	return timetable.SubstituteDayLockKey(tenantID, date)
}

// SubstituteDayLock takes the day-wide staffing lock (substituteDayLockKey)
// on the caller's transaction, for compositions that serialize with the
// retained deviation and substitution writes (#1843 sick cascade).
func SubstituteDayLock(db *bun.DB) func(context.Context, timezone.Date) error {
	return func(ctx context.Context, date timezone.Date) error {
		return repoBase.AcquireXactLock(ctx, db, substituteDayLockKey(tenant.FromContext(ctx), date))
	}
}

type legacyListRepository[T any] interface {
	List(context.Context, *modelBase.QueryOptions) ([]T, error)
}

// legacyList reads through the retained repositories' List(options), which
// they serve beside their model interfaces.
func legacyList[T any](ctx context.Context, repository any, options *modelBase.QueryOptions) ([]T, error) {
	lister, ok := repository.(legacyListRepository[T])
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", repository)
	}
	return lister.List(ctx, options)
}

// int64FilterArgs widens IDs for the persistence-neutral Filter.In API.
func int64FilterArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

func dateFilterArgs(dates []timezone.Date) []any {
	args := make([]any, len(dates))
	for i, date := range dates {
		args[i] = date
	}
	return args
}

func activityInstanceIDs(instances []*scheduleModel.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func indexInstanceStaffRows(rows []*scheduleModel.InstanceStaff) map[int64][]*scheduleModel.InstanceStaff {
	byInstance := make(map[int64][]*scheduleModel.InstanceStaff)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

// formatTimeOfDay formats the time-of-day component of t as "15:04:05", the
// location-independent key the materializer dedupes occurrences by.
func formatTimeOfDay(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

// MinSchoolGradeLevel and MaxSchoolGradeLevel are School Structure's
// supported grade range, which the Timetable owner's template writes
// validate Jahrgang targets and filters against (#3424 slice S2). The
// composition root binds them from here, beside NormalizeSchoolClass.
const (
	MinSchoolGradeLevel = schoolclass.MinGradeLevel
	MaxSchoolGradeLevel = schoolclass.MaxGradeLevel
)

// NormalizeSchoolClass is School Structure's class-name normalization, which
// the Timetable owner's class-block read compares with (#2970); the retained
// template writes normalize the same way.
func NormalizeSchoolClass(class string) string {
	return schoolclass.Normalize(class)
}
