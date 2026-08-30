package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// RolloverDeadlineRunner is the narrow contract the scheduler needs
// from the enrollment rollover service. Defined here so the scheduler
// doesn't import the full enrollment service package — mirrors the
// opt-in shape used by StudentLifecycleRepository.
type RolloverDeadlineRunner interface {
	RunDeadlineWorker(ctx context.Context, asOf time.Time) (any, error)
}

// rolloverDeadlineRunner adapts the real service's typed return value
// (*enrollment.DeadlineWorkerSummary) to a generic `any` so the
// scheduler doesn't pull the enrollment package's type into its
// public surface. The factory passes a small lambda when wiring.
type rolloverDeadlineRunner struct {
	run func(ctx context.Context, asOf time.Time) (any, error)
}

func (r *rolloverDeadlineRunner) RunDeadlineWorker(ctx context.Context, asOf time.Time) (any, error) {
	return r.run(ctx, asOf)
}

// NewRolloverDeadlineRunner adapts a typed (ctx, asOf) → (T, error)
// callable into the scheduler's narrow runner interface. Used by the
// factory to wire enrollment.RolloverService.RunDeadlineWorker without
// the scheduler depending on the enrollment package.
func NewRolloverDeadlineRunner(fn func(ctx context.Context, asOf time.Time) (any, error)) RolloverDeadlineRunner {
	return &rolloverDeadlineRunner{run: fn}
}

// SetRolloverDeadlineRunner wires the per-tick rollover resolver.
// Nil runner → task does not register (opt-in pattern).
func (s *Scheduler) SetRolloverDeadlineRunner(r RolloverDeadlineRunner) {
	s.rolloverDeadlineRunner = r
}

// scheduleRolloverDeadlineTask registers the per-tenant tick. Reuses
// the existing operations.session_cleanup_interval_minutes signal as
// a "background cadence" knob would be heavier than this needs —
// instead, the tick runs hourly and is itself idempotent.
func (s *Scheduler) scheduleRolloverDeadlineTask() {
	if s.rolloverDeadlineRunner == nil {
		s.getLogger().Info("rollover-deadline task not configured (no runner)")
		return
	}

	s.registerTask("rollover-deadline", "interval-poll", s.runRolloverDeadlineTaskPolling)
}

const rolloverDeadlineInterval = 1 * time.Hour

func (s *Scheduler) runRolloverDeadlineTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in rollover-deadline task",
		"rollover-deadline task started",
		30*time.Second, func() time.Duration { return rolloverDeadlineInterval },
		s.checkAndRunRolloverDeadline,
		slog.String("interval", rolloverDeadlineInterval.String()))
}

func (s *Scheduler) checkAndRunRolloverDeadline(task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(30 * time.Minute)
	defer cancel()

	now := time.Now()
	s.forEachTenantSettings(ctx, "rollover-deadline", func(tenantCtx context.Context, tenantID int64) error {
		_, err := s.rolloverDeadlineRunner.RunDeadlineWorker(tenantCtx, now)
		if err != nil {
			s.getLogger().Error("rollover-deadline: tenant tick failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
		// Returning the worker error is what makes the tenant runtime roll
		// back this tenant's transaction. Swallowing it here can commit writes
		// made before a fatal transaction/savepoint failure.
		return err
	})
}
