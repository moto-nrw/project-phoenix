package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	holidayClosureNotificationsStartupDelay = 50 * time.Second
	// holidayClosureNotificationsInterval: a click processes its own notice
	// at once; the job takes up what a broken request or a failed attempt
	// left and follows the mails. A minute keeps a stuck command visible
	// soon and costs one cheap query per run when nothing waits.
	holidayClosureNotificationsInterval = time.Minute
	holidayClosureNotificationsTimeout  = 5 * time.Minute
)

// scheduleHolidayClosureNotificationsTask processes the confirmed notices
// of changed school closing days (ADR 0196). A command it cannot process
// turns blocked and is reported as an error, which reaches Sentry.
func (s *Scheduler) scheduleHolidayClosureNotificationsTask() {
	if s.holidayClosureNotifications.ProcessDue == nil {
		s.getLogger().Info("holiday closure notifications not configured")
		return
	}
	s.registerTask("holiday-closure-notifications", "1m-poll", s.runHolidayClosureNotificationsTask)
}

func (s *Scheduler) runHolidayClosureNotificationsTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in holiday closure notifications task",
		"holiday closure notifications using interval polling",
		holidayClosureNotificationsStartupDelay,
		func() time.Duration { return holidayClosureNotificationsInterval },
		s.checkAndRunHolidayClosureNotifications,
	)
}

func (s *Scheduler) checkAndRunHolidayClosureNotifications(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, holidayClosureNotificationsTimeout)
	defer cancel()

	if err := s.holidayClosureNotifications.ProcessDue(ctx); err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("holiday closure notifications failed",
			slog.String("error", err.Error()),
		)
	}
}

// The watchdog has its own task, context and timeout. A stuck processing task
// cannot prevent it from inspecting the queue. Grafana also watches this
// task's heartbeat, so a stopped worker is observable outside the process.
func (s *Scheduler) scheduleHolidayClosureWatchdogTask() {
	s.registerTask("holiday-closure-watchdog", "1m-poll", func(task *ScheduledTask) {
		s.runIntervalPolling(task, "panic in holiday closure watchdog task", "holiday closure watchdog using interval polling",
			holidayClosureNotificationsStartupDelay, func() time.Duration { return time.Minute }, s.checkHolidayClosureWatchdog)
	})
}

func (s *Scheduler) checkHolidayClosureWatchdog(ctx context.Context, _ *ScheduledTask) {
	ctx, cancel := s.taskContext(ctx, 30*time.Second)
	defer cancel()
	count, err := s.holidayClosureNotifications.Overdue(ctx, 15*time.Minute)
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("holiday closure watchdog failed", "error", err.Error())
		return
	}
	s.getLogger().Info("holiday closure watchdog checked", "overdue", count)
	if count > 0 {
		s.getLogger().Error("holiday closure commands overdue", "overdue", count)
	}
}
