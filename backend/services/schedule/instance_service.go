// Package schedule — instance lifecycle service (WP-B9).
//
// Drives the state machine on schedule.activity_instances:
//
//	          start                 complete
//	planned  ─────────►  active  ─────────►  completed
//	   │                   │
//	   │ cancel            │ cancel
//	   ▼                   ▼
//	cancelled           cancelled
//
// Every illegal transition returns ErrInvalidInstanceTransition (→ 409).
// Start creates an active.group bridge row + copies instance_staff rows to
// active.group_supervisors in the caller's tenant transaction. Cancel from
// active ends the active.group cleanly via active.EndActivitySession —
// visits close, supervisors close, SSE checkout events fire — so no ghost
// state is left behind.
//
// Re-plan-week is the admin escape hatch: delete all status='planned'
// non-spontaneous rows in a date window, then re-materialize. active/
// completed/cancelled and spontaneous planned rows are never touched.
package schedule

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Sentinel errors callers branch on via errors.Is.
var (
	// ErrInstanceNotFound is returned when an instance id does not resolve in
	// the current tenant's scope. Handlers map this to 404.
	ErrInstanceNotFound = errors.New("activity instance not found")

	// ErrInvalidInstanceTransition is returned when the requested lifecycle
	// action is not legal from the instance's current status. Handlers map
	// this to 409. The error message carries the current status so clients
	// can show a useful diagnostic without re-fetching.
	ErrInvalidInstanceTransition = errors.New("invalid instance transition")

	// ErrInvalidInstanceReference is returned when a caller supplies a
	// tenant-scoped foreign id that does not resolve in the current tenant.
	// Handlers map this to 400.
	ErrInvalidInstanceReference = errors.New("invalid instance reference")

	// ErrInstanceWeekend is returned when an update introduces or restores a
	// weekend date. Existing legacy weekend rows may retain their date, but the
	// decision is made after the instance and day locks are held.
	ErrInstanceWeekend = errors.New("timetable entries can only be scheduled from Monday to Friday")

	// ErrAmbiguousTemplateInstanceDelete is returned when a single-instance
	// delete would need to persist a date-wide cancellation exception but the
	// template has multiple materialized slots on that date. Handlers map this
	// to 409 so the UI can explain that the user must delete the series or keep
	// the slots unchanged until slot-scoped exceptions exist.
	ErrAmbiguousTemplateInstanceDelete = errors.New("template instance delete is ambiguous")

	// ErrUnderstaffedAckStillStaffed is returned when a caller tries to mark a
	// block "deliberately unstaffed" (understaffed_ack=true) while it still has
	// at least one non-absent staff row. The flag means "runs on purpose with
	// nobody covering"; allowing it alongside real staffing produces
	// contradictory state (the block reads as unstaffed yet /gaps never lists it
	// because it is staffed). Handlers map this to 409.
	ErrUnderstaffedAckStillStaffed = errors.New("cannot acknowledge understaffing while the block is fully staffed")

	// ErrInstanceMoved is returned when a lifecycle transition (start, complete,
	// cancel, update) reloads the instance under its (tenant, date) day lock and
	// finds the block on a DIFFERENT date than the one it locked — a concurrent
	// PUT /instances/{id} moved it while this call was waiting on the lock. The
	// held advisory lock is keyed on the stale date, so continuing would act on
	// the moved block without holding its real day's lock and would break the
	// ascending day-lock ordering the other flows rely on to avoid deadlock.
	// Handlers map this to 409 with code "instance_moved" so the client reopens
	// the block on its new day (#1840).
	ErrInstanceMoved = errors.New("instance was moved concurrently")

	ErrInstanceStartTooEarly       = errors.New("activity instance cannot be started yet")
	ErrInstanceStartExpired        = errors.New("activity instance can no longer be started")
	ErrInstanceCompleteEarly       = errors.New("activity instance cannot be completed before planned end")
	ErrLifecycleSettings           = errors.New("activity lifecycle settings unavailable")
	ErrCompletionConfirmationStale = errors.New("activity completion confirmation is stale")
)

type LifecycleSettings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type lifecycleContextKey string

const lifecycleActorKey lifecycleContextKey = "actor"
const lifecycleConfirmedStudentsKey lifecycleContextKey = "confirmed-students"

func WithLifecycleActor(ctx context.Context, accountID int64) context.Context {
	return context.WithValue(ctx, lifecycleActorKey, accountID)
}

func WithCompletionConfirmation(ctx context.Context, studentIDs []int64) context.Context {
	return context.WithValue(ctx, lifecycleConfirmedStudentsKey, slices.Clone(studentIDs))
}

type LifecycleAvailability struct {
	CanStart            bool
	StartAvailableAt    time.Time
	CanComplete         bool
	CompleteAvailableAt time.Time
}

// EvaluateLifecycleAvailability is the shared clock policy used by API
// payloads and lifecycle writes. Planned starts are valid from the configured
// lead boundary until (but not including) plan end. Spontaneous blocks are not
// constrained by plan times.
func EvaluateLifecycleAvailability(instance *scheduleModel.ActivityInstance, now time.Time, startLeadMinutes int, enforcePlannedEnd bool) LifecycleAvailability {
	start := instanceBoundary(instance.Date, instance.StartTime)
	end := instanceBoundary(instance.Date, instance.EndTime)
	availableAt := start.Add(-time.Duration(startLeadMinutes) * time.Minute)
	if instance.IsSpontaneous {
		return LifecycleAvailability{CanStart: true, StartAvailableAt: now, CanComplete: true, CompleteAvailableAt: now}
	}
	return LifecycleAvailability{
		CanStart:            !now.Before(availableAt) && now.Before(end),
		StartAvailableAt:    availableAt,
		CanComplete:         !enforcePlannedEnd || !now.Before(end),
		CompleteAvailableAt: end,
	}
}

// ActiveSessionEnder is the subset of active.Service used by Complete and
// Cancel. Defined as a local interface so unit tests can stub the call
// without constructing the full active service.
type ActiveSessionEnder interface {
	EndActivitySession(ctx context.Context, activeGroupID int64) error
}

// InstanceService drives lifecycle transitions on schedule.activity_instances.
type InstanceService interface {
	Start(ctx context.Context, instanceID, startedByStaffID int64) (*StartInstanceResult, error)
	Complete(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error)
	Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*StartInstanceResult, error)
	// Cancel transitions planned|active → cancelled. reason is an optional
	// short "why" stored on the instance (Vertretungsplan, #1840); pass nil for
	// a plain cancel. actorAccountID stamps the Änderungsprotokoll entry
	// (#1886); nil records an actor-less event.
	Cancel(ctx context.Context, instanceID int64, reason *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error)
	DeleteCancelled(ctx context.Context, instanceID int64) error
	// SetUnderstaffedAck flips the "deliberately unstaffed" acknowledgement on a
	// planned or active instance (Vertretungsplan, issue #1840). It only
	// annotates the block — no lifecycle transition, no active-state change — so
	// gap detection stops reporting an intentionally-open position. Rejected on
	// completed/cancelled instances with ErrInvalidInstanceTransition.
	SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error)
	// ClearUnderstaffedAckIfStaffed clears a lingering "deliberately unstaffed"
	// acknowledgement only when the instance's current staff rows leave it fully
	// staffed (present >= planned). Used by the /substitute flow after adding
	// coverage so partial coverage never reopens an acknowledged gap (#1840).
	ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error
	// ReplanWeek deletes planned non-spontaneous instances in [from, to] and
	// re-materializes. A non-nil activityGroupID restricts the delete to one
	// template's instances; nil re-plans the whole grid.
	ReplanWeek(ctx context.Context, from, to timezone.Date, activityGroupID *int64, actorAccountID *int64) (*ReplanWeekResult, error)
	// GetPlannedStudentIDsByDate returns the unique student IDs (of the given
	// candidates) that have a planned instance on the date (issue #584
	// lookup; repository result returned verbatim).
	GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error)
	Create(ctx context.Context, req CreateInstanceInput) (*scheduleModel.ActivityInstance, error)
	UpdatePlanned(ctx context.Context, instanceID int64, req UpdateInstanceInput, actorAccountID *int64) (*scheduleModel.ActivityInstance, error)

	// Day-wide deviation writes (#1840/#1886) — the ONLY write path for
	// absence/presence/substitution deviations; each appends its
	// Änderungsprotokoll entry in the same tx. See deviation_service.go.
	ApplyAbsence(ctx context.Context, row *scheduleModel.InstanceStaff, instance *scheduleModel.ActivityInstance, reason *string, actorAccountID *int64, activeTouched map[int64]*scheduleModel.ActivityInstance) error
	ApplyPresence(ctx context.Context, row *scheduleModel.InstanceStaff, instance *scheduleModel.ActivityInstance, actorAccountID *int64, activeTouched map[int64]*scheduleModel.ActivityInstance) error
	ApplySubstitute(ctx context.Context, op SubstituteWriteOp, subID int64, reason *string, now time.Time, actorAccountID *int64, activeTouched map[int64]*scheduleModel.ActivityInstance) error

	// #1843 sick-cascade variants: same writes as ApplyAbsence/ApplyPresence
	// but with provenance stamping and the sick_reported/sick_cleared events.
	ApplySickAbsence(ctx context.Context, row *scheduleModel.InstanceStaff, instance *scheduleModel.ActivityInstance, reason *string, sickAbsenceID int64, actorAccountID *int64, activeTouched map[int64]*scheduleModel.ActivityInstance) error
	ClearSickAbsence(ctx context.Context, row *scheduleModel.InstanceStaff, instance *scheduleModel.ActivityInstance, sickAbsenceID int64, actorAccountID *int64, activeTouched map[int64]*scheduleModel.ActivityInstance) error
	// QueueActivityUpdates emits one activity_update per touched active group
	// after the surrounding tenant transaction commits. Rollbacks emit nothing.
	QueueActivityUpdates(ctx context.Context, touched map[int64]*scheduleModel.ActivityInstance)

	// ApplyDeviations applies a whole Vertretungsplan slide-over save atomically
	// (#1840/#1886): day-lock, validate + classify (Phase A), then the absence /
	// presence / substitution writes plus acknowledgement reconciliation
	// (Phase B). See deviation_apply.go. Returns a DeviationError carrying the
	// exact HTTP mapping on a validation/conflict failure.
	ApplyDeviations(ctx context.Context, instanceID int64, in ApplyDeviationsInput) (*ApplyDeviationsResult, error)
	// MoveStaffBetweenBlocks moves (or pool-assigns) one staff member onto the
	// target block atomically in one save (#1884): removal from the source and
	// assignment to the target share the day lock and the tenant tx, and leave
	// a single staff_moved Änderungsprotokoll entry. See instance_move_staff.go.
	MoveStaffBetweenBlocks(ctx context.Context, targetID int64, in MoveStaffInput) (*MoveStaffResult, error)
	// AcknowledgeUnderstaffed applies the standalone "deliberately unstaffed"
	// acknowledgement: it gates past blocks, serializes against same-day staffing
	// saves, then delegates to SetUnderstaffedAck. The note must arrive already
	// trimmed/validated.
	AcknowledgeUnderstaffed(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error)
}

// CreateInstanceInput bundles the fields needed to insert a fresh instance
// (spontaneous or scheduled) outside the materialization flow.
//
// By default, IsSpontaneous is computed from ActivityGroupID: when nil the
// instance is purely free-form; when set it is bound to a template (e.g. an
// admin-scheduled extra Yoga slot using the existing Yoga template's metadata,
// but on a date that materialization would not have emitted). Operational
// spontaneous starts may override this while still linking template metadata.
type CreateInstanceInput struct {
	Date             timezone.Date
	StartTime        time.Time // 2000-01-01 HH:MM in UTC
	EndTime          time.Time // 2000-01-01 HH:MM in UTC
	Title            string
	Description      *string
	Notes            *string
	RoomID           int64
	ActivityGroupID  *int64
	ListKind         *string
	IsSpontaneous    *bool
	StaffIDs         []int64
	StudentIDs       []int64
	CreatedByStaffID *int64
	// RequiredStaff is the optional manual Personalbedarf override (#1839);
	// nil = derive from the Betreuungsschlüssel.
	RequiredStaff *int
}

type UpdateInstanceInput struct {
	Date            timezone.Date
	StartTime       time.Time
	EndTime         time.Time
	Title           string
	Description     *string
	Notes           *string
	RoomID          int64
	ActivityGroupID *int64
	ListKind        *string
	StaffIDs        []int64
	StudentIDs      []int64
	// RequiredStaff is the optional manual Personalbedarf override (#1839);
	// nil = derive from the Betreuungsschlüssel.
	RequiredStaff *int
	// CalendarPeriodID, when non-nil, stamps the materializer marker used by
	// FindPlannedTemplateBackedFrom (activity_group_id set, period set,
	// is_spontaneous=false). Convert-to-series is the only caller that sets
	// this on a pre-existing seed; ordinary PUT leaves the column alone so
	// hand-linked one-offs stay outside enrollment resync.
	CalendarPeriodID *int64
}

// StartInstanceResult bundles what the start endpoint returns. ActiveGroupID
// mirrors instance.ActiveGroupID for clients that only consume the envelope.
type StartInstanceResult struct {
	Instance      *scheduleModel.ActivityInstance
	ActiveGroupID int64
	Warnings      []InstanceConflictWarning
}

// ReplanWeekResult wraps the materialization result plus the delete count so
// admins see both numbers in one response.
type ReplanWeekResult struct {
	From             timezone.Date
	To               timezone.Date
	DeletedInstances int
	Materialization  *MaterializationResult
}

// InstanceServiceDependencies aggregates wiring. All repo fields are required;
// Broadcaster is optional (nil → no SSE).
type InstanceServiceDependencies struct {
	InstanceRepo      scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	InstanceStudents  scheduleModel.InstanceStudentRepository
	ExceptionRepo     scheduleModel.ActivityExceptionRepository
	ActiveGroupRepo   activeModel.GroupRepository
	SupervisorRepo    activeModel.GroupSupervisorRepository
	VisitRepo         activeModel.VisitRepository
	RoomRepo          facilitiesModel.RoomRepository
	ActivityGroupRepo activitiesModel.GroupRepository
	StaffRepo         usersModel.StaffRepository
	StudentRepo       usersModel.StudentRepository
	ActiveService     ActiveSessionEnder
	Materialization   MaterializationService
	// CareDayService decides which still-expected children may be stamped
	// absent when an instance ends (#1747) — required.
	CareDayService CareDayService
	// DeviationEventRepo appends the Änderungsprotokoll (#1886) — required.
	DeviationEventRepo auditModel.DeviationEventRepository
	Broadcaster        realtime.Broadcaster
	DB                 *bun.DB
	Logger             *slog.Logger
	Settings           LifecycleSettings
	RecoveryRepo       scheduleModel.ActivityRecoveryRepository
	Now                func() time.Time
	EnforceTimePolicy  bool
}

type instanceService struct {
	deps InstanceServiceDependencies
}

// NewInstanceService constructs an InstanceService. Panics if a required
// dependency is nil — the service has no sensible degraded mode for lifecycle
// transitions, so the factory must wire it completely at startup.
func NewInstanceService(deps InstanceServiceDependencies) InstanceService {
	if deps.InstanceRepo == nil || deps.InstanceStaffRepo == nil || deps.InstanceStudents == nil ||
		deps.ExceptionRepo == nil ||
		deps.ActiveGroupRepo == nil || deps.SupervisorRepo == nil || deps.VisitRepo == nil ||
		deps.RoomRepo == nil || deps.ActivityGroupRepo == nil || deps.StaffRepo == nil ||
		deps.StudentRepo == nil || deps.ActiveService == nil || deps.Materialization == nil ||
		deps.CareDayService == nil || deps.DeviationEventRepo == nil || deps.DB == nil ||
		(deps.EnforceTimePolicy && (deps.Settings == nil || deps.RecoveryRepo == nil)) {
		panic("schedule.NewInstanceService: required dependency is nil")
	}
	return &instanceService{deps: deps}
}

func (s *instanceService) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func instanceBoundary(day timezone.Date, wallClock time.Time) time.Time {
	return time.Date(day.Year, day.Month, day.Day, wallClock.Hour(), wallClock.Minute(), wallClock.Second(), wallClock.Nanosecond(), timezone.Berlin)
}

func (s *instanceService) validateStartTime(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time) error {
	if !s.deps.EnforceTimePolicy || instance.IsSpontaneous {
		return nil
	}
	lead, err := s.deps.Settings.ResolveInt(ctx, configModel.KeyTimetableStartLeadMinutes)
	if err != nil {
		return fmt.Errorf("%w: resolve start lead: %v", ErrLifecycleSettings, err)
	}
	availability := EvaluateLifecycleAvailability(instance, now, lead, true)
	if now.Before(availability.StartAvailableAt) {
		return fmt.Errorf("%w: available at %s", ErrInstanceStartTooEarly, availability.StartAvailableAt.Format(time.RFC3339))
	}
	if !availability.CanStart {
		return ErrInstanceStartExpired
	}
	return nil
}

func (s *instanceService) validateCompleteTime(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time) error {
	if !s.deps.EnforceTimePolicy || instance.IsSpontaneous {
		return nil
	}
	enforce, err := s.deps.Settings.ResolveBool(ctx, configModel.KeyTimetableEnforcePlannedEnd)
	if err != nil {
		return fmt.Errorf("%w: resolve planned end policy: %v", ErrLifecycleSettings, err)
	}
	availability := EvaluateLifecycleAvailability(instance, now, 0, enforce)
	if !availability.CanComplete {
		return fmt.Errorf("%w: available at %s", ErrInstanceCompleteEarly, instanceBoundary(instance.Date, instance.EndTime).Format(time.RFC3339))
	}
	return nil
}

func (s *instanceService) getLogger() *slog.Logger {
	return cmp.Or(s.deps.Logger, slog.Default())
}

// notScheduledStudentIDs returns the instance's assigned children who are not
// booked into care at all on the instance's date (#1747). Used to spare them
// the expected → absent stamp when an instance ends.
//
// A child whose day was explicitly cancelled is NOT in this list: that is a
// reported absence and has to be written, or it vanishes from the attendance
// history and the exports (see CareDayStatus.ExemptFromAbsence).
//
// Neither is a child whose row somebody set by hand (ManualStatusAt). Staff can
// set an unbooked slot back to 'expected' — "the plan is wrong, this child is
// coming" — and that decision outranks the derivation: the row is a genuine
// expectation, so it must take the ordinary expected → absent path rather than
// be spared it and stamped as a non-booking (#1747 review).
func (s *instanceService) notScheduledStudentIDs(
	ctx context.Context, instance *scheduleModel.ActivityInstance,
) ([]int64, error) {
	rows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: load attendance rows", Err: err}
	}
	if len(rows) == 0 {
		return nil, nil
	}

	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ManualStatusAt != nil {
			continue
		}
		studentIDs = append(studentIDs, row.StudentID)
	}
	if len(studentIDs) == 0 {
		return nil, nil
	}

	careDay, err := s.deps.CareDayService.ResolveForDate(ctx, studentIDs, instance.Date)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: resolve care day", Err: err}
	}

	notScheduled := make([]int64, 0)
	for _, studentID := range studentIDs {
		if careDay[studentID].ExemptFromAbsence() {
			notScheduled = append(notScheduled, studentID)
		}
	}
	return notScheduled, nil
}

// Start implements planned → active. Runs inside the caller's tenant tx
// (TenantTxMiddleware); any failure rolls back the whole thing — no dangling
// active.group, no half-linked instance, no stale supervisors.
func (s *instanceService) Start(ctx context.Context, instanceID, startedByStaffID int64) (*StartInstanceResult, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot start instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	if err := s.validateStartTime(ctx, instance, s.now()); err != nil {
		return nil, err
	}

	// Serialize against concurrent day-wide staffing saves (/substitute,
	// /deviations) on the block's day BEFORE reading the roster we copy into
	// active.group_supervisors. Those endpoints hold this (tenant, date) advisory
	// lock while they flip is_absent and insert substitute rows; without contending
	// on it, Start can read the pre-deviation roster and create supervisors from
	// stale rows — leaving a just-absented planned supervisor live and the assigned
	// substitute missing from the running session. Advisory xact locks are
	// re-entrant, matching Cancel's day lock. Reload under the lock so a concurrent
	// cancel/complete is observed before we materialize the bridge (#1840).
	lockedDate := instance.Date
	if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenant.FromContext(ctx), lockedDate)); err != nil {
		return nil, &ScheduleError{Op: "start instance: lock day", Err: err}
	}
	instance, err = s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// A concurrent PUT /instances/{id} may have MOVED the block to another day
	// while we waited on the lock keyed on lockedDate. Continuing would copy the
	// roster into active.group_supervisors while holding only the stale day's
	// lock, letting /substitute or /deviations saves for the block's real date
	// race with us. Reject so the client reopens the block on its new day (#1840).
	if instance.Date != lockedDate {
		return nil, ErrInstanceMoved
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot start instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	if err := s.validateStartTime(ctx, instance, s.now()); err != nil {
		return nil, err
	}

	// Conflict detection is read-only + advisory. Warnings reflect state
	// inside the tx; they never block the transition.
	warnings := DetectStartConflicts(ctx, ConflictDependencies{
		GroupRepo:         s.deps.ActiveGroupRepo,
		SupervisorRepo:    s.deps.SupervisorRepo,
		VisitRepo:         s.deps.VisitRepo,
		InstanceRepo:      s.deps.InstanceRepo,
		InstanceStaffRepo: s.deps.InstanceStaffRepo,
		InstanceStudents:  s.deps.InstanceStudents,
	}, instance, s.getLogger())

	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "start instance: load instance_staff", Err: err}
	}

	now := s.now()
	newGroup := &activeModel.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        instance.ActivityGroupID,
		DeviceID:       nil,
		RoomID:         instance.RoomID,
	}
	newGroup.SetTenantID(tenant.FromContext(ctx))
	if err := s.deps.ActiveGroupRepo.Create(ctx, newGroup); err != nil {
		return nil, &ScheduleError{Op: "start instance: create active.group", Err: err}
	}

	// Each non-absent staff row becomes a supervisor. IsAbsent=true rows are
	// intentionally skipped — those staff are marked out for this instance
	// (sick, substitution flow, etc.); copying them into active.group_supervisors
	// would make the staff appear as actively supervising, pollute the staff
	// conflict model for their next planner start elsewhere, and contradict
	// the semantics of the absence flag.
	//
	// A full-row failure aborts the whole transition. Unlike the NFC best-
	// effort path, a planner start with missing supervisors would silently
	// undermine the conflict model the next time that staff starts elsewhere.
	activeStaffRows := make([]*scheduleModel.InstanceStaff, 0, len(staffRows))
	for _, row := range staffRows {
		if row.IsAbsent {
			continue
		}
		activeStaffRows = append(activeStaffRows, row)
		sup := &activeModel.GroupSupervisor{
			StaffID:   row.StaffID,
			GroupID:   newGroup.ID,
			Role:      "supervisor",
			StartDate: timezone.DateFromTime(now),
		}
		sup.SetTenantID(tenant.FromContext(ctx))
		if err := s.deps.SupervisorRepo.Create(ctx, sup); err != nil {
			return nil, &ScheduleError{Op: "start instance: create supervisor", Err: err}
		}
	}

	// Fold open, supervisor-less sessions in this room into the fresh bridge
	// group before announcing it — same tenant tx, so a failure rolls back the
	// whole transition.
	if err := s.absorbUnsupervisedOpenGroups(ctx, instance.ID, instance.RoomID, newGroup.ID); err != nil {
		return nil, &ScheduleError{Op: "start instance: absorb unsupervised sessions", Err: err}
	}

	// Targeted UPDATE: only the columns affected by the transition. A full-row
	// Update via the repo would re-marshal StartTime/EndTime (TIME columns)
	// that bun decodes as year 0000 on read — Postgres rejects year 0000 on
	// write, so a round-trip through repo.Update() breaks. Touch only what
	// this transition actually changes.
	instance.Status = scheduleModel.InstanceStatusActive
	activeGroupID := newGroup.ID
	instance.ActiveGroupID = &activeGroupID
	if startedByStaffID > 0 {
		startedByCopy := startedByStaffID
		instance.StartedBy = &startedByCopy
	}
	instance.StartedAt = &now
	if err := s.updateLifecycleColumns(ctx, instance, "status", "active_group_id", "started_at", "started_by"); err != nil {
		return nil, &ScheduleError{Op: "start instance: update instance", Err: err}
	}

	s.broadcastInstanceEvent(ctx, realtime.EventInstanceStarted, instance, newGroup, activeStaffRows)

	return &StartInstanceResult{
		Instance:      instance,
		ActiveGroupID: newGroup.ID,
		Warnings:      warnings,
	}, nil
}

// absorbUnsupervisedOpenGroups folds today's unbridged sessions without an
// active supervisor in the given room into the freshly started bridge group:
// their open visits move over, then the orphan session is ended. Typical case:
// a kiosk scan into the room auto-created an unsupervised fallback session
// (e.g. Schulhof, #2161) before the planned block started. A group already
// linked to any timetable instance is not a fallback and must retain its
// bridge, even when the instance currently has no active supervisor. Supervised
// sessions are also left alone: parallel supervised groups in one room are a
// sanctioned pattern (#2139). Absorbed open visits are mirrored into the
// target instance's attendance rows before commit. No per-student SSE fires
// here; the EventInstanceStarted broadcast that follows makes clients refetch.
func (s *instanceService) absorbUnsupervisedOpenGroups(ctx context.Context, instanceID, roomID, newGroupID int64) error {
	openGroups, err := s.deps.ActiveGroupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return fmt.Errorf("find open groups in room %d: %w", roomID, err)
	}

	// Lock candidates in ascending ID order. Today every other active.groups
	// locker takes exactly one row lock per transaction, so no wait cycle can
	// form regardless of order — but a deterministic order keeps this loop
	// deadlock-free even if a second multi-row locker appears later.
	slices.SortFunc(openGroups, func(a, b *activeModel.Group) int {
		return cmp.Compare(a.ID, b.ID)
	})

	today := timezone.TodayDate()
	movedTotal := 0
	for _, group := range openGroups {
		if group.ID == newGroupID {
			continue
		}
		if timezone.DateFromTime(group.StartTime) != today {
			continue
		}

		// Serialize the "still open and unsupervised" decision with generic
		// supervision claims. ClaimActiveGroup and CreateGroupSupervisor take the
		// same row lock before they check EndTime and insert their supervisor row,
		// so either the claim wins and this re-check sees a supervisor, or
		// absorption wins and the claimant sees the ended session after waiting.
		lockedGroup, err := s.deps.ActiveGroupRepo.FindByIDForUpdate(ctx, group.ID)
		if err != nil {
			return fmt.Errorf("lock open group %d: %w", group.ID, err)
		}
		if lockedGroup == nil || lockedGroup.EndTime != nil {
			continue
		}
		if lockedGroup.RoomID != roomID {
			continue
		}
		if timezone.DateFromTime(lockedGroup.StartTime) != today {
			continue
		}

		instance, err := s.deps.InstanceRepo.FindByActiveGroupID(ctx, group.ID)
		if err != nil {
			return fmt.Errorf("load timetable bridge of group %d: %w", group.ID, err)
		}
		if instance != nil {
			continue
		}

		supervisors, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, group.ID, true)
		if err != nil {
			return fmt.Errorf("load supervisors of group %d: %w", group.ID, err)
		}
		if len(supervisors) > 0 {
			continue
		}

		moved, err := s.deps.VisitRepo.TransferActiveVisitsBetweenGroups(ctx, group.ID, newGroupID)
		if err != nil {
			return fmt.Errorf("move open visits from group %d to group %d: %w", group.ID, newGroupID, err)
		}

		if err := s.deps.ActiveGroupRepo.EndSession(ctx, group.ID); err != nil {
			return fmt.Errorf("end absorbed group %d: %w", group.ID, err)
		}
		movedTotal += moved
		s.getLogger().Info("absorbed unsupervised session into started instance",
			slog.Int64("absorbed_group_id", group.ID),
			slog.Int64("new_group_id", newGroupID),
			slog.Int("moved_visits", moved),
		)
	}

	if movedTotal > 0 {
		if err := s.syncAbsorbedVisitAttendance(ctx, instanceID, newGroupID); err != nil {
			return err
		}
	}

	return nil
}

func (s *instanceService) syncAbsorbedVisitAttendance(ctx context.Context, instanceID, activeGroupID int64) error {
	visits, err := s.deps.VisitRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return fmt.Errorf("load absorbed visits from group %d: %w", activeGroupID, err)
	}

	for _, visit := range visits {
		if visit == nil || visit.ExitTime != nil {
			continue
		}

		updated, err := s.deps.InstanceStudents.UpdateAttendanceFromCheckin(
			ctx, instanceID, visit.StudentID, visit.EntryTime,
		)
		if err != nil {
			return fmt.Errorf("mark absorbed student %d present: %w", visit.StudentID, err)
		}
		if updated {
			continue
		}

		attendance, err := s.deps.InstanceStudents.FindByInstanceAndStudent(ctx, instanceID, visit.StudentID)
		if err != nil {
			return fmt.Errorf("load absorbed student %d attendance: %w", visit.StudentID, err)
		}
		if attendance != nil {
			continue
		}

		if _, err := s.deps.InstanceStudents.CreateUnplannedPresentIfAbsent(
			ctx, instanceID, visit.StudentID, visit.EntryTime,
		); err != nil {
			return fmt.Errorf("create absorbed student %d attendance: %w", visit.StudentID, err)
		}
	}
	return nil
}

// Complete implements active → completed. The bridge active.group is ended
// via active.EndActivitySession — this closes open visits, ends supervisors,
// and fires per-student checkout SSE events, matching today's observable
// behavior when a session ends.
func (s *instanceService) Complete(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	// Serialize completion against concurrent day-wide staffing saves
	// (/substitute, /deviations) on the block's day, exactly as Cancel does. Those
	// endpoints take this (tenant, date) advisory lock, re-read the instance under
	// it, then rewrite instance_staff and open active.group_supervisors rows.
	// Without the same lock here, a deviation save can pass its own "still
	// plannable" re-read while Complete is mid-flight, then commit its staff writes
	// AFTER Complete has ended the active.group and stamped the instance completed —
	// leaving a completed block with post-completion staffing state (a rewritten
	// historical roster, even a fresh open supervisor row on an already-closed
	// group). Advisory xact locks are re-entrant; reload under the lock so a
	// concurrent move/cancel/complete is observed before we act (#1840).
	lockedDate := instance.Date
	if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenant.FromContext(ctx), lockedDate)); err != nil {
		return nil, &ScheduleError{Op: "complete instance: lock day", Err: err}
	}
	instance, err = s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// A concurrent PUT may have MOVED the block to another day while we waited on
	// the lock keyed on lockedDate; the held lock no longer covers the block's
	// real day, so ending the bridge now would race staffing saves on that day.
	// Reject so the client reopens it on its new day (#1840).
	if instance.Date != lockedDate {
		return nil, ErrInstanceMoved
	}

	if instance.Status != scheduleModel.InstanceStatusActive {
		return nil, fmt.Errorf("%w: cannot complete instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	if err := s.validateCompleteTime(ctx, instance, s.now()); err != nil {
		return nil, err
	}
	if instance.ActiveGroupID == nil {
		// An active instance without a bridge shouldn't exist post WP-B9.
		// Treat as data corruption — abort the tx rather than silently
		// "completing" an instance that never actually ran.
		return nil, &ScheduleError{Op: "complete instance", Err: fmt.Errorf("active instance %d has no active_group_id", instance.ID)}
	}

	// Serialize check-in (CreateVisit locks the group) and checkout (visit
	// row locks) with this snapshot so recovery restores the rows that
	// EndActivitySession actually closes.
	lockedGroup, err := s.deps.ActiveGroupRepo.FindByIDForUpdate(ctx, *instance.ActiveGroupID)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: lock group", Err: err}
	}
	if lockedGroup == nil || lockedGroup.EndTime != nil {
		return nil, fmt.Errorf("%w: active group is not open", ErrInvalidInstanceTransition)
	}
	if s.deps.RecoveryRepo != nil {
		if err := s.deps.RecoveryRepo.LockOpenVisits(ctx, *instance.ActiveGroupID); err != nil {
			return nil, &ScheduleError{Op: "complete instance: lock visits", Err: err}
		}
	}

	visitsBefore, err := s.deps.VisitRepo.FindByActiveGroupID(ctx, *instance.ActiveGroupID)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: snapshot visits", Err: err}
	}
	supervisorsBefore, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, *instance.ActiveGroupID, true)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: snapshot supervisors", Err: err}
	}
	attendanceBefore, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: snapshot attendance", Err: err}
	}
	snapshot := scheduleModel.ActivityCompletionSnapshot{ActiveGroupID: *instance.ActiveGroupID}
	for _, visit := range visitsBefore {
		if visit.ExitTime == nil {
			snapshot.VisitIDs = append(snapshot.VisitIDs, visit.ID)
		}
	}
	if confirmed, required := ctx.Value(lifecycleConfirmedStudentsKey).([]int64); required {
		actual := make([]int64, 0, len(snapshot.VisitIDs))
		for _, visit := range visitsBefore {
			if visit.ExitTime == nil {
				actual = append(actual, visit.StudentID)
			}
		}
		slices.Sort(confirmed)
		slices.Sort(actual)
		if !slices.Equal(confirmed, actual) {
			return nil, ErrCompletionConfirmationStale
		}
	}
	for _, supervisor := range supervisorsBefore {
		snapshot.SupervisorIDs = append(snapshot.SupervisorIDs, supervisor.ID)
	}
	for _, row := range attendanceBefore {
		snapshot.Attendance = append(snapshot.Attendance, scheduleModel.CompletionAttendanceSnapshot{RowID: row.ID, Status: row.Status, Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt, NotScheduled: row.NotScheduled})
	}
	instance.CompletionSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: encode snapshot", Err: err}
	}

	// Mark any remaining expected students as absent before ending the active
	// group. Runs inside the caller's tenant tx — if EndActivitySession or
	// updateLifecycleColumns fail below, the bulk update rolls back too, so
	// the instance never leaves the tx in a half-finished state.
	//
	// Children who are not booked into care today are left alone (#1747): they
	// were never expected, so "absent" would claim they failed to show up to
	// care they were not booked for. A cancelled day still gets its absence.
	notScheduled, err := s.notScheduledStudentIDs(ctx, instance)
	if err != nil {
		return nil, err
	}
	// Persist WHY those rows are spared before sparing them. Without the
	// marker the spared rows are indistinguishable from ordinary expected
	// rows, and every later writer of `status` — the attendance PATCH, a sick
	// report — would silently create or destroy the fact (#1747 review).
	refs := make([]scheduleModel.StudentInstanceRef, 0, len(notScheduled))
	for _, studentID := range notScheduled {
		refs = append(refs, scheduleModel.StudentInstanceRef{
			StudentID:  studentID,
			InstanceID: instance.ID,
		})
	}
	if err := s.deps.InstanceStudents.MarkNotScheduled(ctx, refs); err != nil {
		return nil, &ScheduleError{Op: "complete instance: mark not scheduled", Err: err}
	}
	if _, err := s.deps.InstanceStudents.BulkUpdateStatus(
		ctx, instance.ID, scheduleModel.AttendanceStatusExpected, scheduleModel.AttendanceStatusAbsent, notScheduled,
	); err != nil {
		return nil, &ScheduleError{Op: "complete instance: mark absent", Err: err}
	}

	if err := s.deps.ActiveService.EndActivitySession(ctx, *instance.ActiveGroupID); err != nil {
		return nil, &ScheduleError{Op: "complete instance: end active.group", Err: err}
	}

	now := s.now()
	instance.Status = scheduleModel.InstanceStatusCompleted
	instance.CompletedAt = &now
	instance.ReopenUntil = ptrTo(now.Add(5 * time.Minute))
	completedByAccountID, _ := ctx.Value(lifecycleActorKey).(int64)
	if completedByAccountID > 0 {
		instance.CompletedBy = ptrTo(completedByAccountID)
	}
	if err := s.updateLifecycleColumns(ctx, instance, "status", "completed_at", "completed_by", "reopen_until", "completion_snapshot"); err != nil {
		return nil, &ScheduleError{Op: "complete instance: update", Err: err}
	}

	s.broadcastInstanceEvent(ctx, realtime.EventInstanceCompleted, instance, nil, nil)
	return instance, nil
}

func ptrTo[T any](value T) *T { return &value }

// Reopen restores the exact live group, visits, supervisors and attendance
// captured immediately before completion. The day lock serializes this with
// lifecycle and staffing writes; any conflict aborts the tenant transaction.
func (s *instanceService) Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*StartInstanceResult, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	lockedDate := instance.Date
	if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenant.FromContext(ctx), lockedDate)); err != nil {
		return nil, &ScheduleError{Op: "reopen instance: lock day", Err: err}
	}
	instance, err = s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Date != lockedDate {
		return nil, ErrInstanceMoved
	}
	if instance.Status != scheduleModel.InstanceStatusCompleted || instance.ReopenUntil == nil || s.now().After(*instance.ReopenUntil) {
		return nil, fmt.Errorf("%w: reopen window expired", ErrInvalidInstanceTransition)
	}
	if !isAdmin && (instance.CompletedBy == nil || *instance.CompletedBy != accountID) {
		return nil, ErrTimetableOperationForbidden
	}
	var snapshot scheduleModel.ActivityCompletionSnapshot
	if len(instance.CompletionSnapshot) == 0 || json.Unmarshal(instance.CompletionSnapshot, &snapshot) != nil {
		return nil, fmt.Errorf("%w: completion snapshot missing", ErrInvalidInstanceTransition)
	}
	if err := s.validateReopenOccupancy(ctx, instance, snapshot); err != nil {
		return nil, err
	}
	if err := s.validateReopenAttendanceUnchanged(ctx, instance); err != nil {
		return nil, err
	}
	closedVisits, err := s.deps.VisitRepo.FindByActiveGroupID(ctx, snapshot.ActiveGroupID)
	if err != nil {
		return nil, &ScheduleError{Op: "reopen instance: load visits", Err: err}
	}
	studentByVisit := make(map[int64]int64, len(closedVisits))
	for _, visit := range closedVisits {
		studentByVisit[visit.ID] = visit.StudentID
	}
	for _, visitID := range snapshot.VisitIDs {
		studentID := studentByVisit[visitID]
		if studentID <= 0 {
			return nil, fmt.Errorf("%w: snapshot visit missing", ErrInvalidInstanceTransition)
		}
		current, findErr := s.deps.VisitRepo.GetCurrentByStudentID(ctx, studentID)
		if findErr != nil && !modelBase.IsNoRows(findErr) {
			return nil, findErr
		}
		if current != nil {
			return nil, fmt.Errorf("%w: student %d already has an active visit", ErrTimetableOperationConflict, studentID)
		}
	}
	if err := s.deps.RecoveryRepo.Restore(ctx, instance.ID, snapshot, s.now()); err != nil {
		return nil, &ScheduleError{Op: "reopen instance: restore snapshot", Err: err}
	}
	instance.Status = scheduleModel.InstanceStatusActive
	instance.ActiveGroupID = ptrTo(snapshot.ActiveGroupID)
	instance.CompletedAt, instance.CompletedBy, instance.ReopenUntil = nil, nil, nil
	instance.CompletionSnapshot = nil
	s.broadcastInstanceEvent(ctx, realtime.EventInstanceStarted, instance, nil, nil)
	return &StartInstanceResult{Instance: instance, ActiveGroupID: snapshot.ActiveGroupID, Warnings: []InstanceConflictWarning{}}, nil
}

// CanReopenInstance is the actor-aware reopen gate the list payload exposes
// so clients can hide the action when the five-minute window, snapshot, or
// actor check would reject it.
func CanReopenInstance(instance *scheduleModel.ActivityInstance, accountID int64, isAdmin bool, now time.Time) bool {
	if instance == nil || instance.Status != scheduleModel.InstanceStatusCompleted {
		return false
	}
	if instance.ReopenUntil == nil || now.After(*instance.ReopenUntil) {
		return false
	}
	if len(instance.CompletionSnapshot) == 0 {
		return false
	}
	if isAdmin {
		return true
	}
	return instance.CompletedBy != nil && *instance.CompletedBy == accountID
}

func (s *instanceService) validateReopenOccupancy(ctx context.Context, instance *scheduleModel.ActivityInstance, snapshot scheduleModel.ActivityCompletionSnapshot) error {
	hasConflict, _, err := s.deps.ActiveGroupRepo.CheckRoomConflict(ctx, instance.RoomID, snapshot.ActiveGroupID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: check room", Err: err}
	}
	if hasConflict {
		return activeSvc.ErrRoomConflict
	}
	if len(snapshot.VisitIDs) == 0 {
		return nil
	}
	room, err := s.deps.RoomRepo.FindByIDForUpdate(ctx, instance.RoomID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: lock room", Err: err}
	}
	if room == nil {
		return &ScheduleError{Op: "reopen instance: lock room", Err: fmt.Errorf("room %d not found", instance.RoomID)}
	}
	if room.Capacity == nil || *room.Capacity <= 0 {
		return nil
	}
	currentOccupancy, err := s.deps.VisitRepo.CountActiveByRoomID(ctx, instance.RoomID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: count room occupancy", Err: err}
	}
	if currentOccupancy+len(snapshot.VisitIDs) > *room.Capacity {
		return activeSvc.ErrRoomCapacityExceeded
	}
	return nil
}

func (s *instanceService) validateReopenAttendanceUnchanged(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	if s.deps.RecoveryRepo != nil {
		if err := s.deps.RecoveryRepo.LockAttendance(ctx, instance.ID); err != nil {
			return &ScheduleError{Op: "reopen instance: lock attendance", Err: err}
		}
	}
	if instance.CompletedAt == nil {
		return nil
	}
	rows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: load attendance", Err: err}
	}
	for _, row := range rows {
		if row.GetUpdatedAt().After(*instance.CompletedAt) {
			return fmt.Errorf("%w: attendance changed after completion", ErrTimetableOperationConflict)
		}
	}
	return nil
}

// Cancel implements planned|active → cancelled. From active, the bridge is
// ended the same way Complete does (visits + supervisors close, checkout
// events fire). From planned there is no bridge yet; just stamp the status.
func (s *instanceService) Cancel(ctx context.Context, instanceID int64, reason *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	// Serialize the cancellation against concurrent day-wide staffing saves
	// (/substitute, /deviations) on the block's day. Both those endpoints take
	// this (tenant, date) advisory lock before they classify and rewrite staff
	// rows. Cancel is a public route in its own right (POST /instances/{id}/cancel
	// calls this service directly, not only the /deviations cancel branch); without
	// the same lock a direct cancel can commit between another admin's lock
	// acquisition and their staff-row writes, leaving a cancelled block with
	// freshly-rewritten staffing — a rewritten historical block. Advisory xact
	// locks are re-entrant, so the /deviations cancel branch (which already holds
	// this lock) re-acquires it harmlessly. Reload under the lock so a concurrent
	// move/complete/cancel is observed before we act (#1840).
	lockedDate := instance.Date
	if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenant.FromContext(ctx), lockedDate)); err != nil {
		return nil, &ScheduleError{Op: "cancel instance: lock day", Err: err}
	}
	instance, err = s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// A concurrent PUT may have MOVED the block to another day while we waited on
	// the lock keyed on lockedDate. The direct route POST /instances/{id}/cancel
	// reaches this method without the /deviations branch's own move guard, so
	// cancelling now would stamp the moved block while holding only the stale
	// day's lock and break the ascending day-lock ordering. Reject so the client
	// reopens it on its new day (#1840). The /deviations cancel branch already
	// rejected a move before delegating, so this never fires a second time there.
	if instance.Date != lockedDate {
		return nil, ErrInstanceMoved
	}

	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusActive:
		// allowed
	default:
		return nil, fmt.Errorf("%w: cannot cancel instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}

	if instance.Status == scheduleModel.InstanceStatusActive {
		if instance.ActiveGroupID == nil {
			// Mirrors the Complete branch: an active instance without a bridge
			// is data corruption, not a normal state. Aborting here is safer
			// than silently "cancelling" a session that never actually ran —
			// a staff member downstream might have assumed the visits were
			// closed for them.
			return nil, &ScheduleError{Op: "cancel instance", Err: fmt.Errorf("active instance %d has no active_group_id", instance.ID)}
		}
		if err := s.deps.ActiveService.EndActivitySession(ctx, *instance.ActiveGroupID); err != nil {
			return nil, &ScheduleError{Op: "cancel instance: end active.group", Err: err}
		}
	}

	previousStatus := instance.Status
	now := time.Now()
	instance.Status = scheduleModel.InstanceStatusCancelled
	instance.CompletedAt = &now
	instance.CancelReason = reason
	if err := s.updateLifecycleColumns(ctx, instance, "status", "completed_at", "cancel_reason"); err != nil {
		return nil, &ScheduleError{Op: "cancel instance: update", Err: err}
	}
	if err := s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      auditModel.DeviationEventCancellation,
		oldValue:       map[string]any{"status": previousStatus},
		newValue:       map[string]any{"status": scheduleModel.InstanceStatusCancelled},
		reason:         reason,
		actorAccountID: actorAccountID,
	}); err != nil {
		return nil, err
	}

	s.broadcastInstanceEvent(ctx, realtime.EventInstanceCancelled, instance, nil, nil)
	return instance, nil
}

// SetUnderstaffedAck flips the "deliberately unstaffed" flag (issue #1840). It
// is a planning annotation, not a lifecycle transition: no active.group is
// touched and no SSE lifecycle event fires. Only planned/active instances can
// carry the flag — acknowledging a completed or cancelled block is meaningless
// and returns ErrInvalidInstanceTransition (→ 409). Clearing the flag (ack=
// false) also clears the note so a stale reason cannot linger.
func (s *instanceService) SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error) {
	return s.setUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID, true)
}

// setUnderstaffedAck implements SetUnderstaffedAck. logEvent=false is the
// re-plan reapply path: reattaching a surviving acknowledgement is not a state
// change, so it writes no protocol entry (#1886, "log only losses").
func (s *instanceService) setUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64, logEvent bool) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusActive:
		// allowed
	default:
		return nil, fmt.Errorf("%w: cannot acknowledge understaffing on instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}

	// The "deliberately unstaffed" flag records that at least one planned
	// position is deliberately left unfilled (#1840). It is valid whenever the
	// block ends up understaffed — fewer people present than planned, or nobody
	// at all — so a single absent-without-replacement position can be
	// acknowledged while other staff remain. Only a fully staffed block (present
	// >= planned) rejects it. Clearing the flag (ack=false) is always allowed.
	if ack {
		rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
		if err != nil {
			return nil, &ScheduleError{Op: "set understaffed ack: load staff", Err: err}
		}
		if !IsUnderstaffed(rows) {
			return nil, ErrUnderstaffedAckStillStaffed
		}
	}

	previousAck := instance.UnderstaffedAck
	previousNote := instance.UnderstaffedNote
	instance.UnderstaffedAck = ack
	if ack {
		instance.UnderstaffedNote = note
	} else {
		instance.UnderstaffedNote = nil
	}
	if err := s.updateLifecycleColumns(ctx, instance, "understaffed_ack", "understaffed_note"); err != nil {
		return nil, &ScheduleError{Op: "set understaffed ack: update", Err: err}
	}
	// Protocol the flag change (#1886); an idempotent replay (same ack, same
	// note) still wrote the columns above but records no event to keep the
	// Verlauf free of no-op noise.
	if logEvent && (previousAck != ack || !equalOptionalString(previousNote, instance.UnderstaffedNote)) {
		eventType := auditModel.DeviationEventUnderstaffedAck
		if !ack {
			eventType = auditModel.DeviationEventUnderstaffedUnack
		}
		if err := s.logDeviationEvent(ctx, deviationEventInput{
			instance:       instance,
			eventType:      eventType,
			oldValue:       understaffedAckValue(previousAck, previousNote),
			newValue:       understaffedAckValue(ack, instance.UnderstaffedNote),
			reason:         instance.UnderstaffedNote,
			actorAccountID: actorAccountID,
		}); err != nil {
			return nil, err
		}
	}
	return instance, nil
}

// understaffedAckValue builds the old/new JSONB shape for ack events.
func understaffedAckValue(ack bool, note *string) map[string]any {
	v := map[string]any{"understaffed_ack": ack}
	if note != nil {
		v["note"] = *note
	}
	return v
}

// equalOptionalString compares two optional strings by value.
func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// DeleteCancelled permanently removes a planned or cancelled instance.
// Historical name retained for compatibility with existing handlers. Active
// and completed instances stay protected: deleting those would hide live
// sessions or attendance history. For materialized template occurrences,
// write a cancelled activity_exception first so materialization cannot
// resurrect the deleted single occurrence.
func (s *instanceService) DeleteCancelled(ctx context.Context, instanceID int64) error {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return err
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusCancelled:
		// allowed
	default:
		return fmt.Errorf("%w: cannot delete instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}

	if instance.ActivityGroupID != nil && !instance.IsSpontaneous {
		if err := s.rejectAmbiguousTemplateDelete(ctx, *instance.ActivityGroupID, instance.Date); err != nil {
			return err
		}
		if err := s.ensureCancelledSlotException(ctx, *instance.ActivityGroupID, instance.Date, deletedSlotReason); err != nil {
			return err
		}
	}

	if err := s.deps.InstanceRepo.Delete(ctx, instance.ID); err != nil {
		return &ScheduleError{Op: "delete instance", Err: err}
	}
	s.getLogger().Info("instance deleted",
		slog.Int64("tenant_id", tenant.FromContext(ctx)),
		slog.Int64("instance_id", instance.ID),
		slog.String("date", instance.Date.String()),
		slog.String("status", instance.Status),
	)
	s.broadcastPlannedInstanceChanged(ctx, "instance_delete")
	return nil
}

func (s *instanceService) rejectAmbiguousTemplateDelete(ctx context.Context, activityGroupID int64, date timezone.Date) error {
	rows, err := s.deps.InstanceRepo.FindByActivityGroupAndDate(ctx, activityGroupID, date)
	if err != nil {
		return &ScheduleError{Op: "delete instance: check same-day template slots", Err: err}
	}
	templateBacked := 0
	for _, row := range rows {
		if row != nil && !row.IsSpontaneous {
			templateBacked++
		}
	}
	if templateBacked > 1 {
		return fmt.Errorf("%w: template has %d same-day slots", ErrAmbiguousTemplateInstanceDelete, templateBacked)
	}
	return nil
}

// Create inserts a new activity instance and optionally pre-assigns staff.
// Spontaneity is derived from the absence of an ActivityGroupID — a value
// of nil means there is no template binding, so is_spontaneous is set to
// true. Conflict detection is intentionally not run here; the read-side
// of the planner surfaces conflicts on the response, and surfacing them
// in the create path would block admins from creating overlapping rows
// they may explicitly want (e.g. parallel offers in different rooms).
func (s *instanceService) Create(ctx context.Context, req CreateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "create instance", Err: errors.New("no tenant in context")}
	}
	if !s.hasTx(ctx) {
		var created *scheduleModel.ActivityInstance
		err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var err error
			created, err = s.Create(txCtx, req)
			return err
		})
		return created, err
	}
	if err := s.lockRecurrenceThenGradeTransitions(ctx, "create instance"); err != nil {
		return nil, err
	}
	if err := s.validateInstanceReferences(ctx, req.Date, req.RoomID, req.ActivityGroupID, req.StaffIDs, req.StudentIDs, req.CreatedByStaffID); err != nil {
		return nil, &ScheduleError{Op: "create instance: validate references", Err: err}
	}

	isSpontaneous := req.ActivityGroupID == nil
	if req.IsSpontaneous != nil {
		isSpontaneous = *req.IsSpontaneous
	}

	inst := &scheduleModel.ActivityInstance{
		Date:            req.Date,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		Title:           req.Title,
		Description:     req.Description,
		Notes:           req.Notes,
		RoomID:          req.RoomID,
		ActivityGroupID: req.ActivityGroupID,
		RequiredStaff:   req.RequiredStaff,
		ListKind:        req.ListKind,
		Status:          scheduleModel.InstanceStatusPlanned,
		IsSpontaneous:   isSpontaneous,
		CreatedBy:       req.CreatedByStaffID,
	}
	inst.SetTenantID(tenantID)

	if err := s.deps.InstanceRepo.Create(ctx, inst); err != nil {
		return nil, &ScheduleError{Op: "create instance: insert", Err: err}
	}

	for _, staffID := range sliceutil.UniquePositive(req.StaffIDs) {
		if staffID <= 0 {
			continue
		}
		row := &scheduleModel.InstanceStaff{
			InstanceID: inst.ID,
			StaffID:    staffID,
			// RoomID nil → uses instance.RoomID at runtime
			IsPrimary: false,
		}
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStaffRepo.Create(ctx, row); err != nil {
			return nil, &ScheduleError{Op: "create instance: assign staff", Err: err}
		}
	}
	newStudentIDs := sliceutil.UniquePositive(req.StudentIDs)
	if err := s.lockCareExceptionDaysForStudents(ctx, newStudentIDs, inst.Date); err != nil {
		return nil, err
	}
	for _, studentID := range newStudentIDs {
		if studentID <= 0 {
			continue
		}
		row := &scheduleModel.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  studentID,
			Status:     scheduleModel.AttendanceStatusExpected,
		}
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStudents.Create(ctx, row); err != nil {
			return nil, &ScheduleError{Op: "create instance: assign student", Err: err}
		}
	}
	if _, err := s.deps.InstanceStudents.ApplyActiveStatusDaysForInstance(ctx, inst.ID, inst.Date); err != nil {
		return nil, &ScheduleError{Op: "create instance: apply student status days", Err: err}
	}
	if _, err := s.deps.InstanceStudents.ApplyActivePartialAbsencesForInstance(ctx, inst.ID, inst.Date); err != nil {
		return nil, &ScheduleError{Op: "create instance: apply student partial absences", Err: err}
	}

	s.getLogger().Info("instance created",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("instance_id", inst.ID),
		slog.String("date", inst.Date.String()),
		slog.Bool("spontaneous", inst.IsSpontaneous),
		slog.Int("staff_assigned", len(req.StaffIDs)),
	)
	s.broadcastPlannedInstanceChanged(ctx, "instance_create")

	return inst, nil
}

func (s *instanceService) UpdatePlanned(ctx context.Context, instanceID int64, req UpdateInstanceInput, actorAccountID *int64) (*scheduleModel.ActivityInstance, error) {
	if !s.hasTx(ctx) {
		var updated *scheduleModel.ActivityInstance
		err := tenant.WithTenantTx(ctx, s.deps.DB, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			var err error
			updated, err = s.UpdatePlanned(txCtx, instanceID, req, actorAccountID)
			return err
		})
		return updated, err
	}

	// Both tenant gates BEFORE the day locks below: this edit rewrites the
	// block's roster, so it must not interleave with a grade transition's
	// graduation + roster-archive pass, and the recurrence-gate-first order is
	// what keeps that acquisition acyclic against the day locks (see the helper).
	if err := s.lockRecurrenceThenGradeTransitions(ctx, "update instance"); err != nil {
		return nil, err
	}

	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot update instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	// Serialize with the day-wide staffing endpoints (#1840). This edit rewrites
	// the block's assignments and may MOVE it to another date, so it must hold the
	// same (tenant, date) advisory lock the /deviations and /substitute saves take
	// — for BOTH the current and (when moved) the new date. Without it a concurrent
	// deviation save that read this block's old date before the move could mark the
	// wrong day's rows absent while the moved block itself is left untouched. Taken
	// in ascending date order so re-plan / PUT / deviation contention on shared
	// days can never deadlock.
	tenantID := tenant.FromContext(ctx)
	lockedDate := instance.Date
	if err := s.acquireSubstituteDayLockPair(ctx, tenantID, lockedDate, req.Date); err != nil {
		return nil, &ScheduleError{Op: "update instance: lock day", Err: err}
	}

	// Reload and re-validate under the lock. The pre-lock read above may be stale:
	// while we waited on the day-lock pair, a concurrent cancel/start/complete
	// could have committed (leaving the block cancelled/active — no longer
	// editable), or another PUT could have MOVED it to a day we do not hold the
	// lock for. Mutating the pre-lock instance would rewrite date/staff rows on a
	// block that is now historical, cancelled, or on a different day. Re-read and
	// re-check status and date before applying anything (#1840).
	instance, err = s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Date != lockedDate {
		return nil, ErrInstanceMoved
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot update instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	if err := validateLegacyWeekendInstanceDate(instance.Date, req.Date); err != nil {
		return nil, err
	}

	if err := s.validateInstanceReferences(ctx, req.Date, req.RoomID, req.ActivityGroupID, req.StaffIDs, req.StudentIDs, nil); err != nil {
		return nil, &ScheduleError{Op: "update instance: validate references", Err: err}
	}

	// Capture the slot the instance occupied BEFORE mutation: if the edit
	// moves a template-backed occurrence (date or start_time change), the
	// original (group, date, start) key becomes vacant and a later
	// materialization run would re-create it as a duplicate. consumeMovedSlot
	// writes a cancelled activity_exception for the original date so the
	// materializer treats that slot as consumed (skipped_exception).
	origSlot := capturedSlot{
		ActivityGroupID: instance.ActivityGroupID,
		Date:            instance.Date,
		StartHHMMSS:     formatTimeOfDay(instance.StartTime),
	}

	instance.Date = req.Date
	instance.StartTime = req.StartTime
	instance.EndTime = req.EndTime
	instance.Title = req.Title
	instance.Description = req.Description
	instance.Notes = req.Notes
	instance.RoomID = req.RoomID
	instance.ActivityGroupID = req.ActivityGroupID
	instance.RequiredStaff = req.RequiredStaff
	instance.ListKind = req.ListKind
	instance.IsSpontaneous = req.ActivityGroupID == nil
	columns := []string{
		"date",
		"start_time",
		"end_time",
		"title",
		"description",
		"notes",
		"room_id",
		"activity_group_id",
		"required_staff",
		"list_kind",
		"is_spontaneous",
	}
	if req.CalendarPeriodID != nil {
		instance.CalendarPeriodID = req.CalendarPeriodID
		columns = append(columns, "calendar_period_id")
	}

	if err := s.updateLifecycleColumns(ctx, instance, columns...); err != nil {
		return nil, &ScheduleError{Op: "update instance", Err: err}
	}
	if err := s.consumeMovedSlot(ctx, origSlot, req); err != nil {
		return nil, err
	}
	if err := s.replaceInstanceAssignments(ctx, instance, req.StaffIDs, req.StudentIDs, actorAccountID); err != nil {
		return nil, err
	}

	// Vertretungsplan integrity (#1840): a lingering "deliberately unstaffed"
	// acknowledgement must be cleared only when this edit left the block fully
	// staffed — a partially-covered block (e.g. two planned positions, one still
	// absent) stays understaffed and must keep its acknowledgement, or an
	// unrelated title/room edit would silently reopen an intentionally
	// acknowledged gap.
	if err := s.clearStaleAckIfStaffed(ctx, instance, actorAccountID); err != nil {
		return nil, err
	}

	s.broadcastPlannedInstanceChanged(ctx, "instance_update")
	return instance, nil
}

func validateLegacyWeekendInstanceDate(existing, requested timezone.Date) error {
	if requested.Weekday() != time.Saturday && requested.Weekday() != time.Sunday {
		return nil
	}
	if existing == requested {
		return nil
	}
	return ErrInstanceWeekend
}

// clearStaleAckIfStaffed clears a lingering "deliberately unstaffed"
// acknowledgement on `instance` ONLY when its current instance_staff rows leave
// the block fully staffed (present >= planned). A still-understaffed block keeps
// the acknowledgement: partial coverage must not silently reopen an
// intentionally acknowledged gap, and the amber card would otherwise contradict
// /gaps (#1840). This is the same IsUnderstaffed rule SetUnderstaffedAck
// enforces at set time; it writes only when it actually clears the flag.
func (s *instanceService) clearStaleAckIfStaffed(ctx context.Context, instance *scheduleModel.ActivityInstance, actorAccountID *int64) error {
	if !instance.UnderstaffedAck {
		return nil
	}
	rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "clear stale ack: load staff", Err: err}
	}
	if IsUnderstaffed(rows) {
		return nil // still short-staffed → keep the acknowledgement
	}
	previousNote := instance.UnderstaffedNote
	instance.UnderstaffedAck = false
	instance.UnderstaffedNote = nil
	if err := s.updateLifecycleColumns(ctx, instance, "understaffed_ack", "understaffed_note"); err != nil {
		return &ScheduleError{Op: "clear stale ack: update", Err: err}
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      auditModel.DeviationEventUnderstaffedUnack,
		oldValue:       understaffedAckValue(true, previousNote),
		newValue:       understaffedAckValue(false, nil),
		actorAccountID: actorAccountID,
	})
}

// ClearUnderstaffedAckIfStaffed loads the instance and clears a lingering
// understaffed acknowledgement only when its staff rows now leave it fully
// staffed. The /substitute flow calls this after adding coverage: a single
// replacement on a block with several open positions must not reopen an
// acknowledged gap (#1840).
func (s *instanceService) ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return err
	}
	return s.clearStaleAckIfStaffed(ctx, instance, actorAccountID)
}

// replaceInstanceAssignments wipes and re-creates the instance's staff and
// student rows from the request lists. Extracted from UpdatePlanned to keep
// its cognitive complexity in check.
//
// Vertretungsplan integrity (#1840): the Betreuungsplan editor sends every
// staff row as a plain staff_id, so a blind wipe-and-recreate would discard the
// per-row metadata. Two kinds of metadata need different treatment:
//
//   - is_primary and the room_id override are PLANNED attributes the editor
//     cannot re-express (it only sends staff_ids), so they are always carried
//     forward for a staff member who is still present — losing them would leave
//     the block without a primary or reset a Lernzeit room split.
//   - the DEVIATION state (is_substitute / is_absent / absence_reason) is only
//     carried forward when the roster MEMBERSHIP is unchanged (a title/room/
//     student-only edit): keeping it then is what stops an unrelated edit from
//     turning absent staff and their substitutes back into ordinary present
//     staff. But when the staff set is intentionally changed here, the admin is
//     re-planning the base roster, so the prior deviations no longer describe it.
//     Carrying them onto the recreated rows would leave a regular staff member
//     stored as a substitute, corrupt the planned headcount, and make it
//     impossible to promote a former substitute back to planned staff through
//     this editor — so the deviation flags are dropped and every row is
//     recreated as plain planned staff (#1840, review follow-up).
func (s *instanceService) replaceInstanceAssignments(ctx context.Context, instance *scheduleModel.ActivityInstance, staffIDs, studentIDs []int64, actorAccountID *int64) error {
	instanceID := instance.ID
	prior, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return &ScheduleError{Op: "update instance: load existing staff", Err: err}
	}
	priorByStaff := make(map[int64]*scheduleModel.InstanceStaff, len(prior))
	for _, row := range prior {
		priorByStaff[row.StaffID] = row
	}

	// Roster membership is unchanged iff the new set equals the prior set
	// (staff_id is unique per instance, so a length + membership check suffices).
	newStaffIDs := sliceutil.UniquePositive(staffIDs)
	rosterUnchanged := len(newStaffIDs) == len(priorByStaff)
	if rosterUnchanged {
		for _, staffID := range newStaffIDs {
			if _, ok := priorByStaff[staffID]; !ok {
				rosterUnchanged = false
				break
			}
		}
	}

	// Änderungsprotokoll (#1886): a roster change deliberately drops the prior
	// deviation state (see doc comment above). Record what is being discarded —
	// one event per prior row that carried a deviation — so "warum ist die
	// Vertretung weg?" stays answerable after a roster edit.
	if !rosterUnchanged {
		for _, row := range prior {
			if !row.IsSubstitute && !row.IsAbsent {
				continue
			}
			dropped := map[string]any{
				"is_substitute": row.IsSubstitute,
				"is_absent":     row.IsAbsent,
			}
			if row.AbsenceReason != nil {
				dropped["reason"] = *row.AbsenceReason
			}
			if err := s.logDeviationEvent(ctx, deviationEventInput{
				instance:       instance,
				eventType:      auditModel.DeviationEventDroppedByEdit,
				subjectStaffID: &row.StaffID,
				oldValue:       dropped,
				actorAccountID: actorAccountID,
			}); err != nil {
				return err
			}
		}
	}

	// Care-day locks before any attendance row mutation. Partial-absence writers
	// take student → care-day then update rows; deleting/recreating rows first
	// while waiting for care-day deadlocks against that order.
	priorStudents, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return &ScheduleError{Op: "update instance: load existing students", Err: err}
	}
	lockStudentIDs := make([]int64, 0, len(priorStudents)+len(studentIDs))
	for _, row := range priorStudents {
		if row != nil && row.StudentID > 0 {
			lockStudentIDs = append(lockStudentIDs, row.StudentID)
		}
	}
	lockStudentIDs = append(lockStudentIDs, studentIDs...)
	if err := s.lockCareExceptionDaysForStudents(ctx, sliceutil.UniquePositive(lockStudentIDs), instance.Date); err != nil {
		return err
	}

	if err := s.deps.InstanceStaffRepo.DeleteByInstanceID(ctx, instanceID); err != nil {
		return &ScheduleError{Op: "update instance: clear staff", Err: err}
	}
	if err := s.deps.InstanceStudents.DeleteByInstanceID(ctx, instanceID); err != nil {
		return &ScheduleError{Op: "update instance: clear students", Err: err}
	}
	tenantID := tenant.FromContext(ctx)
	for _, staffID := range newStaffIDs {
		row := &scheduleModel.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
		if p := priorByStaff[staffID]; p != nil {
			row.IsPrimary = p.IsPrimary
			row.RoomID = p.RoomID
			if rosterUnchanged {
				row.IsSubstitute = p.IsSubstitute
				row.IsAbsent = p.IsAbsent
				row.AbsenceReason = p.AbsenceReason
			}
		}
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStaffRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "update instance: assign staff", Err: err}
		}
	}
	for _, studentID := range sliceutil.UniquePositive(studentIDs) {
		row := &scheduleModel.InstanceStudent{
			InstanceID: instanceID,
			StudentID:  studentID,
			Status:     scheduleModel.AttendanceStatusExpected,
		}
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStudents.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "update instance: assign student", Err: err}
		}
	}
	if _, err := s.deps.InstanceStudents.ApplyActiveStatusDaysForInstance(ctx, instanceID, instance.Date); err != nil {
		return &ScheduleError{Op: "update instance: apply student status days", Err: err}
	}
	if _, err := s.deps.InstanceStudents.ApplyActivePartialAbsencesForInstance(ctx, instanceID, instance.Date); err != nil {
		return &ScheduleError{Op: "update instance: apply student partial absences", Err: err}
	}
	return nil
}

// lockCareExceptionDaysForStudents takes student → care-day locks for every
// child on the instance date, sorted, matching partial-absence writers.
func (s *instanceService) lockCareExceptionDaysForStudents(
	ctx context.Context, studentIDs []int64, date timezone.Date,
) error {
	if len(studentIDs) == 0 || s.deps.DB == nil {
		return nil
	}
	sorted := append([]int64(nil), studentIDs...)
	slices.Sort(sorted)
	for _, studentID := range sorted {
		if err := LockCareExceptionDay(ctx, s.deps.DB, studentID, date); err != nil {
			return &ScheduleError{Op: "lock care exception day for roster rewrite", Err: err}
		}
	}
	return nil
}

// capturedSlot is the (template, date, start) key an instance occupied before
// an UpdatePlanned mutation. StartHHMMSS is the formatTimeOfDay string so the
// comparison is independent of the year anchor bun picks when scanning TIME
// columns (existing rows scan as year 0000, request values are anchored at
// 2000-01-01).
type capturedSlot struct {
	ActivityGroupID *int64
	Date            timezone.Date
	StartHHMMSS     string
}

// movedSlotReason is the audit hint stored on the auto-created exception so
// admins can tell it apart from manually entered cancellations.
const movedSlotReason = "Einzeltermin verschoben"

// deletedSlotReason is stored on per-date cancellation exceptions created by
// DeleteCancelled for a deleted materialized occurrence.
const deletedSlotReason = "Einzeltermin gelöscht"

// consumeMovedSlot writes a cancelled activity_exception for the original
// (template, date) when an UpdatePlanned call moved a template-backed
// instance to a different date or start time. Without it the original slot
// key is vacant again and the next materialization run re-creates the
// occurrence next to the moved one.
//
// Notes:
//   - Spontaneous instances (no activity_group_id before the edit) are never
//     re-materialized, so nothing is written for them.
//   - Exceptions are unique per (template, date) — a cancelled exception
//     consumes ALL of the template's slots on that date. For the common
//     one-slot-per-weekday template this is exact; on multi-slot days the
//     sibling slots' instances already exist (skipped_existing), so the only
//     collateral is that newly added schedule slots skip that single date.
//   - If an exception row already exists for the original date the slot is
//     already consumed (or deliberately modified by an admin); it is left
//     untouched and the situation is logged.
func (s *instanceService) consumeMovedSlot(ctx context.Context, orig capturedSlot, req UpdateInstanceInput) error {
	if orig.ActivityGroupID == nil {
		return nil // spontaneous before the edit — materialization never recreates it
	}
	if orig.Date == req.Date && orig.StartHHMMSS == formatTimeOfDay(req.StartTime) {
		return nil // slot key unchanged — nothing vacated
	}

	existing, err := s.deps.ExceptionRepo.FindByActivityGroupAndDate(ctx, *orig.ActivityGroupID, orig.Date)
	if err != nil {
		return &ScheduleError{Op: "update instance: check slot exception", Err: err}
	}
	if existing != nil {
		// Unique per (template, date): we cannot add a second row. A
		// cancelled exception already consumes the slot; a modified one was
		// authored deliberately — overwriting it would destroy admin data.
		s.getLogger().Warn("moved instance: exception already exists for original date, leaving it untouched",
			slog.Int64("activity_group_id", *orig.ActivityGroupID),
			slog.String("original_date", orig.Date.String()),
			slog.String("exception_type", existing.ExceptionType),
		)
		return nil
	}

	reason := movedSlotReason
	exc := &scheduleModel.ActivityException{
		ActivityGroupID: *orig.ActivityGroupID,
		ExceptionDate:   orig.Date,
		ExceptionType:   scheduleModel.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exc.SetTenantID(tenant.FromContext(ctx))
	if err := s.deps.ExceptionRepo.Create(ctx, exc); err != nil {
		return &ScheduleError{Op: "update instance: consume moved slot", Err: err}
	}
	s.getLogger().Info("moved instance: original slot consumed via cancelled exception",
		slog.Int64("activity_group_id", *orig.ActivityGroupID),
		slog.String("original_date", orig.Date.String()),
		slog.String("original_start", orig.StartHHMMSS),
		slog.String("new_date", req.Date.String()),
	)
	return nil
}

func (s *instanceService) ensureCancelledSlotException(ctx context.Context, activityGroupID int64, date timezone.Date, reason string) error {
	existing, err := s.deps.ExceptionRepo.FindByActivityGroupAndDate(ctx, activityGroupID, date)
	if err != nil {
		return &ScheduleError{Op: "delete instance: check slot exception", Err: err}
	}
	if existing != nil {
		if existing.ExceptionType == scheduleModel.ActivityExceptionCancelled {
			return nil
		}
		existing.ExceptionType = scheduleModel.ActivityExceptionCancelled
		existing.StartTime = nil
		existing.EndTime = nil
		existing.RoomID = nil
		existing.Reason = &reason
		if err := s.deps.ExceptionRepo.Update(ctx, existing); err != nil {
			return &ScheduleError{Op: "delete instance: cancel existing exception", Err: err}
		}
		s.getLogger().Info("deleted instance: existing exception converted to cancellation",
			slog.Int64("activity_group_id", activityGroupID),
			slog.String("date", date.String()),
		)
		return nil
	}

	exc := &scheduleModel.ActivityException{
		ActivityGroupID: activityGroupID,
		ExceptionDate:   date,
		ExceptionType:   scheduleModel.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exc.SetTenantID(tenant.FromContext(ctx))
	if err := s.deps.ExceptionRepo.Create(ctx, exc); err != nil {
		return &ScheduleError{Op: "delete instance: create cancellation exception", Err: err}
	}
	s.getLogger().Info("deleted instance: slot consumed via cancelled exception",
		slog.Int64("activity_group_id", activityGroupID),
		slog.String("date", date.String()),
	)
	return nil
}

// hasTx reports whether the caller already runs inside the tenant transaction
// the advisory gates need. False for direct service calls outside
// TenantTxMiddleware (CLI, tests), which then get their own transaction.
func (s *instanceService) hasTx(ctx context.Context) bool {
	if s.deps.DB == nil {
		return true // no DB wired (unit tests): nothing to lock, nothing to wrap
	}
	_, ok := modelBase.TxFromContext(ctx)
	return ok
}

// lockRecurrenceThenGradeTransitions takes the two tenant-wide gates that every
// writer of recurrence-derived roster state holds, in the project-wide order:
// recurrence FIRST, grade transitions second (see
// education.TenantTransitionsLockKey).
//
// The transitions gate is what makes the manual planner writes safe against a
// concurrent graduation. Create and UpdatePlanned insert instance_students rows
// from a student set the request validated; a grade transition committing its
// graduation and its roster-archive pass in between would leave the departed
// child back on an upcoming roster with nothing left to remove them — the same
// race materialization already closes this way (#405 review).
//
// Taking the recurrence gate up front also keeps the acquisition order acyclic
// against the per-day substitute locks: every path that holds BOTH a tenant gate
// and a day lock (re-plan, and these two) takes the recurrence gate first, while
// the day-lock-only paths (/deviations, /substitute, move-staff) never wait on a
// gate at all.
func (s *instanceService) lockRecurrenceThenGradeTransitions(ctx context.Context, op string) error {
	if s.deps.DB == nil {
		return nil
	}
	if err := lockTenantRecurrenceWrites(ctx, s.deps.DB); err != nil {
		return &ScheduleError{Op: op + ": lock recurrence", Err: err}
	}
	if err := lockTenantGradeTransitions(ctx, s.deps.DB); err != nil {
		return &ScheduleError{Op: op + ": lock grade transitions", Err: err}
	}
	return nil
}

// validateInstanceReferences checks every FK the caller supplied against the
// current tenant. `date` is the date the instance's rows will LIVE on (the
// request's date, i.e. the target date of a move), because it decides whether a
// graduated student is still rejected — see the alumnus gate below.
func (s *instanceService) validateInstanceReferences(
	ctx context.Context,
	date timezone.Date,
	roomID int64,
	activityGroupID *int64,
	staffIDs []int64,
	studentIDs []int64,
	createdByStaffID *int64,
) error {
	if roomID <= 0 {
		return fmt.Errorf("%w: invalid room_id", ErrInvalidInstanceReference)
	}
	if room, err := s.deps.RoomRepo.FindByID(ctx, roomID); err != nil || room == nil {
		if err != nil && !modelBase.IsNoRows(err) {
			return fmt.Errorf("validate room_id: %w", err)
		}
		return fmt.Errorf("%w: invalid room_id", ErrInvalidInstanceReference)
	}

	if activityGroupID != nil {
		if *activityGroupID <= 0 {
			return fmt.Errorf("%w: invalid activity_group_id", ErrInvalidInstanceReference)
		}
		group, err := s.deps.ActivityGroupRepo.FindByID(ctx, *activityGroupID)
		if err != nil || group == nil {
			if err != nil && !modelBase.IsNoRows(err) {
				return fmt.Errorf("validate activity_group_id: %w", err)
			}
			return fmt.Errorf("%w: invalid activity_group_id", ErrInvalidInstanceReference)
		}
	}

	uniqueStaffIDs := sliceutil.UniquePositive(staffIDs)
	if len(uniqueStaffIDs) > 0 {
		found, err := s.deps.StaffRepo.FindByIDs(ctx, uniqueStaffIDs)
		if err != nil {
			return fmt.Errorf("validate staff_ids: %w", err)
		}
		if len(found) != len(uniqueStaffIDs) {
			return fmt.Errorf("%w: invalid staff_ids", ErrInvalidInstanceReference)
		}
	}

	if createdByStaffID != nil {
		if *createdByStaffID <= 0 {
			return fmt.Errorf("%w: invalid created_by_staff_id", ErrInvalidInstanceReference)
		}
		staff, err := s.deps.StaffRepo.FindByID(ctx, *createdByStaffID)
		if err != nil || staff == nil {
			if err != nil && !modelBase.IsNoRows(err) {
				return fmt.Errorf("validate created_by_staff_id: %w", err)
			}
			return fmt.Errorf("%w: invalid created_by_staff_id", ErrInvalidInstanceReference)
		}
	}

	uniqueStudentIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueStudentIDs) > 0 {
		found, err := s.deps.StudentRepo.FindByIDs(ctx, uniqueStudentIDs)
		if err != nil {
			return fmt.Errorf("validate student_ids: %w", err)
		}
		if len(found) != len(uniqueStudentIDs) {
			return fmt.Errorf("%w: invalid student_ids", ErrInvalidInstanceReference)
		}
		// A graduated (alumnus) student is soft-deleted, and FindByIDs is
		// unfiltered. Materialization already refuses to copy them onto new
		// rosters, but these MANUAL paths write instance_students directly: a
		// planner form opened before the graduation and saved afterwards — or any
		// direct request carrying the id — would recreate exactly the roster rows
		// apply archived, and slot lists, staffing ratios and exports would count
		// the departed child again. The caller holds the grade-transition gate, so
		// this status cannot flip under us (#405 review).
		//
		// The gate stops at the same boundary the graduation's archive pass uses
		// (RemoveStudentsFromFutureRosters: today or later). A past-dated roster is
		// frozen history — the archive left the graduate's row there and the roster
		// READ still shows them — so rejecting the write would make such a block
		// uneditable for good, not protect anything.
		if !date.Before(timezone.TodayDate()) {
			for _, id := range uniqueStudentIDs {
				if found[id].IsAlumnus() {
					return fmt.Errorf("%w: graduated student in student_ids", ErrInvalidInstanceReference)
				}
			}
		}
	}

	return nil
}

// updateLifecycleColumns writes only the named columns on the given instance.
// Rationale: the bun pgdriver decodes TIME columns (start_time, end_time) as
// year 0000 on read, and Postgres rejects year 0000 on write — so a
// repo.Update() round-trip breaks the instance. Restricting the UPDATE to
// the columns this transition actually changes sidesteps that entirely and
// is also semantically cleaner (we don't want lifecycle code accidentally
// clobbering Title, etc.).
func (s *instanceService) updateLifecycleColumns(ctx context.Context, instance *scheduleModel.ActivityInstance, columns ...string) error {
	if len(columns) == 0 {
		return nil
	}
	_, err := s.deps.InstanceRepo.UpdateColumns(ctx, instance, columns...)
	return err
}

// ReplanWeek deletes protected-status='planned' non-spontaneous rows in
// [from, to] for the current tenant, then re-materializes the window.
// Everything else survives. A non-nil activityGroupID narrows the delete to
// that template's instances (the re-materialization still covers the whole
// window — insert-only, so other templates' surviving rows are skipped as
// existing). The DELETE is one raw statement so the predicate stays explicit
// and readable; the cascade on instance_staff / instance_students is declared
// at the DDL level (ON DELETE CASCADE).
func (s *instanceService) ReplanWeek(ctx context.Context, from, to timezone.Date, activityGroupID *int64, actorAccountID *int64) (*ReplanWeekResult, error) {
	if to.Before(from) {
		return nil, &ScheduleError{Op: "replan week: validate window", Err: errors.New("to_date must not be before from_date")}
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		// Defence-in-depth: if the route is ever hit without
		// TenantTxMiddleware the DELETE would silently match zero rows (and
		// MaterializeForTenant would operate on tenant 0 with nothing to
		// find). Fail fast instead of silently no-oping the admin action.
		return nil, &ScheduleError{Op: "replan week", Err: errors.New("no tenant in context")}
	}
	if _, ok := modelBase.TxFromContext(ctx); !ok {
		var result *ReplanWeekResult
		err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var err error
			result, err = s.ReplanWeek(txCtx, from, to, activityGroupID, actorAccountID)
			return err
		})
		return result, err
	}
	if err := lockTenantRecurrenceWrites(ctx, s.deps.DB); err != nil {
		return nil, &ScheduleError{Op: "replan week: lock recurrence", Err: err}
	}

	// Serialize against concurrent per-day substitute/absence saves for EVERY date
	// in the window BEFORE snapshotting or deleting (#1840). Re-plan deletes and
	// regenerates the exact rows /substitute and /deviations mutate; without the
	// day lock a concurrent save can commit a deviation between our snapshot and
	// delete (or after the snapshot but before regenerate), so re-plan would drop
	// it and regenerate from the stale snapshot — both requests succeed yet the
	// saved deviation is lost. Both endpoints take
	// timetable:substitute-day:<tenant>:<date>; we take it for the whole [from, to]
	// range, ascending, so two overlapping re-plans acquire in the same order and
	// cannot deadlock. Transaction-scoped: released when the tenant tx ends.
	if err := s.acquireSubstituteDayLocks(ctx, tenantID, from, to); err != nil {
		return nil, &ScheduleError{Op: "replan week: lock days", Err: err}
	}

	// Snapshot the manual Vertretungsplan overrides in the window BEFORE deleting
	// (#1840). Re-plan regenerates each occurrence from the (possibly just-edited)
	// base template. Freezing a deviated occurrence in place — the old approach —
	// is wrong: the "edit all occurrences" flow updates the template and re-plans,
	// so a frozen row keeps stale title/room/time/roster, and a template time
	// change leaves the frozen row beside a freshly materialized one (a duplicate
	// block whose (date, group, start_time) key no longer matches). Instead we
	// delete everything, regenerate, and reapply only the deviation fields.
	snapshots, occurrences, err := s.snapshotDeviations(ctx, from, to, activityGroupID)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: snapshot deviations", Err: err}
	}

	// preserveDeviations=false: delete deviated occurrences too so they are
	// regenerated with the current template values; the snapshot above lets us
	// reapply the overrides afterward.
	// A re-plan rebuilds every future derived row in its window. Legacy
	// Saturday/Sunday occurrences must be deleted too: materialization no
	// longer recreates weekends, and retaining them would leave stale title,
	// room, time, roster, or notes after a series edit.
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, from, &to, activityGroupID, false)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: delete planned", Err: err}
	}

	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, MaterializationSourceManual)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: materialize", Err: err}
	}

	reapplied, err := s.reapplyDeviations(ctx, snapshots, occurrences, nil, actorAccountID)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: reapply deviations", Err: err}
	}

	// Deletions bypass the CRUD broadcast paths, so the caches watching
	// staffing_deviation_changed would keep serving removed blocks. The
	// materializer broadcasts its own event when it recreated rows; a second
	// event for the delete side is harmless (subscribers refetch).
	if deleted > 0 {
		s.broadcastPlannedInstanceChanged(ctx, "replan_week")
	}

	s.getLogger().Info("replan week completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("from", from.String()),
		slog.String("to", to.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int("instances_created", mat.InstancesCreated),
		slog.Int("deviations_snapshotted", len(snapshots)),
		slog.Int("deviations_reapplied", reapplied),
	)

	return &ReplanWeekResult{
		From:             from,
		To:               to,
		DeletedInstances: int(deleted),
		Materialization:  mat,
	}, nil
}

// acquireSubstituteDayLocks takes the day-wide substitute/deviation advisory
// lock for every date in [from, to] inclusive, in ascending order. Ascending
// order gives a total lock ordering so two overlapping windows can never
// deadlock. Shares substituteDayLockKey with AcquireSubstituteDayLock so
// re-plan contends with the /substitute and /deviations endpoints (#1840).
func (s *instanceService) acquireSubstituteDayLocks(ctx context.Context, tenantID int64, from, to timezone.Date) error {
	for d := from; !d.After(to); d = d.AddDays(1) {
		if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenantID, d)); err != nil {
			return err
		}
	}
	return nil
}

// acquireSubstituteDayLockPair takes the day-wide substitute lock for two dates
// (deduping when they are equal) in ascending order, sharing the total lock
// ordering acquireSubstituteDayLocks relies on so a planned edit that moves a
// block never deadlocks against a re-plan window or a deviation save (#1840).
func (s *instanceService) acquireSubstituteDayLockPair(ctx context.Context, tenantID int64, a, b timezone.Date) error {
	first, second := a, b
	if second.Before(first) {
		first, second = second, first
	}
	if err := repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenantID, first)); err != nil {
		return err
	}
	if second == first {
		return nil
	}
	return repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenantID, second))
}

// deviationSnapshot captures the Vertretungsplan overrides (#1840) and the
// per-occurrence Personalbedarf pin (#1839) on one planned, template-backed
// occurrence so ReplanWeek can regenerate it from the (edited) template and
// then reapply the manual overrides. Keyed by
// (activityGroupID, date, startTime) — the same slot key the materializer uses —
// so the regenerated occurrence can be matched back.
type deviationSnapshot struct {
	date             timezone.Date
	activityGroupID  int64
	startTime        string // "15:04:05", for multi-slot disambiguation
	understaffedAck  bool
	understaffedNote *string
	// requiredStaff preserves a per-occurrence Personalbedarf pin (#1839).
	// Materialized rows leave the column NULL and inherit the template's
	// override at read time, so a non-NULL value here is always a deliberate
	// single-occurrence pin that must survive the re-plan.
	requiredStaff *int
	// absentPlanned: planned (non-substitute) rows that were marked absent.
	absentPlanned []snapshotAbsence
	// substitutes: extra substitute rows (is_substitute=true) to recreate.
	substitutes []snapshotSubstitute
}

type snapshotAbsence struct {
	staffID int64
	reason  *string
}

type snapshotSubstitute struct {
	staffID   int64
	roomID    *int64
	isPrimary bool
	isAbsent  bool
	reason    *string
}

// snapshotDeviations records every reappliable Vertretungsplan override on the
// planned, template-backed occurrences ReplanWeek is about to delete. Only those
// rows are regenerated, so only those can carry an override worth preserving.
func (s *instanceService) snapshotDeviations(ctx context.Context, from, to timezone.Date, activityGroupID *int64) ([]deviationSnapshot, map[groupDay]int, error) {
	instances, err := s.deps.InstanceRepo.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}
	snapshots := make([]deviationSnapshot, 0)
	// occurrences counts the planned, template-backed occurrences per (group, date)
	// BEFORE this re-plan deletes them — the ORIGINAL cardinality, not the number
	// of deviated slots. reapplyDeviations uses it to tell a genuine
	// single-occurrence move (follow a changed start_time) from a multi-slot day
	// where a deviated slot was deleted and only its start_time can safely
	// disambiguate the survivor (#1840). Counted over every matching occurrence,
	// including those with no override.
	occurrences := make(map[groupDay]int)
	for _, inst := range instances {
		if inst.Date.Weekday() == time.Saturday || inst.Date.Weekday() == time.Sunday {
			continue
		}
		if inst.Status != scheduleModel.InstanceStatusPlanned || inst.IsSpontaneous || inst.ActivityGroupID == nil {
			continue
		}
		if activityGroupID != nil && *inst.ActivityGroupID != *activityGroupID {
			continue
		}
		occurrences[groupDay{*inst.ActivityGroupID, inst.Date}]++
		rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return nil, nil, err
		}
		snap := deviationSnapshot{
			date:             inst.Date,
			activityGroupID:  *inst.ActivityGroupID,
			startTime:        formatTimeOfDay(inst.StartTime),
			understaffedAck:  inst.UnderstaffedAck,
			understaffedNote: inst.UnderstaffedNote,
			requiredStaff:    inst.RequiredStaff,
		}
		for _, row := range rows {
			switch {
			case row.IsSubstitute:
				snap.substitutes = append(snap.substitutes, snapshotSubstitute{
					staffID:   row.StaffID,
					roomID:    row.RoomID,
					isPrimary: row.IsPrimary,
					isAbsent:  row.IsAbsent,
					reason:    row.AbsenceReason,
				})
			case row.IsAbsent:
				snap.absentPlanned = append(snap.absentPlanned, snapshotAbsence{
					staffID: row.StaffID,
					reason:  row.AbsenceReason,
				})
			}
		}
		// Nothing overridden → nothing to reapply (but the occurrence was still
		// counted above, which is what the sole-occurrence check needs).
		if !snap.understaffedAck && snap.requiredStaff == nil &&
			len(snap.absentPlanned) == 0 && len(snap.substitutes) == 0 {
			continue
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, occurrences, nil
}

// reapplyDeviations reattaches each snapshotted override onto the freshly
// materialized occurrence, returning how many snapshots were reapplied. A
// snapshot whose occurrence no longer materializes (template weekday/period
// changed) is silently dropped — there is nothing to attach it to.
func (s *instanceService) reapplyDeviations(
	ctx context.Context,
	snapshots []deviationSnapshot,
	occurrences map[groupDay]int,
	targetActivityGroupID *int64,
	actorAccountID *int64,
) (int, error) {
	reapplied := 0
	for _, snap := range snapshots {
		// sole is true only when the group had exactly ONE planned occurrence on
		// this date BEFORE the re-plan deleted it. Counting snapshots instead
		// (the old approach) misreads a multi-slot day on which only one slot
		// carried a deviation as "sole": if a series edit then deletes that
		// deviated slot while another survives, the time-agnostic match below would
		// reapply the deleted slot's absences/substitutes/ack onto the surviving
		// block. Keying on the ORIGINAL occurrence cardinality forces an exact
		// start_time match whenever the day had more than one slot, so a deleted
		// slot's overrides are dropped rather than misattributed (#1840).
		sole := occurrences[groupDay{snap.activityGroupID, snap.date}] == 1
		inst, err := s.matchRegeneratedInstance(ctx, snap, sole, targetActivityGroupID)
		if err != nil {
			return reapplied, err
		}
		if inst == nil {
			// The slot no longer regenerates (weekday/period/time changed): the
			// snapshotted deviation is dropped. Record the loss in the
			// Änderungsprotokoll (#1886) — successful reapplies are NOT logged,
			// only losses, per the owner's decision.
			if err := s.logSnapshotDropped(ctx, snap, targetActivityGroupID, actorAccountID); err != nil {
				return reapplied, err
			}
			continue
		}

		// Reapply a per-occurrence Personalbedarf pin (#1839). Column-scoped
		// update: full-row Update does not round-trip SQL TIME columns safely
		// through Bun (see ActivityInstanceRepository.MarkCompleted).
		if snap.requiredStaff != nil {
			inst.RequiredStaff = snap.requiredStaff
			if _, err := s.deps.InstanceRepo.UpdateColumns(ctx, inst, "required_staff"); err != nil {
				return reapplied, err
			}
		}

		rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return reapplied, err
		}
		byStaff := make(map[int64]*scheduleModel.InstanceStaff, len(rows))
		for _, row := range rows {
			byStaff[row.StaffID] = row
		}

		// Reapply planned-staff absences onto the regenerated roster. A staff no
		// longer planned on the template simply has no row → the absence is moot.
		absencesReapplied := 0
		for _, ab := range snap.absentPlanned {
			row, ok := byStaff[ab.staffID]
			if !ok || row.IsSubstitute || row.IsAbsent {
				continue
			}
			row.IsAbsent = true
			row.AbsenceReason = ab.reason
			if err := s.deps.InstanceStaffRepo.Update(ctx, row); err != nil {
				return reapplied, err
			}
			absencesReapplied++
		}

		// Recreate substitute rows ONLY up to the number of planned absences
		// actually reapplied on this regenerated occurrence. A substitute row exists
		// solely to cover an absent planned position (the /substitute and
		// /deviations flows always flip an is_absent row on the same instance when
		// they insert a substitute), so the surviving absences bound how many
		// substitutes may return. If the "edit all occurrences" flow removed SOME
		// (not necessarily all) absent employees from the template, their absences
		// no longer land (their staff have no row on the regenerated occurrence) and
		// absencesReapplied drops below the substitute count — the now-uncovered
		// substitutes must be dropped too, or they become orphaned extra supervisors
		// that overstaff the block and report it as fully staffed.
		//
		// The snapshot carries no substitute→absent linkage, so we cannot tell WHICH
		// substitute is orphaned when only some absences survive; capping the
		// recreated count at absencesReapplied guarantees the block is never
		// overstaffed (the common 1:1 absence↔substitute case stays exact). The
		// understaffed-ack reapply below still runs — a snapshot may be
		// acknowledgement-only.
		//
		// A substitute already on the regenerated instance (e.g. now a planned
		// supervisor) is skipped so the recreate respects UNIQUE(instance_id,
		// staff_id); it adds no new row, so it does not consume the coverage budget.
		//
		// Only ACTIVE substitutes (is_absent=false) staff the block, so only they
		// are capped at absencesReapplied — an absent substitute row is dead history
		// (a replacement that was itself swapped out via "Entfernen") and staffs
		// nothing. Counting absent history against the budget would let a single
		// reapplied absence be consumed by a recreated is_absent=true row that
		// happens to sort first in the snapshot, exhausting the budget and dropping
		// the still-active replacement — the next ReplanWeek then silently turns a
		// covered block into an open gap. Guarding the cap on !sub.isAbsent keeps the
		// active coverage intact regardless of snapshot order while still bounding
		// live substitutes to the surviving absences (#1840).
		recreated := 0
		for _, sub := range snap.substitutes {
			if _, taken := byStaff[sub.staffID]; taken {
				continue
			}
			if !sub.isAbsent {
				if recreated >= absencesReapplied {
					continue
				}
				recreated++
			}
			newRow := &scheduleModel.InstanceStaff{
				InstanceID:    inst.ID,
				StaffID:       sub.staffID,
				RoomID:        sub.roomID,
				IsPrimary:     sub.isPrimary,
				IsSubstitute:  true,
				IsAbsent:      sub.isAbsent,
				AbsenceReason: sub.reason,
			}
			if err := s.deps.InstanceStaffRepo.Create(ctx, newRow); err != nil {
				return reapplied, err
			}
			byStaff[sub.staffID] = newRow
		}

		// Reapply the "deliberately unstaffed" acknowledgement. SetUnderstaffedAck
		// re-reads the roster we just wrote and rejects the ack only when the block
		// is fully staffed after reapply — in which case the now-stale ack is
		// dropped, matching the endpoints' reconciliation.
		if snap.understaffedAck {
			if _, err := s.setUnderstaffedAck(ctx, inst.ID, true, snap.understaffedNote, nil, false); err != nil {
				if !errors.Is(err, ErrUnderstaffedAckStillStaffed) {
					return reapplied, err
				}
			}
		}
		reapplied++
	}
	return reapplied, nil
}

// groupDay keys snapshots by their (activity group, date) slot so reapply can
// tell a day with a single override from one whose several slots collapsed.
type groupDay struct {
	activityGroupID int64
	date            timezone.Date
}

// matchRegeneratedInstance finds the freshly materialized planned occurrence a
// snapshot should reapply to. When `sole` (the group had exactly ONE planned
// occurrence on this date before the re-plan — see reapplyDeviations) a lone
// surviving occurrence is matched even if its start_time changed, so the
// deviation follows the moved block. Otherwise — the day had several slots — it
// disambiguates strictly by the original start_time and drops the snapshot when
// none matches, so overrides from a deleted slot are never merged onto a
// surviving block (a slot whose time changed cannot be mapped safely either)
// (#1840).
func (s *instanceService) matchRegeneratedInstance(
	ctx context.Context,
	snap deviationSnapshot,
	sole bool,
	targetActivityGroupID *int64,
) (*scheduleModel.ActivityInstance, error) {
	activityGroupID := snap.activityGroupID
	if targetActivityGroupID != nil {
		activityGroupID = *targetActivityGroupID
	}
	candidates, err := s.deps.InstanceRepo.FindByActivityGroupAndDate(ctx, activityGroupID, snap.date)
	if err != nil {
		return nil, err
	}
	var planned []*scheduleModel.ActivityInstance
	for _, c := range candidates {
		if c.Status == scheduleModel.InstanceStatusPlanned && !c.IsSpontaneous {
			planned = append(planned, c)
		}
	}
	if len(planned) == 0 {
		return nil, nil
	}
	if len(planned) == 1 && sole {
		return planned[0], nil
	}
	for _, c := range planned {
		if formatTimeOfDay(c.StartTime) == snap.startTime {
			return c, nil
		}
	}
	return nil, nil
}

// loadForTransition is the shared load + not-found branch used by all three
// transitions. The base repo wraps sql.ErrNoRows inside a DatabaseError so
// errors.Is alone doesn't catch it — isNotFoundDBError unwraps.
func (s *instanceService) loadForTransition(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrInstanceNotFound
		}
		return nil, &ScheduleError{Op: "load instance", Err: err}
	}
	if instance == nil {
		return nil, ErrInstanceNotFound
	}
	return instance, nil
}

// broadcastPlannedInstanceChanged queues one tenant-wide
// staffing_deviation_changed event after the surrounding tenant transaction
// commits. Ordinary CRUD on still-planned instances (create, edit incl.
// staff_ids/date moves, delete) triggers no lifecycle transition, so no
// instance_* event fires — yet it changes who is planned where, and the same
// caches (planner, "Heute geplant" card) go stale until reload (#1844).
// source names the emitting flow for log review.
func (s *instanceService) broadcastPlannedInstanceChanged(ctx context.Context, source string) {
	broadcastStaffingChanged(ctx, s.deps.Broadcaster, s.getLogger(), source)
}

// broadcastStaffingChanged queues one tenant-wide staffing_deviation_changed
// event after the surrounding tenant transaction commits (outside a tx it
// fires immediately). Shared by the instance CRUD paths, ReplanWeek, the
// materializer, and the template split/end flows — every write that changes
// who is planned where must emit it, or the planner and "Heute geplant" card
// (which disables focus revalidation) go stale until reload (#1844). A nil
// broadcaster is a no-op (unit tests, CLI wiring).
func broadcastStaffingChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger, source string) {
	if broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventStaffingDeviationChanged, "", realtime.EventData{Source: &source})
	tenant.RegisterAfterCommit(ctx, func() {
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("SSE planned instance broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// broadcastInstanceEvent is a nil-safe fire-and-forget SSE wrapper. Events
// with a live active.group route to group subscribers; events without (i.e.
// cancelled-from-planned) broadcast to all so the admin dashboard sees them.
//
// Broadcasts are queued through tenant.RegisterAfterCommit so subscribers that
// refetch in response to the event see committed timetable state.
func (s *instanceService) broadcastInstanceEvent(
	ctx context.Context,
	eventType realtime.EventType,
	instance *scheduleModel.ActivityInstance,
	activeGroup *activeModel.Group,
	staffRows []*scheduleModel.InstanceStaff,
) {
	if s.deps.Broadcaster == nil || instance == nil {
		return
	}

	instanceIDStr := fmt.Sprintf("%d", instance.ID)
	instanceDate := instance.Date.String()
	instanceStart := instance.StartTime.Format("15:04:05")

	data := realtime.EventData{
		InstanceID:        &instanceIDStr,
		InstanceDate:      &instanceDate,
		InstanceStartTime: &instanceStart,
	}

	activeGroupIDStr := ""
	if activeGroup != nil {
		activeGroupIDStr = fmt.Sprintf("%d", activeGroup.ID)
		roomIDStr := fmt.Sprintf("%d", activeGroup.RoomID)
		data.RoomID = &roomIDStr
		if s.deps.RoomRepo != nil {
			if room, err := s.deps.RoomRepo.FindByID(ctx, activeGroup.RoomID); err == nil && room != nil {
				name := room.Name
				data.RoomName = &name
			}
		}
		if instance.ActivityGroupID != nil && s.deps.ActivityGroupRepo != nil {
			if ag, err := s.deps.ActivityGroupRepo.FindByID(ctx, *instance.ActivityGroupID); err == nil && ag != nil {
				name := ag.Name
				data.ActivityName = &name
			}
		}
		if len(staffRows) > 0 {
			ids := make([]string, 0, len(staffRows))
			for _, r := range staffRows {
				ids = append(ids, fmt.Sprintf("%d", r.StaffID))
			}
			data.SupervisorIDs = &ids
		}
	} else if instance.ActiveGroupID != nil {
		activeGroupIDStr = fmt.Sprintf("%d", *instance.ActiveGroupID)
	}

	event := realtime.NewEvent(eventType, activeGroupIDStr, data)
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if activeGroupIDStr != "" {
			if err := s.deps.Broadcaster.BroadcastToGroup(tenantID, activeGroupIDStr, event); err != nil {
				s.getLogger().Warn("SSE broadcast failed",
					slog.String("event_type", string(eventType)),
					slog.String("active_group_id", activeGroupIDStr),
					slog.String("error", err.Error()),
				)
			}
		}
		if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			s.getLogger().Warn("SSE broadcast-to-tenant failed",
				slog.String("event_type", string(eventType)),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
		s.broadcastActiveSupervisionChanged(tenantID, activeGroupIDStr, instanceIDStr, eventType)
	})
}

func (s *instanceService) broadcastActiveSupervisionChanged(
	tenantID int64,
	activeGroupID string,
	instanceID string,
	sourceEventType realtime.EventType,
) {
	reason := instanceRefreshReason(sourceEventType)
	data := realtime.EventData{
		InstanceID: &instanceID,
		Reason:     &reason,
	}
	event := realtime.NewEvent(realtime.EventActiveSupervisionChanged, activeGroupID, data)
	if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("SSE active supervision broadcast failed",
			slog.String("event_type", string(sourceEventType)),
			slog.String("active_group_id", activeGroupID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	}
}

func instanceRefreshReason(eventType realtime.EventType) string {
	switch eventType {
	case realtime.EventInstanceStarted:
		return "instance_started"
	case realtime.EventInstanceCompleted:
		return "instance_completed"
	case realtime.EventInstanceCancelled:
		return "instance_cancelled"
	case realtime.EventInstanceOverdue:
		return "instance_overdue"
	default:
		return "instance_changed"
	}
}

// GetPlannedStudentIDsByDate returns the unique student IDs (of the given
// candidates) that have a planned instance on the date.
func (s *instanceService) GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error) {
	return s.deps.InstanceStudents.FindPlannedStudentIDsByDate(ctx, studentIDs, date)
}
