package application

import (
	"context"
	"log/slog"
	"time"
)

// schoolNamesInterval is how often the worker names the schools in the
// product analytics. A new or renamed school shows its name in the usage
// dashboards after the next tick; the first tick runs shortly after start.
const schoolNamesInterval = 24 * time.Hour

// scheduleSchoolNamesTask registers the analytics school names. Like the
// billing capture it runs once across all schools, not per tenant.
func (s *Scheduler) scheduleSchoolNamesTask() {
	s.registerTask("analytics-school-names", "interval-poll", s.runSchoolNamesTaskPolling)
}

func (s *Scheduler) runSchoolNamesTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in analytics-school-names task",
		"analytics-school-names task started",
		2*time.Minute, func() time.Duration { return schoolNamesInterval },
		s.checkAndRunSchoolNames,
		slog.String("interval", schoolNamesInterval.String()))
}

func (s *Scheduler) checkAndRunSchoolNames(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, time.Minute)
	defer cancel()

	named, err := s.schoolRepo.PublishSchoolNames(s.withUnitOfWork(ctx))
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("analytics-school-names: publishing failed",
			slog.String("error", err.Error()),
		)
		return
	}
	s.getLogger().Info("analytics-school-names: schools named",
		slog.Int("schools", named),
	)
}
