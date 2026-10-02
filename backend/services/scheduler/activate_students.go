package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The student statuses the activate-students tick moves between.
const (
	studentStatusPending  = "pending"
	studentStatusActive   = "active"
	studentStatusInactive = "inactive"
)

// StudentLifecycleRepository is the narrow contract the activate-students tick
// needs from the student repository. The root binds the retained repository;
// statuses travel as their stored values.
type StudentLifecycleRepository interface {
	// FindPendingDueForActivation returns the pending children whose care
	// starts on or before asOf.
	FindPendingDueForActivation(ctx context.Context, asOf calendar.Date) ([]int64, error)
	// FindActiveDueForDeactivation returns the active children whose care
	// ended before asOf.
	FindActiveDueForDeactivation(ctx context.Context, asOf calendar.Date) ([]int64, error)
	// Compare-and-set, deliberately not the unconditional UpdateStatus: the tick
	// decides from rows it read earlier, and by the time it writes, a grade
	// transition may have graduated the child (see the repository method's doc).
	TransitionStatus(ctx context.Context, studentID int64, expected, next string) (bool, error)
}

// CareExitEffector performs the effect-day housekeeping for children whose
// care has ended (#2487): closing what is still open, freeing the bracelet,
// closing the open parent requests. Narrow contract so the scheduler does not
// import the whole users service package.
type CareExitEffector interface {
	ApplyDueEffects(ctx context.Context, asOf calendar.Date) (int, error)
}

// StudentLifecycleAuditor records scheduler-authored status transitions.
type StudentLifecycleAuditor interface {
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
}

// activateStudentsInterval is how often the activate-students tick runs. It
// used to be a school setting that was never read per school (#3733). Date
// transitions only happen on day boundaries; the cadence is a safety-net for
// restarts and clock drift, not a precision dial.
const activateStudentsInterval = 60 * time.Minute

// scheduleActivateStudentsTask registers the per-tenant activate-students
// poll, every activateStudentsInterval.
func (s *Scheduler) scheduleActivateStudentsTask() {
	if s.studentLifecycleRepo == nil {
		s.getLogger().Info("activate-students task not configured (no StudentLifecycleRepository)")
		return
	}

	s.registerTask("activate-students", "interval-poll", s.runActivateStudentsTaskPolling)
}

// runActivateStudentsTaskPolling ticks every activateStudentsInterval.
func (s *Scheduler) runActivateStudentsTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in activate-students task",
		"activate-students using interval polling for per-tenant scheduling",
		20*time.Second, func() time.Duration { return activateStudentsInterval }, s.checkAndRunActivateStudents)
}

// checkAndRunActivateStudents iterates active tenants and runs activation +
// deactivation per tenant. Re-entry is guarded by task.Running so a slow
// tenant cannot cause overlapping ticks.
func (s *Scheduler) checkAndRunActivateStudents(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	ctx, cancel := s.taskContext(ctx, 30*time.Minute)
	defer cancel()

	now := time.Now()
	s.forEachTenantSettings(ctx, "activate-students", func(tenantCtx context.Context, tenantID int64) error {
		return s.runActivateStudentsForTenantWithError(tenantCtx, tenantID, now)
	})
}

// runActivateStudentsForTenant flips eligible pending students to active and
// eligible active students to inactive. Idempotent — running twice on the
// same day is a no-op after the first pass. Status-update failures are logged
// and skipped. Audit failures abort the tenant transaction so an automated
// transition can never commit without its history entry.
func (s *Scheduler) runActivateStudentsForTenantWithError(ctx context.Context, tenantID int64, now time.Time) error {
	asOf := calendar.DateFromTime(now)

	pending, err := s.studentLifecycleRepo.FindPendingDueForActivation(ctx, asOf)
	if err != nil {
		s.getLogger().Error("activate-students: load pending failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	} else {
		if err := s.applyStatusTransitions(ctx, tenantID, pending,
			studentStatusPending, studentStatusActive); err != nil {
			return err
		}
	}

	// The enrollment interval's upper bound is INCLUSIVE: a child still
	// belongs to the OGS on their last care day and only leaves the day after
	// (#2487). The deactivation boundary is therefore YESTERDAY, not today —
	// matching filterStudentsStartedOnDate, which every operational reader
	// already agrees with.
	careBoundary := asOf.AddDays(-1)

	// The effects run BEFORE the status transition and select on the same
	// still-'active' rows, which is what keeps them from being re-applied on
	// every later tick.
	if s.careExitEffector != nil {
		if _, err := s.careExitEffector.ApplyDueEffects(ctx, asOf); err != nil {
			s.getLogger().Error("activate-students: care exit effects failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("apply care exit effects: %w", err)
		}
	}

	dueInactive, err := s.studentLifecycleRepo.FindActiveDueForDeactivation(ctx, careBoundary)
	if err != nil {
		s.getLogger().Error("activate-students: load active-due failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return s.applyStatusTransitions(ctx, tenantID, dueInactive,
		studentStatusActive, studentStatusInactive)
}

// applyStatusTransitions updates each student to newStatus and emits a slog
// info entry per transition. Status-update errors are logged and skipped;
// audit errors abort the tenant transaction. GDPR: student IDs only — no names
// at info level (CLAUDE.md backend logging rule).
//
// The write is a compare-and-set on `from` — the status the row carried when the
// Find query selected it. Between that query and this update a grade transition
// can graduate the child; the update then waits on its row lock and would
// otherwise overwrite `alumnus` with active/inactive, resurrecting a departed
// student past every alumnus read filter and without any of apply's guards. A row
// whose status moved on is skipped, not an error — the next tick re-evaluates it
// from current data (#405 review).
func (s *Scheduler) applyStatusTransitions(ctx context.Context, tenantID int64, studentIDs []int64, from, to string) error {
	if len(studentIDs) == 0 {
		return nil
	}
	transitions := 0
	for _, studentID := range studentIDs {
		applied, err := s.studentLifecycleRepo.TransitionStatus(ctx, studentID, from, to)
		if err != nil {
			s.getLogger().Error("activate-students: update status failed",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("from", from),
				slog.String("to", to),
				slog.String("error", err.Error()),
			)
			continue
		}
		if !applied {
			s.getLogger().Debug("activate-students: status transition skipped after concurrent change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("expected_status", from),
				slog.String("next_status", to),
			)
			continue
		}
		if s.studentLifecycleAudit != nil {
			if err := s.studentLifecycleAudit.RecordSystemStatusChange(ctx, studentID, from, to); err != nil {
				s.getLogger().Error("activate-students: audit status transition failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", studentID),
					slog.String("from", from),
					slog.String("to", to),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("audit student %d status transition: %w", studentID, err)
			}
		}
		transitions++
		s.getLogger().Info("student status transition",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("from", from),
			slog.String("to", to),
		)
	}
	s.getLogger().Info("activate-students batch complete",
		slog.Int64("tenant_id", tenantID),
		slog.String("from", from),
		slog.String("to", to),
		slog.Int("transitions", transitions),
		slog.Int("candidates", len(studentIDs)),
	)
	return nil
}
