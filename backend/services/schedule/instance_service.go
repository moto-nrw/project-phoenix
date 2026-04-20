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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
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
	ReplanWeek(ctx context.Context, from, to time.Time) (*ReplanWeekResult, error)
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
	From             time.Time
	To               time.Time
	DeletedInstances int
	Materialization  *MaterializationResult
}

// InstanceServiceDependencies aggregates wiring. All repo fields are required;
// the optional-ish ones are Broadcaster (nil → no SSE), RoomRepo and
// ActivityGroupRepo (nil → SSE lacks human-readable names, not a failure).
type InstanceServiceDependencies struct {
	InstanceRepo      scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	InstanceStudents  scheduleModel.InstanceStudentRepository
	ActiveGroupRepo   activeModel.GroupRepository
	SupervisorRepo    activeModel.GroupSupervisorRepository
	VisitRepo         activeModel.VisitRepository
	RoomRepo          facilitiesModel.RoomRepository
	ActivityGroupRepo activitiesModel.GroupRepository
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
		deps.ActiveGroupRepo == nil || deps.SupervisorRepo == nil || deps.VisitRepo == nil ||
		deps.ActiveService == nil || deps.Materialization == nil || deps.DB == nil {
		panic("schedule.NewInstanceService: required dependency is nil")
	}
	return &instanceService{deps: deps}
}

func (s *instanceService) getLogger() *slog.Logger {
	if s.deps.Logger != nil {
		return s.deps.Logger
	}
	return slog.Default()
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

	// Each staff row becomes a supervisor. Failure aborts — unlike the NFC
	// best-effort path, a planner start with missing supervisors would
	// silently undermine the conflict model for the staff's next start.
	for _, row := range staffRows {
		sup := &activeModel.GroupSupervisor{
			StaffID:   row.StaffID,
			GroupID:   newGroup.ID,
			Role:      "supervisor",
			StartDate: now,
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

	s.broadcastInstanceEvent(ctx, realtime.EventInstanceStarted, instance, newGroup, staffRows)

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

	if instance.Status == scheduleModel.InstanceStatusActive && instance.ActiveGroupID != nil {
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
	q := repoBase.GetDB(ctx, s.deps.DB).NewUpdate().
		Model(instance).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".id = ?`, instance.ID).
		Where(`"activity_instance".tenant_id = ?`, tenant.FromContext(ctx))
	for _, col := range columns {
		q = q.Column(col)
	}
	_, err := q.Exec(ctx)
	return err
}

// ReplanWeek deletes protected-status='planned' non-spontaneous rows in
// [from, to] for the current tenant, then re-materializes the window.
// Everything else survives. The DELETE is one raw statement so the predicate
// stays explicit and readable; the cascade on instance_staff / instance_students
// is declared at the DDL level (ON DELETE CASCADE).
func (s *instanceService) ReplanWeek(ctx context.Context, from, to time.Time) (*ReplanWeekResult, error) {
	from = truncateToDay(from)
	to = truncateToDay(to)
	if to.Before(from) {
		return nil, &ScheduleError{Op: "replan week: validate window", Err: errors.New("to_date must not be before from_date")}
	}

	tenantID := tenant.FromContext(ctx)

	res, err := repoBase.GetDB(ctx, s.deps.DB).NewDelete().
		Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).
		Where("date >= ?", from).
		Where("date <= ?", to).
		Where("status = ?", scheduleModel.InstanceStatusPlanned).
		Where("is_spontaneous = ?", false).
		Exec(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: delete planned", Err: err}
	}
	deleted, _ := res.RowsAffected() // nil-driver-safe: fall through with 0

	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, MaterializationSourceManual)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: materialize", Err: err}
	}

	s.getLogger().Info("replan week completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("from", from.Format("2006-01-02")),
		slog.String("to", to.Format("2006-01-02")),
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
		if errors.Is(err, sql.ErrNoRows) || isNotFoundDBError(err) {
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
	instanceDate := instance.Date.Format("2006-01-02")
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
	if activeGroupIDStr != "" {
		if err := s.deps.Broadcaster.BroadcastToGroup(tenantID, activeGroupIDStr, event); err != nil {
			s.getLogger().Warn("SSE broadcast failed",
				slog.String("event_type", string(eventType)),
				slog.String("active_group_id", activeGroupIDStr),
				slog.String("error", err.Error()),
			)
		}
		return
	}
	if err := s.deps.Broadcaster.BroadcastToAll(event); err != nil {
		s.getLogger().Warn("SSE broadcast-all failed",
			slog.String("event_type", string(eventType)),
			slog.String("error", err.Error()),
		)
	}
}

// truncateToDay strips the time component to UTC midnight. Matches the civil-
// date normalization the materialization service uses so the ReplanWeek
// window aligns exactly with the re-materialization run.
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// isNotFoundDBError unwraps models/base.DatabaseError and reports whether
// the underlying error is sql.ErrNoRows. The repo layer wraps sql.ErrNoRows
// in a DatabaseError; errors.Is(err, sql.ErrNoRows) alone misses that case.
func isNotFoundDBError(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}
