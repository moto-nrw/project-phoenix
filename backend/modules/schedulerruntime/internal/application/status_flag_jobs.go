package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A live sick or excused flag stands for the status day of its since
// timestamp (#6291). Two Berlin boundaries move that day on: the fixed end of
// the day (statusFlagClearHour:statusFlagClearMinute), after which the
// end_of_day mode archives the day's flags, and midnight, after which a flag
// of an earlier day is either archived for that day (end_of_day) or carried
// into the new day (manual, next_checkin). The run is due once per boundary
// and school; a run that comes late, after a restart or a failure, still
// dates every flag on its own day.

// statusFlagEndOfDay is the fixed end of the status day on the Berlin
// calendar day of now, exact across daylight saving changes.
func statusFlagEndOfDay(now time.Time) time.Time {
	now = now.In(calendar.Berlin)
	return time.Date(now.Year(), now.Month(), now.Day(), statusFlagClearHour, statusFlagClearMinute, 0, 0, calendar.Berlin)
}

// statusFlagBoundary is the latest boundary at or before now: today's end of
// the day once it has come, today's Berlin midnight before.
func statusFlagBoundary(now time.Time) time.Time {
	if end := statusFlagEndOfDay(now); !now.Before(end) {
		return end
	}
	return calendar.DateFromTime(now).BerlinMidnight()
}

// statusFlagClearDue reports whether a school whose last successful run was
// last (hasLast false: none in this process) runs at now: when a boundary
// has passed since. A missed boundary is therefore caught up at the next
// poll, and a restart runs once.
func statusFlagClearDue(now, last time.Time, hasLast bool) bool {
	return !hasLast || last.Before(statusFlagBoundary(now))
}

// statusFlagArchiveWindow returns, for an end_of_day run at now, the instant
// before which a flag's status day has ended and the status day of a flag
// without a since timestamp. After the end of the day every flag reported
// so far belongs to an ended day; before it only the flags of earlier days.
func statusFlagArchiveWindow(now time.Time) (time.Time, calendar.Date) {
	today := calendar.DateFromTime(now)
	if !now.Before(statusFlagEndOfDay(now)) {
		return now, today
	}
	return today.BerlinMidnight(), today.AddDays(-1)
}

// scheduleStatusFlagClearTask schedules the task that keeps the sick /
// excused flags and their status days in step for every school, according
// to operations.sick_clear_mode and operations.excused_clear_mode.
func (s *Scheduler) scheduleStatusFlagClearTask() {
	// Env var kill switch to allow ops to disable this task without code changes.
	if s.env("STATUS_FLAG_CLEAR_ENABLED") == "false" {
		s.getLogger().Info("status flag clear scheduler is disabled via env var")
		return
	}

	s.registerTask("status-flag-clear", "1m-poll", s.runStatusFlagClearTaskPolling)
}

// runStatusFlagClearTaskPolling checks every minute whether a status-day
// boundary has passed since the last run.
func (s *Scheduler) runStatusFlagClearTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in status flag clear task",
		"status flag clear task using minute-polling for per-tenant scheduling",
		s.checkAndRunStatusFlagClear)
}

// checkAndRunStatusFlagClear runs the schools once a boundary has passed that
// not every school has run for yet.
func (s *Scheduler) checkAndRunStatusFlagClear(ctx context.Context, task *ScheduledTask) {
	now := schedulerTime(s.now)
	if s.statusFlagPass.Load() >= statusFlagBoundary(now).Unix() {
		return
	}
	s.runStatusFlagClear(ctx, task, now)
}

// runStatusFlagClear evaluates each school's clear modes at now, at most once
// per school and boundary. The pass counts as done when no school failed; a
// failed school stays due and is retried at the next poll.
func (s *Scheduler) runStatusFlagClear(ctx context.Context, task *ScheduledTask, now time.Time) {
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

	var failed atomic.Bool
	s.forEachTenantSettings(ctx, "status-flag-clear", func(tenantCtx context.Context, tenantID int64) error {
		if last, ok := s.lastStatusFlagClear.Load(tenantID); ok {
			if at, ok := last.(time.Time); ok && !statusFlagClearDue(now, at, true) {
				return nil
			}
		}
		sickMode := s.resolveStringSetting(tenantCtx, settingSickClearMode, clearModeEndOfDay)
		excusedMode := s.resolveStringSetting(tenantCtx, settingExcusedClearMode, clearModeEndOfDay)
		succeeded := s.rollStatusFlag(tenantCtx, tenantID, sickMode, "sick", "sick_since", now)
		if !s.rollStatusFlag(tenantCtx, tenantID, excusedMode, "excused", "excused_since", now) {
			succeeded = false
		}

		if succeeded {
			s.markRunAtAfterCommit(tenantCtx, &s.lastStatusFlagClear, tenantID, now)
		} else {
			failed.Store(true)
		}
		return nil
	})
	if !failed.Load() {
		s.statusFlagPass.Store(statusFlagBoundary(now).Unix())
	}
}

// rollStatusFlag applies one flag's clear mode at now: end_of_day archives
// the flags whose status day has ended, every other mode carries the flags of
// earlier days into today.
func (s *Scheduler) rollStatusFlag(ctx context.Context, tenantID int64, mode, flagColumn, sinceColumn string, now time.Time) bool {
	if s.studentStatusDayRepo == nil {
		s.logStatusFlagFailure(tenantID, flagColumn, fmt.Errorf("scheduler student status day repo not configured"))
		return false
	}
	if mode != clearModeEndOfDay {
		carried, err := s.studentStatusDayRepo.CarryStatusFlag(ctx, flagColumn, sinceColumn, calendar.DateFromTime(now), now)
		if err != nil {
			s.logStatusFlagFailure(tenantID, flagColumn, err)
			return false
		}
		if carried > 0 {
			s.getLogger().Info(flagColumn+" status carried into the day",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("students_carried", carried),
			)
		}
		return true
	}
	before, date := statusFlagArchiveWindow(now)
	affected, err := s.studentStatusDayRepo.ArchiveEndOfDayStatusFlag(ctx, flagColumn, sinceColumn, before, date, now)
	if err != nil {
		s.logStatusFlagFailure(tenantID, flagColumn, err)
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

func (s *Scheduler) logStatusFlagFailure(tenantID int64, flagColumn string, err error) {
	s.getLogger().Error("end-of-day "+flagColumn+" clear failed",
		slog.Int64("tenant_id", tenantID),
		slog.String("error", err.Error()),
	)
}
