package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	mealOrdersStartupDelay = 45 * time.Second
	// mealOrdersInterval bounds how long after its deadline an order is
	// fixed and how long a late change waits to be recorded (#3873).
	mealOrdersInterval = 5 * time.Minute
	mealOrdersTimeout  = 4 * time.Minute
)

func (s *Scheduler) scheduleMealOrdersTask() {
	s.registerTask("meal-orders", "5m-poll", s.runMealOrdersTask)
}

func (s *Scheduler) runMealOrdersTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in meal orders task",
		"meal orders using interval polling",
		mealOrdersStartupDelay,
		func() time.Duration { return mealOrdersInterval },
		s.checkAndRunMealOrders,
	)
}

// checkAndRunMealOrders settles the meal orders of every school. A school
// without meal registration settles nothing.
func (s *Scheduler) checkAndRunMealOrders(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, mealOrdersTimeout)
	defer cancel()

	if err := s.forEachTenant(ctx, "meal-orders", s.mealOrders); err != nil {
		s.getLogger().Error("meal orders failed",
			slog.String("error", err.Error()),
		)
	}
}
