package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// InstanceLifecycleService is the Timetable owner's instance lifecycle.
type InstanceLifecycleService struct {
	deps ports.InstanceLifecycleDependencies
}

var _ timetable.InstanceLifecycleCapability = (*InstanceLifecycleService)(nil)

// NewInstanceLifecycle composes the lifecycle. The transitions have no
// degraded mode, so a missing required collaborator fails composition.
func NewInstanceLifecycle(deps ports.InstanceLifecycleDependencies) (*InstanceLifecycleService, error) {
	if !lifecycleRepositoriesWired(deps) || !lifecycleCollaboratorsWired(deps) || (deps.EnforceTimePolicy && deps.Settings == nil) {
		return nil, errors.New("timetable instance lifecycle: required dependency is nil")
	}
	return &InstanceLifecycleService{deps: deps}, nil
}

func lifecycleRepositoriesWired(deps ports.InstanceLifecycleDependencies) bool {
	return !noInstanceRows(deps.InstanceRepo) && deps.IdempotencyRepo != nil && deps.InstanceStaffRepo != nil &&
		!noParticipantRows(deps.InstanceStudents) && deps.ExceptionRepo != nil && deps.CalendarPeriodRepo != nil &&
		deps.RecoveryRepo != nil && deps.ActivityGroupRepo != nil && deps.StaffRepo != nil && deps.StudentRepo != nil
}

func lifecycleCollaboratorsWired(deps ports.InstanceLifecycleDependencies) bool {
	return deps.ActiveGroupRepo != nil && deps.SupervisorRepo != nil && deps.Presence != nil && deps.ActiveService != nil &&
		deps.Rooms != nil && deps.CareDays != nil && deps.CareDayLocks != nil && deps.Protocol != nil &&
		deps.Materialization != nil && deps.RecurrenceLock != nil && deps.StartConflicts != nil &&
		deps.SubstituteConflicts != nil && deps.ContentHash != nil && deps.Database != nil
}

// SeriesDeviations exposes the re-plan's deviation snapshot and reapply
// machinery (#1840) to the template split, which preserves Vertretungsplan
// overrides with it.
func (s *InstanceLifecycleService) SeriesDeviations() ports.SeriesDeviations {
	return seriesDeviationPreserver{lifecycle: s}
}

func (s *InstanceLifecycleService) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *InstanceLifecycleService) getLogger() *slog.Logger {
	return OrDefaultLogger(s.deps.Logger)
}

// hasTx reports whether the caller already runs inside the tenant
// transaction the advisory gates need. False for direct calls outside
// TenantTxMiddleware (CLI, tests), which then get their own transaction.
func (s *InstanceLifecycleService) hasTx(ctx context.Context) bool {
	if s.deps.Database == nil {
		return true // no DB wired (unit tests): nothing to lock, nothing to wrap
	}
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every same-day staffing mutation, within the caller's tx.
func (s *InstanceLifecycleService) acquireSubstituteDayLock(ctx context.Context, date calendar.Date) error {
	return s.acquireTenantDayLock(ctx, tenant.FromContext(ctx), date)
}

func (s *InstanceLifecycleService) acquireTenantDayLock(ctx context.Context, tenantID int64, date calendar.Date) error {
	return s.deps.Database.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenantID, date))
}

// acquireSubstituteDayLocks takes the day lock of every date in [from, to]
// inclusive, in ascending order, so two overlapping windows can never
// deadlock (#1840).
func (s *InstanceLifecycleService) acquireSubstituteDayLocks(ctx context.Context, tenantID int64, from, to calendar.Date) error {
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
func (s *InstanceLifecycleService) acquireSubstituteDayLockPair(ctx context.Context, tenantID int64, a, b calendar.Date) error {
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
	if err := s.acquireSubstituteDayLock(ctx, calendar.Date(lockedDate)); err != nil {
		return nil, &ports.ScheduleError{Op: op + ": lock day", Err: err}
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
	if s.deps.Database == nil || s.deps.RecurrenceLock == nil {
		return nil
	}
	if err := s.deps.RecurrenceLock.LockRecurrenceWritesThenGradeTransitions(ctx); err != nil {
		return &ports.ScheduleError{Op: op + ": lock recurrence", Err: err}
	}
	return nil
}

// loadForTransition is the shared load and not-found branch of the
// transitions. The retained repository wraps sql.ErrNoRows, so IsNoRows
// unwraps it.
func (s *InstanceLifecycleService) loadForTransition(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := findLegacyInstance(ctx, s.deps.InstanceRepo.Reads, instanceID)
	if err != nil {
		if domain.IsRecordNotFound(err) {
			return nil, timetable.ErrInstanceNotFound
		}
		return nil, &ports.ScheduleError{Op: "load instance", Err: err}
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
	_, err := updateLegacyInstanceColumns(ctx, s.deps.InstanceRepo, instance, columns...)
	return err
}

// GetPlannedStudentIDsByDate returns which of the children have a planned
// block on the date.
func (s *InstanceLifecycleService) GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error) {
	return plannedStudentIDsOn(ctx, s.deps.InstanceStudents.Plans, studentIDs, calendar.Date(date))
}

// detectStartConflicts asks the owner's start check and keeps the retained
// error classification: a failed read aborts the transition.
func detectStartConflicts(ctx context.Context, query timetable.StartConflictQuery, instance *scheduleModel.ActivityInstance) ([]timetable.InstanceConflictWarning, error) {
	if query == nil {
		return nil, &ports.ScheduleError{Op: "detect start conflicts", Err: errors.New("start conflict detection is not configured")}
	}
	warnings, err := query.DetectStartConflicts(ctx, timetable.StartConflictSubject{InstanceID: instance.ID, RoomID: instance.RoomID})
	if err != nil {
		return nil, &ports.ScheduleError{Op: "detect start conflicts", Err: err}
	}
	return warnings, nil
}

// LifecycleInstanceOf maps a retained row onto the lifecycle's public view.
func LifecycleInstanceOf(row *scheduleModel.ActivityInstance) *timetable.LifecycleInstance {
	if row == nil {
		return nil
	}
	return &timetable.LifecycleInstance{
		ID: row.ID, Date: calendar.Date(row.Date), StartTime: row.StartTime, EndTime: row.EndTime,
		Title: row.Title, RoomID: row.RoomID, ActivityGroupID: row.ActivityGroupID, Status: row.Status,
		IsSpontaneous: row.IsSpontaneous, UnderstaffedAck: row.UnderstaffedAck, UnderstaffedNote: row.UnderstaffedNote,
		ActiveGroupID: row.ActiveGroupID, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, ReopenUntil: row.ReopenUntil,
	}
}

func instanceRowIDs(instances []*scheduleModel.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func ptrTo[T any](value T) *T { return &value }
