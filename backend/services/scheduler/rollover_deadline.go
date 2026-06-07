package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
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

	task := &ScheduledTask{
		Name:     "rollover-deadline",
		Schedule: "interval-poll",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runRolloverDeadlineTaskPolling(task)
}

const rolloverDeadlineInterval = 1 * time.Hour

func (s *Scheduler) runRolloverDeadlineTaskPolling(task *ScheduledTask) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in rollover-deadline task: %v", r)
			s.getLogger().Error("goroutine panic recovered", slog.String("error", err.Error()))
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
		}
	}()

	s.getLogger().Info("rollover-deadline task started",
		slog.String("interval", rolloverDeadlineInterval.String()),
	)

	// Brief startup delay so the rest of the services finish booting
	// before the first tick. Honor s.done so shutdown stays responsive.
	select {
	case <-time.After(30 * time.Second):
	case <-s.done:
		return
	}
	s.checkAndRunRolloverDeadline(task)

	for {
		select {
		case <-time.After(rolloverDeadlineInterval):
			s.checkAndRunRolloverDeadline(task)
		case <-s.done:
			return
		}
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
		return nil
	})
}
