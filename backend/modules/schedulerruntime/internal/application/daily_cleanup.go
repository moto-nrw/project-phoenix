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
// retention jobs (visits, timetable, time-tracking). All three share
// KeyDataCleanupEnabled + KeyDataCleanupTime — one switch and one nightly
// window for all retention. A job runs at the configured minute unless it
// already succeeded today, and catches up at any minute once a scheduled run
// was missed (#3950, see cleanupDue). The success is stored in the tenant
// transaction, so a rollback leaves the run due. Returning an error through
// forEachTenantSettings also rolls back cleanup audit rows and their
// corresponding deletes atomically.
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

	s.forEachTenantSettings(ctx, opName, func(tenantCtx context.Context, tenantID int64) error {
		return s.runDailyGDPRCleanupForTenant(tenantCtx, tenantID, dayCache, task.Name, runForTenant)
	})
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

		timeoutMinutes := s.resolveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes, 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		if !s.executeCleanupForTenant(cleanupCtx, tenantID) {
			return fmt.Errorf("data cleanup failed for tenant %d", tenantID)
		}
		return nil
	})
}

// executeCleanupForTenant runs the cleanup operations for a single tenant.
// Returns true if the primary cleanup (expired visits) succeeded.
func (s *Scheduler) executeCleanupForTenant(ctx context.Context, tenantID int64) bool {
	// Cleanup expired visits
	result, err := s.cleanupService.CleanupExpiredVisits(ctx)
	if err != nil {
		s.getLogger().Error("cleanup failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}

	s.getLogger().Info("cleanup completed for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.Int("students_processed", result.StudentsProcessed),
		slog.Int64("records_deleted", result.RecordsDeleted),
	)

	s.cleanupStaleSupervisors(ctx, tenantID)
	s.cleanupStaleAttendance(ctx, tenantID)
	s.cleanupOpenWorkSessions(ctx, tenantID)
	if !s.cleanupTenantOwnedData(ctx, tenantID) {
		return false
	}

	if s.retention != nil {
		if err := s.retention(ctx, tenantID); err != nil {
			s.getLogger().Error("retention run failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return false
		}
	}

	return true
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

func (s *Scheduler) runDailyGDPRCleanupForTenant(tenantCtx context.Context, tenantID int64, dayCache *sync.Map, taskName string, runForTenant func(context.Context, int64, string) error) error {

	enabled, err := s.dataCleanupEnabled(tenantCtx)
	if err != nil {
		cleanupTime := s.resolveStringSetting(tenantCtx, settingDataCleanupTime, "02:00")
		now := s.cleanupNow()
		due, dueErr := s.dailyCleanupDue(tenantCtx, dayCache, taskName, tenantID, cleanupTime, now)
		if dueErr != nil {
			return errors.Join(err, dueErr)
		}
		if due {
			s.noteCleanupFailure(taskName, tenantID, now)
		}
		return err
	}
	if !enabled {
		return nil
	}

	cleanupTime := s.resolveStringSetting(tenantCtx, settingDataCleanupTime, "02:00")
	now := s.cleanupNow()
	due, err := s.dailyCleanupDue(tenantCtx, dayCache, taskName, tenantID, cleanupTime, now)
	if err != nil || !due {
		return err
	}

	if err := runForTenant(tenantCtx, tenantID, cleanupTime); err != nil {
		s.noteCleanupFailure(taskName, tenantID, now)
		return err
	}
	if err := s.recordCleanupSuccess(tenantCtx, dayCache, taskName, tenantID, now); err != nil {
		s.noteCleanupFailure(taskName, tenantID, now)
		return err
	}
	return nil
}

func (s *Scheduler) cleanupStaleSupervisors(ctx context.Context, tenantID int64) {
	// Cleanup stale supervisors
	if supervisorResult, err := s.cleanupService.CleanupStaleSupervisors(ctx); err != nil {
		s.getLogger().Error("supervisor cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	} else if supervisorResult.RecordsClosed > 0 {
		s.getLogger().Info("supervisor cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("records_closed", supervisorResult.RecordsClosed),
		)
	}

}

func (s *Scheduler) cleanupStaleAttendance(ctx context.Context, tenantID int64) {
	// Cleanup stale attendance records (open from previous days)
	if attendanceResult, err := s.cleanupService.CleanupStaleAttendance(ctx); err != nil {
		s.getLogger().Error("attendance cleanup failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	} else if attendanceResult != nil {
		if !attendanceResult.Success {
			s.getLogger().Warn("attendance cleanup partial failure",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_closed", attendanceResult.RecordsClosed),
				slog.Int("errors", len(attendanceResult.Errors)),
			)
		} else if attendanceResult.RecordsClosed > 0 {
			s.getLogger().Info("attendance cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_closed", attendanceResult.RecordsClosed),
				slog.Int("students_affected", attendanceResult.StudentsAffected),
			)
		}
	}

}

func (s *Scheduler) cleanupOpenWorkSessions(ctx context.Context, tenantID int64) {
	// Cleanup open work sessions
	if s.workSessionCleanup != nil {
		if closedCount, err := s.workSessionCleanup.CleanupOpenSessions(ctx); err != nil {
			s.getLogger().Error("work session cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		} else if closedCount > 0 {
			s.getLogger().Info("work session cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("sessions_closed", closedCount),
			)
		}
	}

}

func (s *Scheduler) cleanupTenantOwnedData(ctx context.Context, tenantID int64) bool {
	// The Feedback owner resolves and enforces its retention setting. A failed
	// cleanup aborts the tenant transaction instead of being logged and ignored.
	if s.feedbackCleaner != nil {
		deleted, err := s.feedbackCleaner.DeleteExpired(ctx)
		if err != nil {
			s.getLogger().Error("feedback cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return false
		}
		if deleted > 0 {
			s.getLogger().Info("feedback cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("records_deleted", deleted),
			)
		}
	}

	if s.enrollmentRejectedCleanup != nil {
		requests, lateInvites, outboxRows, cleanupErr := s.enrollmentRejectedCleanup.CleanupRejectedEnrollments(ctx)
		if cleanupErr != nil {
			s.getLogger().Error("rejected enrollment cleanup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", cleanupErr.Error()))
			return false
		}
		if requests > 0 {
			s.getLogger().Info("rejected enrollment cleanup completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("requests_deleted", requests),
				slog.Int64("late_invites_deleted", lateInvites),
				slog.Int64("outbox_rows_deleted", outboxRows))
		}
	}

	return true
}
