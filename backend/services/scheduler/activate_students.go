package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentLifecycleRepository is the narrow contract the activate-students tick
// needs from the student repository. Defined here so the scheduler does not
// import the full users repository package, matching the opt-in shape used by
// instanceRepo (overdue tick) and timetableCleanup (GDPR cleanup).
type StudentLifecycleRepository interface {
	FindPendingDueForActivation(ctx context.Context, asOf timezone.Date) ([]*userModels.Student, error)
	FindActiveDueForDeactivation(ctx context.Context, asOf timezone.Date) ([]*userModels.Student, error)
	UpdateStatus(ctx context.Context, studentID int64, newStatus userModels.StudentStatus) error
}

// StudentLifecycleAuditor records scheduler-authored status transitions.
type StudentLifecycleAuditor interface {
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after userModels.StudentStatus) error
}

// SetStudentLifecycleRepo wires the repository for the activate-students tick.
// Nil repo → no task registers, matching SetMaterializer's opt-in pattern.
func (s *Scheduler) SetStudentLifecycleRepo(repo StudentLifecycleRepository) {
	s.studentLifecycleRepo = repo
}

// SetStudentLifecycleAudit wires change-history recording for automated status
// transitions.
func (s *Scheduler) SetStudentLifecycleAudit(audit StudentLifecycleAuditor) {
	s.studentLifecycleAudit = audit
}

// scheduleActivateStudentsTask registers the per-tenant activate-students
// poll. The interval is settings-driven (operations.student_activation_interval_minutes,
// default 60). Date transitions only happen on day boundaries; the polling
// cadence is a safety-net for restarts and clock drift, not a precision dial.
func (s *Scheduler) scheduleActivateStudentsTask() {
	if s.studentLifecycleRepo == nil {
		s.getLogger().Info("activate-students task not configured (no StudentLifecycleRepository)")
		return
	}

	task := &ScheduledTask{
		Name:     "activate-students",
		Schedule: "interval-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runActivateStudentsTaskPolling(task)
}

// runActivateStudentsTaskPolling ticks at the configured per-tenant interval.
// The interval is read on each tick from a representative tenant so admins
// can shorten the cadence without restart. Tenants without an override get
// the registry default (60 minutes).
func (s *Scheduler) runActivateStudentsTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in activate-students task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("activate-students using interval polling for per-tenant scheduling")

	// Brief delay so the rest of the services finish booting before the first tick.
	// Honor s.done so shutdown during startup is responsive.
	select {
	case <-time.After(20 * time.Second):
	case <-s.done:
		return
	}
	s.checkAndRunActivateStudents(task)

	// Resolve interval from a representative tenant context. The setting is
	// per-tenant but the polling loop is global — we use the conservative
	// minimum across tenants on each tick by re-reading inside the loop.
	for {
		interval := s.resolveActivateStudentsInterval()
		select {
		case <-time.After(interval):
			s.checkAndRunActivateStudents(task)
		case <-s.done:
			return
		}
	}
}

// resolveActivateStudentsInterval reads the interval setting from a non-tenant
// context (registry default if no override). Per-tenant interval differences
// are not honored — the polling loop is global. This is acceptable because
// the tick is itself idempotent and tolerant of running more often than any
// individual tenant requested.
func (s *Scheduler) resolveActivateStudentsInterval() time.Duration {
	minutes := s.resolveIntSetting(context.Background(), configModel.KeyStudentActivationIntervalMin, "", 60)
	if minutes < 1 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

// checkAndRunActivateStudents iterates active tenants and runs activation +
// deactivation per tenant. Re-entry is guarded by task.Running so a slow
// tenant cannot cause overlapping ticks.
func (s *Scheduler) checkAndRunActivateStudents(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	now := time.Now()
	s.forEachTenantSettings(ctx, "activate-students", func(tenantCtx context.Context, tenantID int64) error {
		return s.runActivateStudentsForTenantWithError(tenantCtx, tenantID, now)
	})
}

// runActivateStudentsForTenant is the logging wrapper for direct callers.
// Tenant iteration uses the error-returning variant below so audit failures
// reach WithTenantTx and force a rollback.
func (s *Scheduler) runActivateStudentsForTenant(ctx context.Context, tenantID int64, now time.Time) {
	if err := s.runActivateStudentsForTenantWithError(ctx, tenantID, now); err != nil {
		s.getLogger().Error("activate-students: tenant transition failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	}
}

// runActivateStudentsForTenant flips eligible pending students to active and
// eligible active students to inactive. Idempotent — running twice on the
// same day is a no-op after the first pass. Status-update failures are logged
// and skipped. Audit failures abort the tenant transaction so an automated
// transition can never commit without its history entry.
func (s *Scheduler) runActivateStudentsForTenantWithError(ctx context.Context, tenantID int64, now time.Time) error {
	asOf := timezone.DateFromTime(now)

	pending, err := s.studentLifecycleRepo.FindPendingDueForActivation(ctx, asOf)
	if err != nil {
		s.getLogger().Error("activate-students: load pending failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	} else {
		if err := s.applyStatusTransitions(ctx, tenantID, pending,
			userModels.StudentStatusPending, userModels.StudentStatusActive); err != nil {
			return err
		}
	}

	dueInactive, err := s.studentLifecycleRepo.FindActiveDueForDeactivation(ctx, asOf)
	if err != nil {
		s.getLogger().Error("activate-students: load active-due failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return s.applyStatusTransitions(ctx, tenantID, dueInactive,
		userModels.StudentStatusActive, userModels.StudentStatusInactive)
}

// applyStatusTransitions updates each student to newStatus and emits a slog
// info entry per transition. Status-update errors are logged and skipped;
// audit errors abort the tenant transaction. GDPR: student IDs only — no names
// at info level (CLAUDE.md backend logging rule).
func (s *Scheduler) applyStatusTransitions(ctx context.Context, tenantID int64, students []*userModels.Student, from, to userModels.StudentStatus) error {
	if len(students) == 0 {
		return nil
	}
	for _, student := range students {
		if err := s.studentLifecycleRepo.UpdateStatus(ctx, student.ID, to); err != nil {
			s.getLogger().Error("activate-students: update status failed",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", student.ID),
				slog.String("from", string(from)),
				slog.String("to", string(to)),
				slog.String("error", err.Error()),
			)
			continue
		}
		if s.studentLifecycleAudit != nil {
			if err := s.studentLifecycleAudit.RecordSystemStatusChange(ctx, student.ID, from, to); err != nil {
				s.getLogger().Error("activate-students: audit status transition failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", student.ID),
					slog.String("from", string(from)),
					slog.String("to", string(to)),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("audit student %d status transition: %w", student.ID, err)
			}
		}
		s.getLogger().Info("student status transition",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", student.ID),
			slog.String("from", string(from)),
			slog.String("to", string(to)),
		)
	}
	s.getLogger().Info("activate-students batch complete",
		slog.Int64("tenant_id", tenantID),
		slog.String("from", string(from)),
		slog.String("to", string(to)),
		slog.Int("transitions", len(students)),
	)
	return nil
}
