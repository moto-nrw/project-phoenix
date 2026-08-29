package scheduler

import (
	"context"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// scheduleOutboxWorkerTask registers the platform email outbox tick.
// Nil worker → no task registers (matches the SetMaterializer pattern).
// The interval is settings-driven (enrollment.outbox_worker_interval_seconds,
// default 30 — see services/config/defaults/enrollment.go). Re-resolves
// on each tick so admins can shorten the cadence without restart.
func (s *Scheduler) scheduleOutboxWorkerTask() {
	if s.outboxWorker == nil {
		s.getLogger().Info("outbox worker not configured (no OutboxWorker)")
		return
	}

	s.registerTask("email-outbox", "interval-poll", s.runOutboxWorkerTaskPolling)
}

func (s *Scheduler) runOutboxWorkerTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in outbox worker task",
		"outbox worker using interval polling",
		15*time.Second, s.resolveOutboxInterval, s.runOutboxOnce)
}

// resolveOutboxInterval reads the polling interval setting. Falls back
// to 30s when no override exists. Same fallback chain pattern as the
// activate-students interval.
func (s *Scheduler) resolveOutboxInterval() time.Duration {
	seconds := s.resolveIntSetting(context.Background(), configModel.KeyEnrollmentOutboxWorkerIntervalSeconds, "", 30)
	if seconds < 1 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

// runOutboxOnce calls the worker. Re-entry is guarded by task.Running
// so a slow tick can't overlap. MaxAttempts is pushed from the
// `enrollment.outbox_max_attempts` setting on each tick — admins can
// tune retry budget without restart.
func (s *Scheduler) runOutboxOnce(task *ScheduledTask) {
	if !s.tenantRuntimeConfigured {
		s.observeTenantRuntime("missing_tenant")
		s.getLogger().Error("outbox worker runtime is not configured")
		return
	}
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

	maxAttempts := s.resolveIntSetting(context.Background(), configModel.KeyEnrollmentOutboxMaxAttempts, "", 6)
	s.outboxWorker.SetMaxAttempts(maxAttempts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ctx = s.withUnitOfWork(ctx)

	const batchSize = 25
	processed, err := s.outboxWorker.RunOnce(ctx, batchSize)
	if err != nil {
		s.getLogger().Error("outbox worker tick failed",
			slog.String("error", err.Error()))
		return
	}
	if processed > 0 {
		s.getLogger().Info("outbox worker tick complete",
			slog.Int("processed", processed))
	}
}
