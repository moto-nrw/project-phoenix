package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// scheduleMaterializationTask registers the weekly timetable-materialization
// task when a Materializer has been wired in. No materializer → no task.
func (s *Scheduler) scheduleMaterializationTask() {
	if s.materializer == nil {
		s.getLogger().Info("timetable materialization not configured (no Materializer service)")
		return
	}

	s.registerTask("timetable-materialization", "1m-poll", s.runMaterializationTaskPolling)
}

// runMaterializationTaskPolling ticks every minute and delegates to
// checkAndRunMaterialization. Minute alignment matches the other scheduler
// tasks so HH:MM:00 ticks land deterministically.
func (s *Scheduler) runMaterializationTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in materialization task",
		"timetable materialization using minute-polling for per-tenant scheduling",
		s.checkAndRunMaterializationWithContext)
}

// checkAndRunMaterializationWithContext iterates active tenants and fires
// materialization for each tenant whose configured weekday matches today's ISO
// weekday, gated on the timetable.materialization_enabled setting.
func (s *Scheduler) checkAndRunMaterializationWithContext(ctx context.Context, task *ScheduledTask) {
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

	now := s.materializationTime()
	s.forEachTenantSettings(ctx, "materialization-check", func(tenantCtx context.Context, tenantID int64) error {
		return s.runMaterializationForTenant(tenantCtx, tenantID, now)
	})
}

func isoWeekdayMatches(wd int, now time.Time) bool {
	today := calendar.DateFromTime(now).Weekday()
	if today == time.Sunday {
		return wd == 7
	}
	return wd == int(today)
}

func (s *Scheduler) materializationTime() time.Time {
	if s.materializationNow != nil {
		return s.materializationNow()
	}
	return time.Now()
}

func (s *Scheduler) scheduleAutoStartTask() {
	if s.autoStart == nil {
		s.getLogger().Info("timetable auto-start tick not configured (missing service)")
		return
	}

	s.registerTask("timetable-auto-start", "1m-poll", s.runAutoStartTaskPolling)
}

func (s *Scheduler) runAutoStartTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in timetable auto-start task",
		"timetable auto-start tick using minute-polling",
		s.checkAndRunAutoStart)
}

func (s *Scheduler) checkAndRunAutoStart(ctx context.Context, task *ScheduledTask) {
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

	s.forEachTenantSettings(ctx, "timetable-auto-start", func(tenantCtx context.Context, tenantID int64) error {
		if !s.resolveBoolSetting(tenantCtx, settingTimetableEnabled, "", false) {
			return nil
		}
		if !s.resolveBoolSetting(tenantCtx, settingTimetableAutoStartPlanned, "", false) {
			return nil
		}

		result, err := s.autoStart.RunForTenant(tenantCtx, time.Now())
		if err != nil {
			return err
		}
		if result.Started > 0 || result.SkippedConflict > 0 || result.Failed > 0 {
			s.getLogger().Info("timetable auto-start completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("checked", result.Checked),
				slog.Int("started", result.Started),
				slog.Int("skipped_no_staff", result.SkippedNoStaff),
				slog.Int("skipped_duty", result.SkippedDuty),
				slog.Int("skipped_conflict", result.SkippedConflict),
				slog.Int("skipped_moved", result.SkippedMoved),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
		return nil
	})
}

// scheduleTimetableCleanupTask registers the shared daily timetable/calendar
// cleanup task when either cleanup service has been wired in.
func (s *Scheduler) scheduleTimetableCleanupTask() {
	if s.timetableCleanup == nil && s.calendarFeedCleanup == nil {
		s.getLogger().Info("timetable cleanup not configured")
		return
	}

	s.registerTask("timetable-cleanup", "1m-poll", s.runTimetableCleanupTaskPolling)
}

// runTimetableCleanupTaskPolling ticks every minute and defers to
// checkAndRunTimetableCleanup. Minute-aligned so HH:MM:00 ticks land
// deterministically.
func (s *Scheduler) runTimetableCleanupTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in timetable cleanup task",
		"timetable cleanup task using minute-polling for per-tenant scheduling",
		s.checkAndRunTimetableCleanup)
}

// checkAndRunTimetableCleanup evaluates each tenant's cleanup settings and
// runs timetable cleanup if the configured cleanup time matches now. Shares
// KeyDataCleanupEnabled + KeyDataCleanupTime + KeyDataCleanupTimeoutMinutes
// with the visits cleanup task — one admin switch for all nightly retention.
func (s *Scheduler) checkAndRunTimetableCleanup(ctx context.Context, task *ScheduledTask) {
	s.checkAndRunDailyGDPRCleanup(ctx, task, &s.lastTimetableCleanup, "timetable-cleanup-check", func(tenantCtx context.Context, tenantID int64, cleanupTime string) error {
		return s.runTimetableCleanupForTenant(tenantCtx, tenantID, cleanupTime)
	})
}

func (s *Scheduler) runTimetableCleanupForTenant(tenantCtx context.Context, tenantID int64, cleanupTime string) error {

	s.getLogger().Info("running timetable GDPR cleanup for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.String("cleanup_time", cleanupTime),
	)

	timeoutMinutes := s.resolveIntSetting(tenantCtx, settingDataCleanupTimeoutMinutes, "CLEANUP_SCHEDULER_TIMEOUT_MINUTES", 30)
	cleanupCtx, cleanupCancel := context.WithTimeout(tenantCtx, time.Duration(timeoutMinutes)*time.Minute)
	defer cleanupCancel()

	if s.timetableCleanup != nil {
		result, err := s.timetableCleanup.CleanupExpiredTimetableData(cleanupCtx)
		if err != nil {
			s.getLogger().Error("timetable cleanup failed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("timetable cleanup for tenant %d: %w", tenantID, err)
		}

		if result.InstancesDeleted > 0 || result.ExceptionsDeleted > 0 {
			s.getLogger().Info("timetable cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("instances_deleted", result.InstancesDeleted),
				slog.Int("exceptions_deleted", result.ExceptionsDeleted),
				slog.Int("students_affected", result.StudentsAffected),
				slog.Int("retention_days", result.RetentionDays),
				slog.Int64("duration_ms", result.DurationMS),
			)
		}
	}

	if s.calendarFeedCleanup != nil {
		deleted, err := s.calendarFeedCleanup.CleanupExpiredFeedTombstones(cleanupCtx)
		if err != nil {
			return fmt.Errorf("calendar feed cleanup for tenant %d: %w", tenantID, err)
		}
		if deleted > 0 {
			s.getLogger().Info("calendar feed cleanup completed for tenant",
				slog.Int64("tenant_id", tenantID),
				slog.Int("tombstones_deleted", deleted),
			)
		}
	}
	return nil
}

func (s *Scheduler) runMaterializationForTenant(tenantCtx context.Context, tenantID int64, now time.Time) error {

	enabled := s.resolveBoolSetting(tenantCtx, settingTimetableMaterializationEnabled, "", true)
	if !enabled {
		return nil
	}

	// Registry default is 5 (Friday, ISO 8601). The helper goes through
	// HasTenantOverride → ResolveInt → env → default, exactly matching
	// the documented fallback pattern.
	targetWeekday := s.resolveIntSetting(tenantCtx, settingTimetableMaterializationWeekday, "", 5)
	if !isoWeekdayMatches(targetWeekday, now) {
		return nil
	}

	if wasRunAt(&s.lastMaterialization, tenantID, now) {
		return nil
	}
	s.markRunAtAfterCommit(tenantCtx, &s.lastMaterialization, tenantID, now)

	weeksAhead := s.resolveIntSetting(tenantCtx, settingTimetableMaterializationWeeksAhead, "", 1)
	from, to := s.materializer.ResolveWindow(calendar.DateFromTime(now), weeksAhead)

	s.getLogger().Info("running timetable materialization for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.Int("target_weekday", targetWeekday),
		slog.Int("weeks_ahead", weeksAhead),
		slog.String("from", from.String()),
		slog.String("to", to.String()),
	)

	result, err := s.materializer.MaterializeScheduledWindow(tenantCtx, from, to)
	if err != nil {
		// Keep the today-mark so every subsequent minute does not retry a
		// known-failing run. It naturally expires on the next scheduler day.
		s.getLogger().Error("timetable materialization failed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if result.InstancesCreated > 0 || result.CandidatesRaced > 0 {
		s.getLogger().Info("timetable materialization completed for tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("created", result.InstancesCreated),
			slog.Int("skipped_existing", result.CandidatesSkippedExisting),
			slog.Int("raced", result.CandidatesRaced),
			slog.Int64("duration_ms", result.DurationMS),
		)
	}
	return nil
}
