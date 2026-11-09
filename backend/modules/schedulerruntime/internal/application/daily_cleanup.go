package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// scheduleCleanupTask schedules the daily cleanup task using minute-polling.
// Each minute, it checks each tenant's configured cleanup time and fires if matched.
func (s *Scheduler) scheduleCleanupTask() {
	// Env var can globally disable cleanup regardless of settings service
	if s.env("CLEANUP_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("cleanup scheduler is disabled via env var")
		return
	}
	// Legacy guard: without settings service, require explicit opt-in via env var
	if s.settings == nil && s.env("CLEANUP_SCHEDULER_ENABLED") != "true" {
		s.getLogger().Info("cleanup scheduler is disabled")
		return
	}

	s.registerTask("visit-cleanup", "1m-poll", s.runCleanupTaskPolling)
}

// runCleanupTaskPolling checks every minute if any tenant's cleanup time matches now.
func (s *Scheduler) runCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in cleanup task",
		"cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunCleanup)
}

// checkAndRunDailyGDPRCleanup is the shared per-tenant gate for the nightly
// retention jobs. They share
// KeyDataCleanupEnabled + KeyDataCleanupTime — one switch and one nightly
// window for all retention. A job runs at the configured minute unless it
// already succeeded today, and catches up at any minute once a scheduled run
// was missed (#3950, see cleanupDue). The success is stored in the tenant
// transaction, so a rollback leaves the run due. Returning an error through
// forEachCleanupTenantSettings also rolls back cleanup audit rows and their
// corresponding deletes atomically. A run that reports errCleanupIncomplete
// commits the steps that succeeded and stays due (#6288). Every failed run
// extends the school's failure streak, which spaces the retries and raises
// the alert (#6327, see cleanup_failures.go).
func (s *Scheduler) checkAndRunDailyGDPRCleanup(ctx context.Context, task *ScheduledTask, dayCache *sync.Map, opName string, runForTenant func(ctx context.Context, tenantID int64, cleanupTime string) error) {
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

	ctx, cancel := s.taskContext(ctx, 2*time.Hour)
	defer cancel()

	attempts := newCleanupAttempts()
	completed := s.forEachCleanupTenantSettings(ctx, opName, func(tenantCtx context.Context, tenantID int64) error {
		return s.runDailyGDPRCleanupForTenant(tenantCtx, tenantID, dayCache, task.Name, attempts, runForTenant)
	})
	s.settleCleanupAttempts(task.Name, attempts, completed)
}

func (s *Scheduler) dataCleanupEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, errors.New("data cleanup settings are not configured")
	}

	enabled, err := s.settings.ResolveBool(ctx, settingDataCleanupEnabled)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", settingDataCleanupEnabled, err)
	}
	return enabled, nil
}

// checkAndRunCleanup evaluates each tenant's cleanup settings and runs if time matches.
func (s *Scheduler) checkAndRunCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastDataCleanup, "cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running data cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes, err := s.resolveRequiredPositiveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", settingDataCleanupTimeoutMinutes, err)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		failed, err := s.executeCleanupForTenant(cleanupCtx, tenantID)
		if err != nil {
			return fmt.Errorf("data cleanup failed for tenant %d: %w", tenantID, err)
		}
		if len(failed) > 0 {
			return &cleanupIncompleteError{tenantID: tenantID, steps: failed}
		}
		return nil
	})
}

// errCleanupIncomplete reports a nightly cleanup whose failed steps undid only
// their own writes. The other steps commit and the run stays due.
var errCleanupIncomplete = errors.New("data cleanup incomplete")

// errCleanupStepPartial reports a step that finished but failed for part of
// its records. A partial failure is a failure: its savepoint rolls back and
// the run stays due (#6288).
var errCleanupStepPartial = errors.New("cleanup step failed for part of its records")

// cleanupStep is one step of the nightly data cleanup, named as it is logged.
type cleanupStep struct {
	name string
	run  func(context.Context, int64) error
}

// executeCleanupForTenant runs the cleanup steps for a single tenant under
// the invariants of #6288: every step runs in its own savepoint; a step's
// result is success or failure, a partial failure counts as failure; a failed
// step prevents no other step; the run stays due while any step failed, every
// step being required; every failure is logged with school and step.
// failed names the steps that failed; an error means the tenant transaction
// itself is unusable.
func (s *Scheduler) executeCleanupForTenant(ctx context.Context, tenantID int64) (failed []string, err error) {
	steps := []cleanupStep{
		{name: "visits", run: s.cleanupExpiredVisits},
		{name: "supervisors", run: s.cleanupStaleSupervisors},
		{name: "attendance", run: s.cleanupStaleAttendance},
		{name: "work_sessions", run: s.cleanupOpenWorkSessions},
		{name: "feedback", run: s.cleanupExpiredFeedback},
		{name: "rejected_enrollments", run: s.cleanupRejectedEnrollments},
		{name: "retention", run: s.runTenantRetention},
	}
	for _, step := range steps {
		usable, stepErr := s.inCleanupSavepoint(ctx, func(stepCtx context.Context) error { return step.run(stepCtx, tenantID) })
		if stepErr != nil {
			failed = append(failed, step.name)
			s.getLogger().Error("cleanup step failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("step", step.name),
				slog.String("error", stepErr.Error()),
			)
		}
		if !usable {
			return failed, stepErr
		}
	}
	return failed, nil
}

// inCleanupSavepoint runs one cleanup step in a savepoint of the tenant
// transaction; without a tenant runtime it runs the step directly.
func (s *Scheduler) inCleanupSavepoint(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if s.tenantRuntime == nil {
		return true, fn(ctx)
	}
	return s.tenantRuntime.WithSavepoint(ctx, fn)
}

func (s *Scheduler) cleanupExpiredVisits(ctx context.Context, tenantID int64) error {
	result, err := s.cleanupService.CleanupExpiredVisits(ctx)
	if err != nil {
		s.getLogger().Error("cleanup failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}

	if result == nil {
		return nil
	}
	if !result.Success {
		return errCleanupStepPartial
	}
	s.getLogger().Info("cleanup completed for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.Int("students_processed", result.StudentsProcessed),
		slog.Int64("records_deleted", result.RecordsDeleted),
	)
	return nil
}

func (s *Scheduler) runTenantRetention(ctx context.Context, tenantID int64) error {
	if s.retention == nil {
		return nil
	}
	if err := s.retention(ctx, tenantID); err != nil {
		s.getLogger().Error("retention run failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

// completeTimetableInstancesForEndedSessions closes the schedule-side rows
// linked to active.groups that the daily session-end job just closed.
//
// The active service owns active.groups/visits/supervisors; timetable
// instances live in schedule.*, so the scheduler bridges the two inside the
// tenant transaction created by the scheduler runtime. If this sync fails, callers
// return the error so the active close rolls back too instead of leaving the
// planner in a stale "active" state.
func (s *Scheduler) completeTimetableInstancesForEndedSessions(ctx context.Context, result *ports.DailySessionsEnded) (int, error) {
	if result == nil || len(result.EndedActiveGroupIDs) == 0 {
		return 0, nil
	}
	if s.instanceStudentRepo == nil || s.timetableBridge == nil {
		return 0, nil
	}

	now := time.Now()

	// The bulk visit close in EndDailySessions bypasses the per-visit
	// attendance syncer, so mirror the checkout into slot attendance here —
	// otherwise history/exports keep showing children as still checked in.
	if _, err := s.instanceStudentRepo.CloseOpenCheckoutsByActiveGroupIDs(ctx, result.EndedActiveGroupIDs, now); err != nil {
		return 0, fmt.Errorf("close open timetable checkouts: %w", err)
	}

	// The bridge finalizes attendance before it stamps the instances completed:
	// children the care plan does not place in the OGS that day are spared the
	// absent stamp (#1747), everybody else flips expected → absent. Shared with
	// the force-start path so no caller can complete an instance that still
	// carries expected rows.
	rows, err := s.timetableBridge.CompleteActiveByActiveGroupIDs(ctx, result.EndedActiveGroupIDs, now)
	if err != nil {
		return 0, fmt.Errorf("complete active timetable instances: %w", err)
	}
	return int(rows), nil
}

func (s *Scheduler) runDailyGDPRCleanupForTenant(tenantCtx context.Context, tenantID int64, dayCache *sync.Map, taskName string, attempts *cleanupAttempts, runForTenant func(context.Context, int64, string) error) error {
	enabled, err := s.dataCleanupEnabled(tenantCtx)
	if err != nil {
		return s.failUnreadableCleanupSwitch(tenantCtx, tenantID, dayCache, taskName, attempts, err)
	}
	if !enabled {
		return nil
	}

	cleanupTime, timeErr := s.settings.ResolveString(tenantCtx, settingDataCleanupTime)
	if timeErr != nil {
		return fmt.Errorf("resolve %s: %w", settingDataCleanupTime, errors.Join(err, timeErr))
	}
	now := s.cleanupNow()
	state, due, err := s.dailyCleanupDue(tenantCtx, dayCache, taskName, tenantID, cleanupTime, now)
	if err != nil || !due {
		return err
	}
	attempts.start(tenantID, now, state.Failures)

	if err := runForTenant(tenantCtx, tenantID, cleanupTime); err != nil {
		attempts.fail(tenantID, err)
		if errors.Is(err, errCleanupIncomplete) {
			// The failed steps undid their own writes: commit the others and
			// still report the run as failed.
			s.getLogger().Error("data cleanup incomplete, succeeded steps committed",
				slog.Int64("tenant_id", tenantID),
				slog.String("job_id", taskName),
				slog.String("error", err.Error()),
			)
			recordStandingJobFailure(tenantCtx, tenantID, "tenant cleanup incomplete", err, nil)
			addJobCommandFailure(tenantCtx, err)
			return s.storeCleanupFailure(tenantCtx, tenantID, taskName, attempts)
		}
		return err
	}
	if err := s.recordCleanupSuccess(tenantCtx, dayCache, taskName, tenantID, now); err != nil {
		attempts.fail(tenantID, err)
		return err
	}
	return nil
}

// failUnreadableCleanupSwitch fails the school's run when the cleanup switch
// cannot be read. A due run counts as failed and is retried after its wait.
// Nothing ran, so the transaction commits the failure streak and the job run
// still reports the failure.
func (s *Scheduler) failUnreadableCleanupSwitch(tenantCtx context.Context, tenantID int64, dayCache *sync.Map, taskName string, attempts *cleanupAttempts, err error) error {
	cleanupTime, timeErr := s.settings.ResolveString(tenantCtx, settingDataCleanupTime)
	if timeErr != nil {
		return fmt.Errorf("resolve %s: %w", settingDataCleanupTime, errors.Join(err, timeErr))
	}
	now := s.cleanupNow()
	state, due, dueErr := s.dailyCleanupDue(tenantCtx, dayCache, taskName, tenantID, cleanupTime, now)
	if dueErr != nil {
		return errors.Join(err, dueErr)
	}
	if due {
		attempts.start(tenantID, now, state.Failures)
		attempts.fail(tenantID, err)
		if storeErr := s.storeCleanupFailure(tenantCtx, tenantID, taskName, attempts); storeErr != nil {
			return errors.Join(err, storeErr)
		}
	}
	s.getLogger().Error("data cleanup switch unreadable",
		slog.Int64("tenant_id", tenantID),
		slog.String("job_id", taskName),
		slog.String("error", err.Error()),
	)
	recordStandingJobFailure(tenantCtx, tenantID, "tenant cleanup switch unreadable", err, nil)
	addJobCommandFailure(tenantCtx, err)
	return nil
}

func (s *Scheduler) cleanupStaleSupervisors(ctx context.Context, tenantID int64) error {
	supervisorResult, err := s.cleanupService.CleanupStaleSupervisors(ctx)
	if err != nil {
		s.getLogger().Error("supervisor cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	if supervisorResult == nil {
		return nil
	}
	if !supervisorResult.Success {
		return errCleanupStepPartial
	}
	if supervisorResult.RecordsClosed > 0 {
		s.getLogger().Info("supervisor cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("records_closed", supervisorResult.RecordsClosed),
		)
	}
	return nil
}

// cleanupStaleAttendance closes attendance records still open from previous
// days. A partial failure is a failure of the step.
func (s *Scheduler) cleanupStaleAttendance(ctx context.Context, tenantID int64) error {
	attendanceResult, err := s.cleanupService.CleanupStaleAttendance(ctx)
	if err != nil {
		s.getLogger().Error("attendance cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	switch {
	case attendanceResult == nil:
	case !attendanceResult.Success:
		s.getLogger().Warn("attendance cleanup partial failure",
			slog.Int64("tenant_id", tenantID),
			slog.Int("records_closed", attendanceResult.RecordsClosed),
			slog.Int("errors", len(attendanceResult.Errors)),
		)
		return errCleanupStepPartial
	case attendanceResult.RecordsClosed > 0:
		s.getLogger().Info("attendance cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("records_closed", attendanceResult.RecordsClosed),
			slog.Int("students_affected", attendanceResult.StudentsAffected),
		)
	}
	return nil
}

func (s *Scheduler) cleanupOpenWorkSessions(ctx context.Context, tenantID int64) error {
	if s.workSessionCleanup == nil {
		return nil
	}
	closedCount, err := s.workSessionCleanup.CleanupOpenSessions(ctx)
	if err != nil {
		s.getLogger().Error("work session cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	if closedCount > 0 {
		s.getLogger().Info("work session cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("sessions_closed", closedCount),
		)
	}
	return nil
}

// cleanupExpiredFeedback lets the Feedback owner resolve and enforce its
// retention setting. A failure leaves the nightly run due.
func (s *Scheduler) cleanupExpiredFeedback(ctx context.Context, tenantID int64) error {
	if s.feedbackCleaner == nil {
		return nil
	}
	deleted, err := s.feedbackCleaner.DeleteExpired(ctx)
	if err != nil {
		s.getLogger().Error("feedback cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	if deleted > 0 {
		s.getLogger().Info("feedback cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("records_deleted", deleted),
		)
	}
	return nil
}

func (s *Scheduler) cleanupRejectedEnrollments(ctx context.Context, tenantID int64) error {
	if s.enrollmentRejectedCleanup == nil {
		return nil
	}
	requests, lateInvites, outboxRows, err := s.enrollmentRejectedCleanup.CleanupRejectedEnrollments(ctx)
	if err != nil {
		s.getLogger().Error("rejected enrollment cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return err
	}
	if requests > 0 {
		s.getLogger().Info("rejected enrollment cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("requests_deleted", requests),
			slog.Int64("late_invites_deleted", lateInvites),
			slog.Int64("outbox_rows_deleted", outboxRows))
	}
	return nil
}
