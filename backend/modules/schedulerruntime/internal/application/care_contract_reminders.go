package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	careContractRemindersStartupDelay = 80 * time.Second
	// careContractRemindersInterval: a reminder is due after whole days, so
	// an hourly look reaches it on its day.
	careContractRemindersInterval = time.Hour
	careContractRemindersTimeout  = 10 * time.Minute
)

// scheduleCareContractRemindersTask reminds parents of care contracts they
// have not signed (#4679).
func (s *Scheduler) scheduleCareContractRemindersTask() {
	if s.careContractReminders == nil {
		s.getLogger().Info("care contract reminders not configured")
		return
	}
	s.registerTask("care-contract-reminders", "1h-poll", s.runCareContractRemindersTask)
}

func (s *Scheduler) runCareContractRemindersTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in care contract reminders task",
		"care contract reminders using interval polling",
		careContractRemindersStartupDelay,
		func() time.Duration { return careContractRemindersInterval },
		s.checkAndRunCareContractReminders,
	)
}

func (s *Scheduler) checkAndRunCareContractReminders(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, careContractRemindersTimeout)
	defer cancel()

	if err := s.careContractReminders(ctx); err != nil {
		s.getLogger().Error("care contract reminders failed",
			slog.String("error", err.Error()),
		)
	}
}
