package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// scheduleSessionEndTask schedules the daily session end task using minute-polling.
func (s *Scheduler) scheduleSessionEndTask() {
	// Env var can globally disable session end regardless of settings service
	if s.env("SESSION_END_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("session end scheduler is disabled via env var")
		return
	}
	// Legacy guard: without settings service, require non-false env var
	if s.settings == nil && s.env("SESSION_END_SCHEDULER_ENABLED") == "false" {
		s.getLogger().Info("session end scheduler is disabled")
		return
	}

	s.registerTask("session-end", "1m-poll", s.runSessionEndTaskPolling)
}

// runSessionEndTaskPolling checks every minute if any tenant's session end time matches now.
func (s *Scheduler) runSessionEndTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in session end task",
		"session end task using minute-polling for per-tenant scheduling",
		s.checkAndRunSessionEnd)
}

// checkAndRunSessionEnd evaluates each tenant's session end settings and runs if time matches.
func (s *Scheduler) checkAndRunSessionEnd(ctx context.Context, task *ScheduledTask) {
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

	s.forEachTenantSettings(ctx, "session-end-check", func(tenantCtx context.Context, tenantID int64) error {
		enabled := s.resolveBoolSetting(tenantCtx, settingSessionEndEnabled, "SESSION_END_SCHEDULER_ENABLED", true)
		if !enabled {
			return nil
		}

		endTime := s.resolveStringSetting(tenantCtx, settingSessionEndTime, "SESSION_END_TIME", "18:00")
		if !timeMatchesNow(endTime) {
			return nil
		}

		if wasRunToday(&s.lastSessionEnd, tenantID) {
			return nil
		}

		s.getLogger().Info("running session end for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("session_end_time", endTime),
		)

		ok, err := s.executeSessionEndForTenant(tenantCtx, tenantID)
		if err != nil {
			return err
		}
		if !ok {
			return nil // failure already logged; retry on the next matching minute
		}

		s.markRunTodayAfterCommit(tenantCtx, &s.lastSessionEnd, tenantID)
		return nil
	})
}

// executeSessionEndForTenant ends the tenant's daily sessions and completes
// the linked timetable instances. Session-end errors are logged and swallowed
// (ok=false, nil error) so other tenants still run; timetable-sync errors are
// returned so the caller's tenant transaction rolls back. ok is true only on
// full success — callers use it to decide whether to mark today as done.
func (s *Scheduler) executeSessionEndForTenant(ctx context.Context, tenantID int64) (bool, error) {
	timeoutMinutes := s.resolveIntSetting(ctx, settingSessionEndTimeoutMinutes, "SESSION_END_TIMEOUT_MINUTES", 10)
	endCtx, endCancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer endCancel()

	result, err := s.activeService.EndDailySessions(endCtx)
	if err != nil {
		s.getLogger().Error("session end failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false, nil // don't fail other tenants
	}
	timetableCompleted, err := s.completeTimetableInstancesForEndedSessions(endCtx, result)
	if err != nil {
		s.getLogger().Error("session end timetable sync failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false, err
	}

	s.getLogger().Info("session end completed for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.Int("sessions_ended", result.SessionsEnded),
		slog.Int("visits_ended", result.VisitsEnded),
		slog.Int("supervisors_ended", result.SupervisorsEnded),
		slog.Int("timetable_instances_completed", timetableCompleted),
	)
	return true, nil
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task.
// Uses a fixed 5-minute polling interval, resolves per-tenant settings on each tick.
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Quick check: if globally disabled via env and no settings service, skip
	if s.settings == nil && s.env("SESSION_CLEANUP_ENABLED") == "false" {
		s.getLogger().Info("session cleanup is disabled")
		return
	}

	s.registerTask("session-cleanup", "5m-poll", s.runSessionCleanupTaskPolling)
}

// runSessionCleanupTaskPolling checks every 5 minutes if any tenant needs session cleanup.
func (s *Scheduler) runSessionCleanupTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in session cleanup task",
		"session cleanup using 5-minute polling for per-tenant scheduling",
		30*time.Second, func() time.Duration { return 5 * time.Minute }, s.checkAndRunSessionCleanup)
}

// checkAndRunSessionCleanup evaluates each tenant's session cleanup settings.
func (s *Scheduler) checkAndRunSessionCleanup(ctx context.Context, task *ScheduledTask) {
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

	s.forEachTenantSettings(ctx, "session-cleanup", func(tenantCtx context.Context, tenantID int64) error {
		return s.runSessionCleanupForTenant(tenantCtx, tenantID)
	})
}

func (s *Scheduler) runSessionCleanupForTenant(tenantCtx context.Context, tenantID int64) error {

	enabled := s.resolveBoolSetting(tenantCtx, settingSessionCleanupEnabled, "SESSION_CLEANUP_ENABLED", true)
	if !enabled {
		return nil
	}

	intervalMinutes := s.resolveIntSetting(tenantCtx, settingSessionCleanupIntervalMinutes, "SESSION_CLEANUP_INTERVAL_MINUTES", 15)
	thresholdMinutes := s.resolveIntSetting(tenantCtx, settingSessionAbandonedThresholdMin, "SESSION_ABANDONED_THRESHOLD_MINUTES", 60)

	// Check if enough time has passed since last run for this tenant
	if val, ok := s.lastSessionCleanup.Load(tenantID); ok {
		if lastRun, ok := val.(time.Time); ok {
			if time.Since(lastRun) < time.Duration(intervalMinutes)*time.Minute {
				return nil // not yet time for this tenant
			}
		}
	}

	threshold := time.Duration(thresholdMinutes) * time.Minute
	count, err := s.activeService.CleanupAbandonedSessions(tenantCtx, threshold)
	if err != nil {
		return err
	}

	if count > 0 {
		s.getLogger().Info("session cleanup completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int("abandoned_sessions", count),
			slog.Int("threshold_minutes", thresholdMinutes),
		)
	}

	s.markRunAtAfterCommit(tenantCtx, &s.lastSessionCleanup, tenantID, time.Now())
	return nil
}
