package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The instance lifecycle (WP-B9, #3424 slice S1) behind
// timetable.InstanceLifecycleCapability: the transitions of one block
// (instance_start.go, instance_complete.go, instance_reopen.go,
// instance_cancel.go), its planning writes (instance_create.go,
// instance_update.go, instance_replan.go), the staffing writes
// (instance_understaffed.go, instance_move_staff.go), the guardian notice of
// a cancellation (instance_guardian_notice.go) and the realtime
// announcements (instance_broadcast.go). The series conversion and the
// scheduler's auto start and auto end run on top of it.
//
// The lifecycle still writes the retained repository rows, whose status
// carries the Student Presence session state. Collaborators the owner may
// not name are consumer-owned ports the composition root binds: the
// Facilities rooms, Care Plan's care days, Communication's cancellation
// notice, the Settings Platform's clock policy, the Audit Platform's
// Änderungsprotokoll, Security Runtime's content fingerprint and the
// realtime hub.

// LifecyclePresence is the Student Presence slice the transitions read and
// move live state through.
type LifecyclePresence interface {
	ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	QueryGroupSupervisions(ctx context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	TransferOpenVisits(ctx context.Context, fromGroupID, toGroupID int64) (int64, error)
	// EndGroup releases an absorbed unsupervised group after its visits
	// moved to the started block's group (#2697).
	EndGroup(ctx context.Context, groupID int64, endedAt time.Time) error
	CountOpenVisitsInRoom(ctx context.Context, roomID int64) (int, error)
}

// SessionEnder ends the Student Presence session of a block: visits and
// supervisors close and the checkout events fire.
type SessionEnder interface {
	EndActivitySession(ctx context.Context, activeGroupID int64) error
}

// LifecycleRooms is the consumer-owned port to the Facilities rooms the
// lifecycle validates, locks and names.
type LifecycleRooms interface {
	// RoomName names one room; ok is false when it does not exist. A
	// missing room may also surface as a not-found error.
	RoomName(ctx context.Context, id int64) (name string, ok bool, err error)
	// RoomNamesByID names the given rooms; a missing room is absent.
	RoomNamesByID(ctx context.Context, ids []int64) (map[int64]string, error)
	// LockRoom takes the room's row lock. ok is false when the room does
	// not exist; a nil capacity is unlimited.
	LockRoom(ctx context.Context, id int64) (capacity *int, ok bool, err error)
}

// CareDayLocks takes Care Plan's student → care-day lock a roster rewrite
// holds before it touches attendance rows.
type CareDayLocks interface {
	LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error
}

// LifecycleSettings is the Settings Platform's clock policy of the
// transitions.
type LifecycleSettings interface {
	// StartLeadMinutes is timetable.start_lead_minutes.
	StartLeadMinutes(ctx context.Context) (int, error)
	// EnforcePlannedEnd is timetable.enforce_planned_end.
	EnforcePlannedEnd(ctx context.Context) (bool, error)
}

// InstanceLifecycleDependencies wires the lifecycle. Broadcaster and
// GuardianNotices are optional (nil announces nothing and refuses every
// notice); Settings is required with EnforceTimePolicy; Logger and Now
// default. Every other field is required.
type InstanceLifecycleDependencies struct {
	InstanceRepo        scheduleModel.ActivityInstanceRepository
	IdempotencyRepo     scheduleModel.InstanceIdempotencyRepository
	InstanceStaffRepo   scheduleModel.InstanceStaffRepository
	InstanceStudents    scheduleModel.InstanceStudentRepository
	ExceptionRepo       scheduleModel.ActivityExceptionRepository
	CalendarPeriodRepo  scheduleModel.CalendarPeriodRepository
	RecoveryRepo        scheduleModel.ActivityRecoveryRepository
	ActivityGroupRepo   activitiesModel.GroupRepository
	StaffRepo           usersModel.StaffRepository
	StudentRepo         usersModel.StudentRepository
	ActiveGroupRepo     studentpresence.SessionRecords
	SupervisorRepo      studentpresence.SupervisionRecords
	Presence            LifecyclePresence
	ActiveService       SessionEnder
	Rooms               LifecycleRooms
	CareDays            CareDays
	CareDayLocks        CareDayLocks
	Protocol            DeviationProtocol
	GuardianNotices     GuardianNotices
	Broadcaster         LifecycleBroadcaster
	Settings            LifecycleSettings
	Materialization     timetable.MaterializationCapability
	RecurrenceLock      timetable.RecurrenceWriteLock
	StartConflicts      timetable.StartConflictQuery
	SubstituteConflicts timetable.SubstituteConflictQuery
	// ContentHash is Security Runtime's content fingerprint (hex SHA-256)
	// of an idempotent create request.
	ContentHash       func([]byte) string
	DB                *bun.DB
	Logger            *slog.Logger
	Now               func() time.Time
	EnforceTimePolicy bool
}

// InstanceLifecycleService is the Timetable owner's instance lifecycle.
type InstanceLifecycleService struct {
	deps  InstanceLifecycleDependencies
	store *postgres.Store
}

var _ timetable.InstanceLifecycleCapability = (*InstanceLifecycleService)(nil)

// NewInstanceLifecycle composes the lifecycle. The transitions have no
// degraded mode, so a missing required collaborator fails composition.
func NewInstanceLifecycle(deps InstanceLifecycleDependencies) (*InstanceLifecycleService, error) {
	if !lifecycleRepositoriesWired(deps) || !lifecycleCollaboratorsWired(deps) || (deps.EnforceTimePolicy && deps.Settings == nil) {
		return nil, errors.New("timetable instance lifecycle: required dependency is nil")
	}
	return &InstanceLifecycleService{deps: deps, store: postgres.New(databaseRuntime(deps.DB))}, nil
}

func lifecycleRepositoriesWired(deps InstanceLifecycleDependencies) bool {
	return deps.InstanceRepo != nil && deps.IdempotencyRepo != nil && deps.InstanceStaffRepo != nil &&
		deps.InstanceStudents != nil && deps.ExceptionRepo != nil && deps.CalendarPeriodRepo != nil &&
		deps.RecoveryRepo != nil && deps.ActivityGroupRepo != nil && deps.StaffRepo != nil && deps.StudentRepo != nil
}

func lifecycleCollaboratorsWired(deps InstanceLifecycleDependencies) bool {
	return deps.ActiveGroupRepo != nil && deps.SupervisorRepo != nil && deps.Presence != nil && deps.ActiveService != nil &&
		deps.Rooms != nil && deps.CareDays != nil && deps.CareDayLocks != nil && deps.Protocol != nil &&
		deps.Materialization != nil && deps.RecurrenceLock != nil && deps.StartConflicts != nil &&
		deps.SubstituteConflicts != nil && deps.ContentHash != nil && deps.DB != nil
}

// SeriesDeviations exposes the re-plan's deviation snapshot and reapply
// machinery (#1840) to the template split, which preserves Vertretungsplan
// overrides with it.
func (s *InstanceLifecycleService) SeriesDeviations() SeriesDeviations {
	return seriesDeviationPreserver{lifecycle: s}
}

func (s *InstanceLifecycleService) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *InstanceLifecycleService) getLogger() *slog.Logger {
	return orDefaultLogger(s.deps.Logger)
}

// hasTx reports whether the caller already runs inside the tenant
// transaction the advisory gates need. False for direct calls outside
// TenantTxMiddleware (CLI, tests), which then get their own transaction.
func (s *InstanceLifecycleService) hasTx(ctx context.Context) bool {
	if s.deps.DB == nil {
		return true // no DB wired (unit tests): nothing to lock, nothing to wrap
	}
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every same-day staffing mutation, within the caller's tx.
func (s *InstanceLifecycleService) acquireSubstituteDayLock(ctx context.Context, date timezone.Date) error {
	return s.acquireTenantDayLock(ctx, tenant.FromContext(ctx), date)
}

func (s *InstanceLifecycleService) acquireTenantDayLock(ctx context.Context, tenantID int64, date timezone.Date) error {
	return s.store.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenantID, date))
}

// acquireSubstituteDayLocks takes the day lock of every date in [from, to]
// inclusive, in ascending order, so two overlapping windows can never
// deadlock (#1840).
func (s *InstanceLifecycleService) acquireSubstituteDayLocks(ctx context.Context, tenantID int64, from, to timezone.Date) error {
	for d := from; !d.After(to); d = d.AddDays(1) {
		if err := s.acquireTenantDayLock(ctx, tenantID, d); err != nil {
			return err
		}
	}
	return nil
}

// acquireSubstituteDayLockPair takes the day lock of two dates (deduped) in
// ascending order, sharing the total order of acquireSubstituteDayLocks so
// a planned edit that moves a block never deadlocks against a re-plan or a
// deviation save (#1840).
func (s *InstanceLifecycleService) acquireSubstituteDayLockPair(ctx context.Context, tenantID int64, a, b timezone.Date) error {
	first, second := a, b
	if second.Before(first) {
		first, second = second, first
	}
	if err := s.acquireTenantDayLock(ctx, tenantID, first); err != nil {
		return err
	}
	if second == first {
		return nil
	}
	return s.acquireTenantDayLock(ctx, tenantID, second)
}

// lockDayAndReload takes the block's day lock and re-reads the block under
// it. A concurrent edit may have MOVED the block to another day while the
// caller waited; the held lock then no longer covers the block's real day,
// so the transition is refused with ErrInstanceMoved (#1840).
func (s *InstanceLifecycleService) lockDayAndReload(ctx context.Context, instance *scheduleModel.ActivityInstance, op string) (*scheduleModel.ActivityInstance, error) {
	lockedDate := instance.Date
	if err := s.acquireSubstituteDayLock(ctx, timezone.Date(lockedDate)); err != nil {
		return nil, &ScheduleError{Op: op + ": lock day", Err: err}
	}
	reloaded, err := s.loadForTransition(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	if reloaded.Date != lockedDate {
		return nil, timetable.ErrInstanceMoved
	}
	return reloaded, nil
}

// lockRecurrenceThenGradeTransitions takes the two tenant-wide gates every
// writer of recurrence-derived roster state holds, recurrence FIRST and
// grade transitions second. The transitions gate keeps a concurrent
// graduation from re-adding a departed child to a manual roster (#405
// review); taking the recurrence gate first keeps the acquisition acyclic
// against the day locks.
func (s *InstanceLifecycleService) lockRecurrenceThenGradeTransitions(ctx context.Context, op string) error {
	if s.deps.DB == nil || s.deps.RecurrenceLock == nil {
		return nil
	}
	if err := s.deps.RecurrenceLock.LockRecurrenceWritesThenGradeTransitions(ctx); err != nil {
		return &ScheduleError{Op: op + ": lock recurrence", Err: err}
	}
	return nil
}

// loadForTransition is the shared load and not-found branch of the
// transitions. The retained repository wraps sql.ErrNoRows, so IsNoRows
// unwraps it.
func (s *InstanceLifecycleService) loadForTransition(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, timetable.ErrInstanceNotFound
		}
		return nil, &ScheduleError{Op: "load instance", Err: err}
	}
	if instance == nil {
		return nil, timetable.ErrInstanceNotFound
	}
	return instance, nil
}

// updateLifecycleColumns writes only the named columns: bun decodes TIME
// columns as year 0000, which PostgreSQL rejects on write, so a full-row
// update would break the block — and a transition must not clobber fields
// it does not own.
func (s *InstanceLifecycleService) updateLifecycleColumns(ctx context.Context, instance *scheduleModel.ActivityInstance, columns ...string) error {
	if len(columns) == 0 {
		return nil
	}
	_, err := s.deps.InstanceRepo.UpdateColumns(ctx, instance, columns...)
	return err
}

// GetPlannedStudentIDsByDate returns which of the children have a planned
// block on the date.
func (s *InstanceLifecycleService) GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error) {
	return s.deps.InstanceStudents.FindPlannedStudentIDsByDate(ctx, studentIDs, scheduleModel.Date(date))
}

// detectStartConflicts asks the owner's start check and keeps the retained
// error classification: a failed read aborts the transition.
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

// LifecycleInstanceOf maps a retained row onto the lifecycle's public view.
func LifecycleInstanceOf(row *scheduleModel.ActivityInstance) *timetable.LifecycleInstance {
	if row == nil {
		return nil
	}
	return &timetable.LifecycleInstance{
		ID: row.ID, Date: timezone.Date(row.Date), StartTime: row.StartTime, EndTime: row.EndTime,
		Title: row.Title, RoomID: row.RoomID, ActivityGroupID: row.ActivityGroupID, Status: row.Status,
		IsSpontaneous: row.IsSpontaneous, UnderstaffedAck: row.UnderstaffedAck, UnderstaffedNote: row.UnderstaffedNote,
		ActiveGroupID: row.ActiveGroupID, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, ReopenUntil: row.ReopenUntil,
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

// anyArgs widens values for the persistence-neutral Filter.In API.
func anyArgs[T any](values []T) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value
	}
	return args
}

func instanceRowIDs(instances []*scheduleModel.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func ptrTo[T any](value T) *T { return &value }
