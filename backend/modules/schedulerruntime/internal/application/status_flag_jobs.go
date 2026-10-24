package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// statusFlagClearDue reports whether now is the minute the end-of-day clear
// runs.
func statusFlagClearDue(now time.Time) bool {
	now = now.In(calendar.Berlin)
	return now.Hour() == statusFlagClearHour && now.Minute() == statusFlagClearMinute
}

// scheduleStatusFlagClearTask schedules a daily task to clear sick / excused
// flags for tenants whose operations.sick_clear_mode or
// operations.excused_clear_mode is set to "end_of_day". The task fires at
// the fixed end of the day (statusFlagClearHour:statusFlagClearMinute).
func (s *Scheduler) scheduleStatusFlagClearTask() {
	// Env var kill switch to allow ops to disable this task without code changes.
	if s.env("STATUS_FLAG_CLEAR_ENABLED") == "false" {
		s.getLogger().Info("status flag clear scheduler is disabled via env var")
		return
	}

	s.registerTask("status-flag-clear", "1m-poll", s.runStatusFlagClearTaskPolling)
}

// runStatusFlagClearTaskPolling checks every minute whether the end of the
// day has come and clears the configured end_of_day flags.
func (s *Scheduler) runStatusFlagClearTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in status flag clear task",
		"status flag clear task using minute-polling for per-tenant scheduling",
		s.checkAndRunStatusFlagClear)
}

// checkAndRunStatusFlagClear clears the end_of_day flags when the fixed end
// of the day has come.
func (s *Scheduler) checkAndRunStatusFlagClear(ctx context.Context, task *ScheduledTask) {
	if !statusFlagClearDue(time.Now()) {
		return
	}
	s.runStatusFlagClear(ctx, task)
}

// runStatusFlagClear evaluates each tenant's clear_mode settings and clears
// the flags set to end_of_day, at most once per tenant and day.
func (s *Scheduler) runStatusFlagClear(ctx context.Context, task *ScheduledTask) {
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

	s.forEachTenantSettings(ctx, "status-flag-clear", func(tenantCtx context.Context, tenantID int64) error {
		if wasRunToday(&s.lastStatusFlagClear, tenantID) {
			return nil
		}
		sickMode := s.resolveStringSetting(tenantCtx, settingSickClearMode, clearModeEndOfDay)
		excusedMode := s.resolveStringSetting(tenantCtx, settingExcusedClearMode, clearModeEndOfDay)
		succeeded := sickMode != clearModeEndOfDay || s.clearEndOfDayFlag(tenantCtx, tenantID, "sick", "sick_since")
		if excusedMode == clearModeEndOfDay && !s.clearEndOfDayFlag(tenantCtx, tenantID, "excused", "excused_since") {
			succeeded = false
		}

		if succeeded {
			s.markRunTodayAfterCommit(tenantCtx, &s.lastStatusFlagClear, tenantID)
		}

		return nil
	})
}

// clearStatusFlag archives a boolean flag + its timestamp for every student in
// the current tenant transaction. Column names are trusted constants — the
// caller picks them from a fixed set.
func (s *Scheduler) clearStatusFlag(ctx context.Context, flagColumn, sinceColumn string) (int64, error) {
	if s.studentStatusDayRepo == nil {
		return 0, fmt.Errorf("scheduler student status day repo not configured")
	}
	return s.studentStatusDayRepo.ArchiveEndOfDayStatusFlag(ctx, flagColumn, sinceColumn, calendar.TodayDate(), time.Now())
}

func (s *Scheduler) clearEndOfDayFlag(ctx context.Context, tenantID int64, flagColumn, sinceColumn string) bool {
	affected, err := s.clearStatusFlag(ctx, flagColumn, sinceColumn)
	if err != nil {
		s.getLogger().Error("end-of-day "+flagColumn+" clear failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}
	if affected > 0 {
		s.getLogger().Info("end-of-day "+flagColumn+" clear completed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("students_cleared", affected),
		)
	}
	return true
}
