package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// billingKeyDatesInterval is how often the billing capture checks whether the
// month's key date is due (#2791). The check itself is one small read; the
// capture writes once per school and month, so a tick after the key date
// only finds nothing left to do.
const billingKeyDatesInterval = 15 * time.Minute

// scheduleBillingKeyDatesTask registers the billing key-date capture. It
// runs across schools in one administrative transaction, not per tenant: all
// schools of one capture share one snapshot of the counts.
func (s *Scheduler) scheduleBillingKeyDatesTask() {
	s.registerTask("billing-key-dates", "interval-poll", s.runBillingKeyDatesTaskPolling)
}

func (s *Scheduler) runBillingKeyDatesTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in billing-key-dates task",
		"billing-key-dates task started",
		45*time.Second, func() time.Duration { return billingKeyDatesInterval },
		s.checkAndRunBillingKeyDates,
		slog.String("interval", billingKeyDatesInterval.String()))
}

func (s *Scheduler) checkAndRunBillingKeyDates(ctx context.Context, task *ScheduledTask) {
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

	written, err := s.schoolRepo.RecordDueBillingKeyDates(s.withUnitOfWork(ctx), time.Now())
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("billing-key-dates: capture failed",
			slog.String("error", err.Error()),
		)
		return
	}
	if written > 0 {
		s.getLogger().Info("billing-key-dates: counts captured",
			slog.Int("schools", written),
		)
	}
}
