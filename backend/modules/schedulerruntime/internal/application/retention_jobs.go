package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// scheduleTimeTrackingCleanupTask registers the daily time-tracking cleanup
// when a TimeTrackingCleanupService has been wired in. Nil → no task.
func (s *Scheduler) scheduleTimeTrackingCleanupTask() {
	if s.timeTrackingCleanup == nil {
		s.getLogger().Info("time-tracking GDPR cleanup not configured (no TimeTrackingCleanupService)")
		return
	}

	s.registerTask("time-tracking-cleanup", "1m-poll", s.runTimeTrackingCleanupTaskPolling)
}

// runTimeTrackingCleanupTaskPolling ticks every minute and defers to
// checkAndRunTimeTrackingCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runTimeTrackingCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in time-tracking cleanup task",
		"time-tracking cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunTimeTrackingCleanup)
}

// checkAndRunTimeTrackingCleanup evaluates each tenant's cleanup settings and
// runs time-tracking cleanup if the configured cleanup time matches now.
// Shares KeyDataCleanupEnabled + KeyDataCleanupTime + KeyDataCleanupTimeoutMinutes
// with the visits and timetable cleanup tasks — one admin switch for all
// nightly retention.
func (s *Scheduler) checkAndRunTimeTrackingCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastTimeTrackingCleanup, "time-tracking-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running time-tracking GDPR cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes, 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.timeTrackingCleanup.CleanupExpiredTimeTrackingData(cleanupCtx)
		if err != nil {
			s.getLogger().Error("time-tracking cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("time-tracking cleanup for tenant %d: %w", tenantID, err)
		}

		if result.SessionsDeleted > 0 || result.AbsencesDeleted > 0 {
			s.getLogger().Info("time-tracking cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("sessions_deleted", result.SessionsDeleted),
				slog.Int("absences_deleted", result.AbsencesDeleted),
				slog.Int("staff_affected", result.StaffAffected),
				slog.Int("retention_days", result.RetentionDays),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

// scheduleStudentChangeLogCleanupTask registers the daily change-history
// cleanup when a StudentChangeLogCleanupService has been wired in. Nil → no
// task.
func (s *Scheduler) scheduleStudentChangeLogCleanupTask() {
	if s.studentChangeLogCleanup == nil {
		s.getLogger().Info("student change-log GDPR cleanup not configured (no StudentChangeLogCleanupService)")
		return
	}

	s.registerTask("student-change-log-cleanup", "1m-poll", s.runStudentChangeLogCleanupTaskPolling)
}

// runStudentChangeLogCleanupTaskPolling ticks every minute and defers to
// checkAndRunStudentChangeLogCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runStudentChangeLogCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in student change-log cleanup task",
		"student change-log cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunStudentChangeLogCleanup)
}

// checkAndRunStudentChangeLogCleanup evaluates each tenant's cleanup settings
// and runs change-history cleanup if the configured cleanup time matches now.
// Shares the same data-cleanup toggle/time/timeout as the other retention
// jobs — one admin switch for all nightly retention.
func (s *Scheduler) checkAndRunStudentChangeLogCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastStudentChangeLogCleanup, "student-change-log-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		s.getLogger().Info("running student change-log GDPR cleanup for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("cleanup_time", cleanupTime),
		)

		timeoutMinutes := s.resolveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes, 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.studentChangeLogCleanup(cleanupCtx)
		if err != nil {
			s.getLogger().Error("student change-log cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("student change-log cleanup for tenant %d: %w", tenantID, err)
		}

		if result.EditsDeleted > 0 {
			s.getLogger().Info("student change-log cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("edits_deleted", result.EditsDeleted),
				slog.Int("students_affected", result.StudentsAffected),
				slog.Int("retention_days", result.RetentionDays),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

// schedulePWAUsageCleanupTask registers the daily PWA usage cleanup when a
// cleanup has been wired in. Nil → no task.
func (s *Scheduler) schedulePWAUsageCleanupTask() {
	if s.pwaUsageCleanup == nil {
		s.getLogger().Info("pwa usage GDPR cleanup not configured (no cleanup capability)")
		return
	}

	s.registerTask("pwa-usage-cleanup", "1m-poll", s.runPWAUsageCleanupTaskPolling)
}

// runPWAUsageCleanupTaskPolling ticks every minute and defers to
// checkAndRunPWAUsageCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runPWAUsageCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in pwa usage cleanup task",
		"pwa usage cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunPWAUsageCleanup)
}

// checkAndRunPWAUsageCleanup evaluates each tenant's cleanup settings and
// sweeps stale standalone-usage rows when the configured cleanup time
// matches now. Shares the same data-cleanup toggle/time/timeout as the
// other retention jobs.
func (s *Scheduler) checkAndRunPWAUsageCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastPWAUsageCleanup, "pwa-usage-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		timeoutMinutes := s.resolveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes, 30)
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.pwaUsageCleanup(cleanupCtx)
		if err != nil {
			s.getLogger().Error("pwa usage cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("pwa usage cleanup for tenant %d: %w", tenantID, err)
		}

		if result.RowsDeleted > 0 {
			s.getLogger().Info("pwa usage cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("cleanup_time", cleanupTime),
				slog.Int("rows_deleted", result.RowsDeleted),
				slog.Int("retention_days", result.RetentionDays),
			)
		}
		return nil
	})
}

// scheduleStaffMessageCleanupTask registers the daily retention sweep for the
// OGS-internal colleague chat (#2598) when a cleanup service is wired in.
// Nil → no task.
func (s *Scheduler) scheduleStaffMessageCleanupTask() {
	if s.staffMessageCleanup == nil {
		s.getLogger().Info("staff message GDPR cleanup not configured (no cleanup capability)")
		return
	}

	s.registerTask("staff-message-cleanup", "1m-poll", s.runStaffMessageCleanupTaskPolling)
}

// runStaffMessageCleanupTaskPolling ticks every minute and defers to
// checkAndRunStaffMessageCleanup.
func (s *Scheduler) runStaffMessageCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in staff message cleanup task",
		"staff message cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunStaffMessageCleanup)
}

// checkAndRunStaffMessageCleanup sweeps expired internal staff messages per
// tenant, sharing the same data-cleanup toggle/time/timeout as the other
// retention jobs.
func (s *Scheduler) checkAndRunStaffMessageCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastStaffMessageCleanup, "staff-message-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		timeoutMinutes, err := s.resolveRequiredPositiveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes)
		if err != nil {
			s.getLogger().Error("staff message cleanup timeout setting failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("key", settingDataCleanupTimeoutMinutes),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("staff message cleanup timeout for tenant %d: %w", tenantID, err)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
		defer cleanupCancel()

		result, err := s.staffMessageCleanup(cleanupCtx)
		if err != nil {
			s.getLogger().Error("staff message cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("staff message cleanup for tenant %d: %w", tenantID, err)
		}

		if result.MessagesDeleted > 0 || result.ThreadsDeleted > 0 {
			s.getLogger().Info("staff message cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("cleanup_time", cleanupTime),
				slog.Int("messages_deleted", result.MessagesDeleted),
				slog.Int("threads_deleted", result.ThreadsDeleted),
				slog.Int("retention_days", result.RetentionDays),
			)
		}
		return nil
	})
}
