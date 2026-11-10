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
	if s.holidayClosureNotifications == nil {
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

	if err := s.holidayClosureNotifications(ctx); err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("holiday closure notifications failed",
			slog.String("error", err.Error()),
		)
	}
}
