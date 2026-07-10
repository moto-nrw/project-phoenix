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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
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

	// ErrAmbiguousTemplateInstanceDelete is returned when a single-instance
	// delete would need to persist a date-wide cancellation exception but the
	// template has multiple materialized slots on that date. Handlers map this
	// to 409 so the UI can explain that the user must delete the series or keep
	// the slots unchanged until slot-scoped exceptions exist.
	ErrAmbiguousTemplateInstanceDelete = errors.New("template instance delete is ambiguous")

	// ErrSchulhofSupervisionRequired prevents timetable instances from
	// creating a second active group in the permanent Schulhof room. Staff
	// must use the dedicated Schulhof supervision flow instead.
	ErrSchulhofSupervisionRequired = errors.New("für den Schulhof bitte die Schulhof-Aufsicht verwenden")
)

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
	Cancel(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error)
	DeleteCancelled(ctx context.Context, instanceID int64) error
	// ReplanWeek deletes planned non-spontaneous instances in [from, to] and
	// re-materializes. A non-nil activityGroupID restricts the delete to one
	// template's instances; nil re-plans the whole grid.
	ReplanWeek(ctx context.Context, from, to timezone.Date, activityGroupID *int64) (*ReplanWeekResult, error)
	// GetPlannedStudentIDsByDate returns the unique student IDs (of the given
	// candidates) that have a planned instance on the date (issue #584
	// lookup; repository result returned verbatim).
	GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error)
	Create(ctx context.Context, req CreateInstanceInput) (*scheduleModel.ActivityInstance, error)
	UpdatePlanned(ctx context.Context, instanceID int64, req UpdateInstanceInput) (*scheduleModel.ActivityInstance, error)
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
	IsSpontaneous    *bool
	StaffIDs         []int64
	StudentIDs       []int64
	CreatedByStaffID *int64
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
	StaffIDs        []int64
	StudentIDs      []int64
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
	Broadcaster       realtime.Broadcaster
	DB                *bun.DB
	Logger            *slog.Logger
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
		deps.DB == nil {
		panic("schedule.NewInstanceService: required dependency is nil")
	}
	return &instanceService{deps: deps}
}

func (s *instanceService) getLogger() *slog.Logger {
	return cmp.Or(s.deps.Logger, slog.Default())
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

	room, err := s.deps.RoomRepo.FindByID(ctx, instance.RoomID)
	if err != nil {
		return nil, &ScheduleError{Op: "start instance: load room", Err: err}
	}
	if room == nil {
		return nil, &ScheduleError{Op: "start instance: load room", Err: errors.New("instance room not found")}
	}
	if room.Name == constants.SchulhofRoomName {
		return nil, ErrSchulhofSupervisionRequired
	}

	// Conflict detection is read-only + advisory. Warnings reflect state
	// inside the tx; they never block the transition.
	warnings := DetectStartConflicts(ctx, ConflictDependencies{
		GroupRepo:         s.deps.ActiveGroupRepo,
		SupervisorRepo:    s.deps.SupervisorRepo,
		VisitRepo:         s.deps.VisitRepo,
		InstanceStaffRepo: s.deps.InstanceStaffRepo,
		InstanceStudents:  s.deps.InstanceStudents,
	}, instance, s.getLogger())

	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "start instance: load instance_staff", Err: err}
	}

	now := time.Now()
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

// Complete implements active → completed. The bridge active.group is ended
// via active.EndActivitySession — this closes open visits, ends supervisors,
// and fires per-student checkout SSE events, matching today's observable
// behavior when a session ends.
func (s *instanceService) Complete(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Status != scheduleModel.InstanceStatusActive {
		return nil, fmt.Errorf("%w: cannot complete instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	if instance.ActiveGroupID == nil {
		// An active instance without a bridge shouldn't exist post WP-B9.
		// Treat as data corruption — abort the tx rather than silently
		// "completing" an instance that never actually ran.
		return nil, &ScheduleError{Op: "complete instance", Err: fmt.Errorf("active instance %d has no active_group_id", instance.ID)}
	}

	// Mark any remaining expected students as absent before ending the active
	// group. Runs inside the caller's tenant tx — if EndActivitySession or
	// updateLifecycleColumns fail below, the bulk update rolls back too, so
	// the instance never leaves the tx in a half-finished state.
	if _, err := s.deps.InstanceStudents.BulkUpdateStatus(
		ctx, instance.ID, scheduleModel.AttendanceStatusExpected, scheduleModel.AttendanceStatusAbsent,
	); err != nil {
		return nil, &ScheduleError{Op: "complete instance: mark absent", Err: err}
	}

	if err := s.deps.ActiveService.EndActivitySession(ctx, *instance.ActiveGroupID); err != nil {
		return nil, &ScheduleError{Op: "complete instance: end active.group", Err: err}
	}

	now := time.Now()
	instance.Status = scheduleModel.InstanceStatusCompleted
	instance.CompletedAt = &now
	if err := s.updateLifecycleColumns(ctx, instance, "status", "completed_at"); err != nil {
		return nil, &ScheduleError{Op: "complete instance: update", Err: err}
	}

	s.broadcastInstanceEvent(ctx, realtime.EventInstanceCompleted, instance, nil, nil)
	return instance, nil
}

// Cancel implements planned|active → cancelled. From active, the bridge is
// ended the same way Complete does (visits + supervisors close, checkout
// events fire). From planned there is no bridge yet; just stamp the status.
func (s *instanceService) Cancel(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
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

	now := time.Now()
	instance.Status = scheduleModel.InstanceStatusCancelled
	instance.CompletedAt = &now
	if err := s.updateLifecycleColumns(ctx, instance, "status", "completed_at"); err != nil {
		return nil, &ScheduleError{Op: "cancel instance: update", Err: err}
	}

	s.broadcastInstanceEvent(ctx, realtime.EventInstanceCancelled, instance, nil, nil)
	return instance, nil
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
	if err := s.validateInstanceReferences(ctx, req.RoomID, req.ActivityGroupID, req.StaffIDs, req.StudentIDs, req.CreatedByStaffID); err != nil {
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
	for _, studentID := range sliceutil.UniquePositive(req.StudentIDs) {
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

	s.getLogger().Info("instance created",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("instance_id", inst.ID),
		slog.String("date", inst.Date.String()),
		slog.Bool("spontaneous", inst.IsSpontaneous),
		slog.Int("staff_assigned", len(req.StaffIDs)),
	)

	return inst, nil
}

func (s *instanceService) UpdatePlanned(ctx context.Context, instanceID int64, req UpdateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot update instance in status %q", ErrInvalidInstanceTransition, instance.Status)
	}
	if err := s.validateInstanceReferences(ctx, req.RoomID, req.ActivityGroupID, req.StaffIDs, req.StudentIDs, nil); err != nil {
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
	instance.IsSpontaneous = req.ActivityGroupID == nil

	if err := s.updateLifecycleColumns(
		ctx,
		instance,
		"date",
		"start_time",
		"end_time",
		"title",
		"description",
		"notes",
		"room_id",
		"activity_group_id",
		"is_spontaneous",
	); err != nil {
		return nil, &ScheduleError{Op: "update instance", Err: err}
	}
	if err := s.consumeMovedSlot(ctx, origSlot, req); err != nil {
		return nil, err
	}
	if err := s.replaceInstanceAssignments(ctx, instance.ID, req.StaffIDs, req.StudentIDs); err != nil {
		return nil, err
	}
	return instance, nil
}

// replaceInstanceAssignments wipes and re-creates the instance's staff and
// student rows from the request lists. Extracted from UpdatePlanned to keep
// its cognitive complexity in check.
func (s *instanceService) replaceInstanceAssignments(ctx context.Context, instanceID int64, staffIDs, studentIDs []int64) error {
	if err := s.deps.InstanceStaffRepo.DeleteByInstanceID(ctx, instanceID); err != nil {
		return &ScheduleError{Op: "update instance: clear staff", Err: err}
	}
	if err := s.deps.InstanceStudents.DeleteByInstanceID(ctx, instanceID); err != nil {
		return &ScheduleError{Op: "update instance: clear students", Err: err}
	}
	tenantID := tenant.FromContext(ctx)
	for _, staffID := range sliceutil.UniquePositive(staffIDs) {
		row := &scheduleModel.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
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

func (s *instanceService) validateInstanceReferences(
	ctx context.Context,
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
func (s *instanceService) ReplanWeek(ctx context.Context, from, to timezone.Date, activityGroupID *int64) (*ReplanWeekResult, error) {
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

	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, from, &to, activityGroupID)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: delete planned", Err: err}
	}

	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, MaterializationSourceManual)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: materialize", Err: err}
	}

	s.getLogger().Info("replan week completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("from", from.String()),
		slog.String("to", to.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int("instances_created", mat.InstancesCreated),
	)

	return &ReplanWeekResult{
		From:             from,
		To:               to,
		DeletedInstances: int(deleted),
		Materialization:  mat,
	}, nil
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
