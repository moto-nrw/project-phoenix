package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A live sick or excused flag stands for the status day of its since
// timestamp (#6291). Two Berlin boundaries move that day on: the fixed end of
// the day (statusFlagClearHour:statusFlagClearMinute), after which the
// end_of_day mode archives the day's flags, and midnight, after which a flag
// of an earlier day is either archived for that day (end_of_day) or carried
// into the new day (manual, next_checkin). The run is due once per boundary
// and school; a run that comes late, after a restart or a failure, still
// dates every flag on its own day. The last success of each school is kept
// in the cleanup-run ledger (#3950), so a run after an outage of several
// days carries a flag into every day it missed (#6291).

// statusFlagJobID names the run in the cleanup-run ledger.
const statusFlagJobID = "status-flag-clear"

// statusFlagCatchUpDays bounds how many days a run after an outage carries a
// flag into: two school weeks. A longer Worker outage is an incident with its
// own alert; the bound keeps one run within its task timeout (at most this
// many rows per flagged child), and the skipped days are logged.
const statusFlagCatchUpDays = 14

// statusFlagCarryDays returns the Berlin days a carry run at now covers: every
// day after the day of the last success up to today, at most the latest
// statusFlagCatchUpDays of them; without a recorded success only today.
// skipped counts the older days the bound left out.
func statusFlagCarryDays(now, last time.Time, hasLast bool) (days []calendar.Date, skipped int) {
	today := calendar.DateFromTime(now)
	first := today
	if hasLast {
		if after := calendar.DateFromTime(last).AddDays(1); after.Before(today) {
			first = after
		}
	}
	if span := first.DaysUntil(today) + 1; span > statusFlagCatchUpDays {
		skipped = span - statusFlagCatchUpDays
		first = today.AddDays(-(statusFlagCatchUpDays - 1))
	}
	for day := first; !day.After(today); day = day.AddDays(1) {
		days = append(days, day)
	}
	return days, skipped
}

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
// without a since timestamp. After 18:00 the cutoff stays at 18:00 even
// for a late run; later reports end at midnight. Before 18:00 only flags of
// earlier days have ended.
func statusFlagArchiveWindow(now time.Time) (time.Time, calendar.Date) {
	today := calendar.DateFromTime(now)
	if !now.Before(statusFlagEndOfDay(now)) {
		return statusFlagEndOfDay(now), today
	}
	return today.BerlinMidnight(), today.AddDays(-1)
}

// scheduleStatusFlagClearTask schedules the task that keeps the sick /
// excused flags and their status days in step for every school, according
// to operations.sick_clear_mode and operations.excused_clear_mode.
func (s *Scheduler) scheduleStatusFlagClearTask() {
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
// per school and boundary. The pass counts as done only when every intended
// school committed its run; a failure before the callback, a failed commit
// or a pass with no schools leaves the boundary due for the next poll.
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
	var intended []int64
	completed := s.forEachTenantSettingsSnapshot(ctx, "status-flag-clear", func(tenantCtx context.Context, tenantID int64) error {
		state, err := s.readCleanupRunState(tenantCtx, &s.lastStatusFlagClear, statusFlagJobID, tenantID)
		if err != nil {
			failed.Store(true)
			return err
		}
		last, hasLast := state.LastSuccess, !state.LastSuccess.IsZero()
		if !statusFlagClearDue(now, last, hasLast) {
			return nil
		}
		run := statusFlagRun{tenantID: tenantID, now: now, last: last, hasLast: hasLast}
		sickMode := s.resolveStringSetting(tenantCtx, settingSickClearMode, clearModeEndOfDay)
		excusedMode := s.resolveStringSetting(tenantCtx, settingExcusedClearMode, clearModeEndOfDay)
		succeeded := s.rollStatusFlag(tenantCtx, run, sickMode, "sick", "sick_since")
		if !s.rollStatusFlag(tenantCtx, run, excusedMode, "excused", "excused_since") {
			succeeded = false
		}
		if !succeeded {
			failed.Store(true)
			return nil
		}
		// The success commits with the run's rows; the process cache follows
		// after commit.
		if err := s.recordCleanupSuccess(tenantCtx, &s.lastStatusFlagClear, statusFlagJobID, tenantID, now); err != nil {
			failed.Store(true)
			return err
		}
		return nil
	}, func(ctx context.Context) (*schedulerMinuteSnapshot, error) {
		snapshot, err := s.getMinuteSnapshot(ctx)
		if snapshot != nil {
			intended = snapshot.tenantIDs
		}
		return snapshot, err
	})
	if !failed.Load() && statusFlagPassComplete(intended, completed) {
		s.statusFlagPass.Store(statusFlagBoundary(now).Unix())
	}
}

// statusFlagPassComplete reports whether the pass had schools to process
// and every intended school is among the committed ones.
func statusFlagPassComplete(intended, completed []int64) bool {
	committed := make(map[int64]bool, len(completed))
	for _, id := range completed {
		committed[id] = true
	}
	for _, id := range intended {
		if !committed[id] {
			return false
		}
	}
	return len(intended) > 0
}

// statusFlagRun is one school's run at now after its last success.
type statusFlagRun struct {
	tenantID int64
	now      time.Time
	last     time.Time
	hasLast  bool
}

// rollStatusFlag applies one flag's clear mode: end_of_day archives the flags
// whose status day has ended, each onto its own day however many days the
// run missed; every other mode carries the flags into each day since the last
// success (statusFlagCarryDays).
func (s *Scheduler) rollStatusFlag(ctx context.Context, run statusFlagRun, mode, flagColumn, sinceColumn string) bool {
	tenantID, now := run.tenantID, run.now
	if s.studentStatusDayRepo == nil {
		s.logStatusFlagFailure(tenantID, flagColumn, fmt.Errorf("scheduler student status day repo not configured"))
		return false
	}
	if mode != clearModeEndOfDay {
		return s.carryStatusFlag(ctx, run, flagColumn, sinceColumn)
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

// carryStatusFlag lets every flag stand for each day the run covers, in day
// order; Care Plan carries a flag into a day only when it was reported before
// that day, so a flag reported during the outage starts with its next day.
func (s *Scheduler) carryStatusFlag(ctx context.Context, run statusFlagRun, flagColumn, sinceColumn string) bool {
	days, skipped := statusFlagCarryDays(run.now, run.last, run.hasLast)
	if skipped > 0 {
		s.getLogger().Warn(flagColumn+" status carry skipped days beyond the catch-up bound",
			slog.Int64("tenant_id", run.tenantID),
			slog.Int("days_skipped", skipped),
			slog.Time("last_success", run.last),
		)
	}
	var carried int64
	for _, day := range days {
		n, err := s.studentStatusDayRepo.CarryStatusFlag(ctx, flagColumn, sinceColumn, day, run.now)
		if err != nil {
			s.getLogger().Error(flagColumn+" status carry failed",
				slog.Int64("tenant_id", run.tenantID),
				slog.String("date", day.String()),
				slog.String("error", err.Error()),
			)
			return false
		}
		carried += n
	}
	if carried > 0 {
		s.getLogger().Info(flagColumn+" status carried into the day",
			slog.Int64("tenant_id", run.tenantID),
			slog.Int("days", len(days)),
			slog.Int64("rows_carried", carried),
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
