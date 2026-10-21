package application

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// scheduleBreakAutoEndTask schedules the break auto-end task.
// Runs at a fixed interval (minimum of configured values across tenants or default 60s).
func (s *Scheduler) scheduleBreakAutoEndTask() {
	if s.breakAutoEnder == nil {
		s.getLogger().Info("break auto-end not configured (no BreakAutoEnder service)")
		return
	}

	// Resolve interval from env var (global, not per-tenant)
	seconds := 60
	if val := s.env("BREAK_AUTO_END_INTERVAL_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			seconds = parsed
		}
	}

	s.registerTask("break-auto-end", fmt.Sprintf("%ds-poll", seconds), func(task *ScheduledTask) {
		s.runBreakAutoEndTaskPolling(task, seconds)
	})
}

// runBreakAutoEndTaskPolling runs break auto-end check at the configured interval for all tenants.
func (s *Scheduler) runBreakAutoEndTaskPolling(task *ScheduledTask, seconds int) {
	interval := time.Duration(seconds) * time.Second
	s.runIntervalPolling(task, "panic in break auto-end task", "break auto-end polling started",
		10*time.Second, func() time.Duration { return interval }, s.checkAndRunBreakAutoEnd,
		slog.Int("interval_seconds", seconds))
}

// checkAndRunBreakAutoEnd runs break auto-end for all tenants.
func (s *Scheduler) checkAndRunBreakAutoEnd(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, 30*time.Second)
	defer cancel()

	breakErr := s.forEachTenant(ctx, "break-auto-end", func(tenantCtx context.Context) error {
		count, err := s.breakAutoEnder.AutoEndExpiredBreaks(tenantCtx)
		if err != nil {
			return err
		}

		if count > 0 {
			s.getLogger().Info("break auto-end completed",
				slog.Int("breaks_ended", count))
		}
		return nil
	})
	if breakErr != nil {
		s.getLogger().Error("break auto-end failed", "error", breakErr)
	}
}

// scheduleAutoCheckoutTask schedules the auto-checkout-at-shift-end task
// (#1798). Runs on the same fixed 60-second poll as break auto-end; the
// feature itself is gated per tenant via tracking.auto_checkout_enabled
// (registry default false — pure opt-in).
func (s *Scheduler) scheduleAutoCheckoutTask() {
	if s.autoCheckouter == nil {
		s.getLogger().Info("auto-checkout not configured (no AutoCheckouter service)")
		return
	}

	s.registerTask("auto-checkout", "60s-poll", s.runAutoCheckoutTaskPolling)
}

// runAutoCheckoutTaskPolling runs the auto-checkout check every 60 seconds.
func (s *Scheduler) runAutoCheckoutTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in auto-checkout task", "auto-checkout polling started",
		10*time.Second, func() time.Duration { return 60 * time.Second }, s.checkAndRunAutoCheckout)
}

// checkAndRunAutoCheckout closes due sessions for every tenant that has the
// feature enabled.
func (s *Scheduler) checkAndRunAutoCheckout(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, 30*time.Second)
	defer cancel()

	s.forEachTenantSettings(ctx, "auto-checkout", func(tenantCtx context.Context, tenantID int64) error {
		// Opt-in per tenant; no env var fallback (new feature, settings-only).
		enabled := s.resolveBoolSetting(tenantCtx, settingTrackingAutoCheckoutEnabled, false)
		if !enabled {
			return nil
		}

		// Zero is valid here: grace 0 means checkout exactly at shift end.
		graceMinutes := s.resolveNonNegativeIntSetting(tenantCtx, settingTrackingAutoCheckoutGraceMinutes, 15)
		count, err := s.autoCheckouter.AutoCheckoutDueSessions(tenantCtx, time.Duration(graceMinutes)*time.Minute)
		if err != nil {
			return err
		}
		if count > 0 {
			s.getLogger().Info("auto-checkout completed",
				slog.Int64("tenant_id", tenantID),
				slog.Int("sessions_closed", count))
		}
		return nil
	})
}

func (s *Scheduler) scheduleAutoEndTask() {
	if s.autoEnd == nil {
		s.getLogger().Info("timetable auto-end tick not configured (missing service)")
		return
	}

	s.registerTask("timetable-auto-end", "1m-poll", s.runAutoEndTaskPolling)
}

func (s *Scheduler) runAutoEndTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in timetable auto-end task",
		"timetable auto-end tick using minute-polling",
		s.checkAndRunAutoEnd)
}

func (s *Scheduler) checkAndRunAutoEnd(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "timetable-auto-end", func(tenantCtx context.Context, tenantID int64) error {
		return s.runAutoEndForTenant(tenantCtx, tenantID)
	})
}

func (s *Scheduler) runAutoEndForTenant(ctx context.Context, tenantID int64) error {
	if !s.resolveBoolSetting(ctx, settingTimetableEnabled, true) ||
		!s.resolveBoolSetting(ctx, settingTimetableAutoEndEnabled, false) {
		return nil
	}
	graceMinutes := s.resolveIntSetting(ctx, settingTimetableAutoEndGraceMinutes, 0)
	result, err := s.autoEnd.RunForTenant(ctx, time.Now(), time.Duration(graceMinutes)*time.Minute)
	if err != nil {
		return fmt.Errorf("auto-end tenant %d: %w", tenantID, err)
	}
	if result.Failed > 0 {
		s.getLogger().Warn("timetable auto-end completed with failures",
			slog.Int64("tenant_id", tenantID),
			slog.Int("checked", result.Checked),
			slog.Int("completed", result.Completed),
			slog.Int("failed", result.Failed),
			slog.Int("grace_minutes", graceMinutes),
			slog.Int64("duration_ms", result.DurationMS),
		)
		return nil
	}
	if result.Completed > 0 || result.SkippedConcurrent > 0 {
		s.getLogger().Info("timetable auto-end completed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("checked", result.Checked),
			slog.Int("completed", result.Completed),
			slog.Int("skipped_concurrent", result.SkippedConcurrent),
			slog.Int("grace_minutes", graceMinutes),
			slog.Int64("duration_ms", result.DurationMS),
		)
	}
	return nil
}
