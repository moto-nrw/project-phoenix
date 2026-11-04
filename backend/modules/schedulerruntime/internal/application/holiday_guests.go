package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	holidayGuestReconciliationStartupDelay = 45 * time.Second
	holidayGuestReconciliationInterval     = time.Hour
	holidayGuestReconciliationTimeout      = 15 * time.Minute
)

// scheduleHolidayGuestReconciliationTask reconciles the holiday guests of
// every school once an hour (#4666): a booking the Träger confirmed after
// the school picked the group joins it within the hour, a cancelled one
// leaves it.
func (s *Scheduler) scheduleHolidayGuestReconciliationTask() {
	if s.holidayGuests == nil {
		s.getLogger().Info("holiday guest reconciliation not configured")
		return
	}

	s.registerTask("holiday-guest-reconciliation", "1h-poll", s.runHolidayGuestReconciliationTask)
}

func (s *Scheduler) runHolidayGuestReconciliationTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in holiday guest reconciliation task",
		"holiday guest reconciliation using interval polling",
		holidayGuestReconciliationStartupDelay,
		func() time.Duration { return holidayGuestReconciliationInterval },
		s.checkAndRunHolidayGuestReconciliation,
	)
}

func (s *Scheduler) checkAndRunHolidayGuestReconciliation(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, holidayGuestReconciliationTimeout)
	defer cancel()

	if err := s.forEachTenant(ctx, "holiday-guest-reconciliation", s.holidayGuests); err != nil {
		s.getLogger().Error("holiday guest reconciliation failed",
			slog.String("error", err.Error()),
		)
	}
}
