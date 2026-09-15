package scheduler

import (
	"context"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// scheduleOutboxWorkerTask registers the platform email outbox tick.
// A nil worker means the typed Worker root is incomplete and cannot start.
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
// so a slow tick can't overlap. MaxAttempts is passed per run from the
// `enrollment.outbox_max_attempts` setting on each tick — admins can
// tune retry budget without restart.
func (s *Scheduler) runOutboxOnce(ctx context.Context, task *ScheduledTask) {
	started := time.Now()
	if !s.tenantRuntimeConfigured {
		recordJobCommandFailure(ctx, tenant.ErrRuntimeRequired)
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

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()
	ctx = s.withUnitOfWork(ctx)

	const batchSize = 25
	processed, err := s.outboxWorker.RunOnce(ctx, batchSize, maxAttempts)
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.traceWorkerFailure(ctx, "email-outbox", "run_failure", err)
		s.getLogger().Error("outbox worker tick failed",
			slog.String("job_id", "email-outbox"),
			slog.String("error", err.Error()),
		)
		return
	}
	s.recordOutboxResult(ctx, processed, started)
}

func (s *Scheduler) recordOutboxResult(ctx context.Context, processed int, started time.Time) {
	backlog, err := s.outboxWorker.Backlog(ctx)
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.traceWorkerFailure(ctx, "email-outbox", "backlog_failure", err)
		s.getLogger().Error("outbox worker backlog query failed",
			slog.String("job_id", "email-outbox"),
			slog.String("error", err.Error()),
		)
		return
	}
	s.getLogger().Info("outbox worker tick complete",
		slog.String("job_id", "email-outbox"),
		slog.Int("processed", processed),
		slog.Int("backlog", backlog),
		slog.Duration("duration", time.Since(started)))
}
