package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	// schoolVacationsInterval is how often the worker renews the school
	// vacations (#5345). They change a few times a year at most.
	schoolVacationsInterval     = 24 * time.Hour
	schoolVacationsStartupDelay = 3 * time.Minute
	// schoolVacationsTimeout covers one request with its 10-second limit and
	// the replacement of every state's rows.
	schoolVacationsTimeout = 2 * time.Minute
)

// scheduleSchoolVacationsTask registers the school vacation sync. Like the
// analytics school names it runs once across all schools, not per tenant.
func (s *Scheduler) scheduleSchoolVacationsTask() {
	s.registerTask("school-vacation-sync", "interval-poll", s.runSchoolVacationsTaskPolling)
}

func (s *Scheduler) runSchoolVacationsTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in school-vacation-sync task",
		"school-vacation-sync task started",
		schoolVacationsStartupDelay, func() time.Duration { return schoolVacationsInterval },
		s.checkAndRunSchoolVacations,
		slog.String("interval", schoolVacationsInterval.String()))
}

// checkAndRunSchoolVacations renews the vacations. A failure only logs: the
// stored vacations stay, and the next tick tries again.
func (s *Scheduler) checkAndRunSchoolVacations(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, schoolVacationsTimeout)
	defer cancel()

	regions, vacations, err := s.schoolVacations(s.withUnitOfWork(ctx))
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("school-vacation-sync: sync failed, stored vacations kept",
			slog.String("error", err.Error()),
		)
		return
	}
	s.getLogger().Info("school-vacation-sync: vacations stored",
		slog.Int("regions", regions),
		slog.Int("vacations", vacations),
	)
}
