package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	emailReconciliationStartupDelay = 2 * time.Minute
	emailReconciliationInterval     = 10 * time.Minute
	emailReconciliationTimeout      = 8 * time.Minute
)

// scheduleEmailReconciliationTask registers the comparison of open mails with
// the mail provider's record (#5933). Lettermint gives up on a webhook after
// about 14 hours, so what happened during an outage arrives only this way.
func (s *Scheduler) scheduleEmailReconciliationTask() {
	s.registerTask("email-delivery-reconciliation", "interval-poll", s.runEmailReconciliationTaskPolling)
}

func (s *Scheduler) runEmailReconciliationTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in email-delivery-reconciliation task",
		"email-delivery-reconciliation task started",
		emailReconciliationStartupDelay, func() time.Duration { return emailReconciliationInterval },
		s.checkAndRunEmailReconciliation,
		slog.String("interval", emailReconciliationInterval.String()))
}

// checkAndRunEmailReconciliation reports the run's alarm as a job failure: the
// log line drives the Grafana alert, the failure reaches Sentry.
func (s *Scheduler) checkAndRunEmailReconciliation(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, emailReconciliationTimeout)
	defer cancel()

	if err := s.emailReconciliation.ReconcileEmailDelivery(s.withUnitOfWork(ctx)); err != nil {
		recordJobCommandFailure(ctx, err)
		s.traceWorkerFailure(ctx, "email-delivery-reconciliation", "alarm", err)
		s.getLogger().Error("email-delivery-reconciliation: alarm",
			slog.String("error", err.Error()),
		)
	}
}
