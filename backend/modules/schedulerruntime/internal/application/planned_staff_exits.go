package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	plannedStaffExitsStartupDelay = 50 * time.Second
	// plannedStaffExitsInterval bounds how long a member may still sign in
	// after the end of their last working day (#4809). Blocked exits wait
	// for their own hourly retry inside the run.
	plannedStaffExitsInterval = 5 * time.Minute
	plannedStaffExitsTimeout  = 4 * time.Minute
)

// schedulePlannedStaffExitsTask runs the planned staff exits of every school
// (#4809): the reminders before and on the last working day and the
// offboarding after its end.
func (s *Scheduler) schedulePlannedStaffExitsTask() {
	if s.plannedStaffExits == nil {
		s.getLogger().Info("planned staff exits not configured")
		return
	}

	s.registerTask("planned-staff-exits", "5m-poll", s.runPlannedStaffExitsTask)
}

func (s *Scheduler) runPlannedStaffExitsTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in planned staff exits task",
		"planned staff exits using interval polling",
		plannedStaffExitsStartupDelay,
		func() time.Duration { return plannedStaffExitsInterval },
		s.checkAndRunPlannedStaffExits,
	)
}

func (s *Scheduler) checkAndRunPlannedStaffExits(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, plannedStaffExitsTimeout)
	defer cancel()

	if err := s.forEachTenant(ctx, "planned-staff-exits", s.plannedStaffExits); err != nil {
		s.getLogger().Error("planned staff exits failed",
			slog.String("error", err.Error()),
		)
	}
}
