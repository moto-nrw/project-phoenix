package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	weekProtocolsStartupDelay = 55 * time.Second
	// weekProtocolsInterval lets the evening run of the weekly protocols
	// (#4769) catch up within a quarter of an hour once the evening hour has
	// come, or after a restart; a recorded day stays recorded.
	weekProtocolsInterval = 15 * time.Minute
	weekProtocolsTimeout  = 10 * time.Minute
)

// scheduleWeekProtocolsTask runs the weekly protocols' evening of every
// school.
func (s *Scheduler) scheduleWeekProtocolsTask() {
	if s.weekProtocols == nil {
		s.getLogger().Info("week protocols not configured")
		return
	}

	s.registerTask("week-protocols", "15m-poll", s.runWeekProtocolsTask)
}

func (s *Scheduler) runWeekProtocolsTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in week protocols task",
		"week protocols using interval polling",
		weekProtocolsStartupDelay,
		func() time.Duration { return weekProtocolsInterval },
		s.checkAndRunWeekProtocols,
	)
}

func (s *Scheduler) checkAndRunWeekProtocols(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, weekProtocolsTimeout)
	defer cancel()

	now := time.Now()
	if err := s.forEachTenant(ctx, "week-protocols", func(tenantCtx context.Context) error {
		return s.weekProtocols(tenantCtx, now)
	}); err != nil {
		s.getLogger().Error("week protocols failed",
			slog.String("error", err.Error()),
		)
	}
}
