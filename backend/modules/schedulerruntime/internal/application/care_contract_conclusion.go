package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	// careContractConclusionInterval bounds how long an accepted child of a
	// school with Weg b waits for its contract (#4680): the run after the
	// acceptance concludes it.
	careContractConclusionInterval     = 5 * time.Minute
	careContractConclusionStartupDelay = 70 * time.Second
	careContractConclusionTimeout      = 4 * time.Minute
)

// scheduleCareContractConclusionTask registers the conclusion at the
// acceptance. Like the school vacations it runs once across all Träger,
// not per tenant: it opens a school's tenant transaction and the Träger
// transaction one after the other, which no tenant batch may hold.
func (s *Scheduler) scheduleCareContractConclusionTask() {
	s.registerTask("care-contract-conclusion", "5m-poll", s.runCareContractConclusionTask)
}

func (s *Scheduler) runCareContractConclusionTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in care-contract-conclusion task",
		"care contract conclusion using interval polling",
		careContractConclusionStartupDelay, func() time.Duration { return careContractConclusionInterval },
		s.checkAndRunCareContractConclusion)
}

// checkAndRunCareContractConclusion concludes the waiting contracts. A
// failure only logs: the children stay candidates and the next tick tries
// again.
func (s *Scheduler) checkAndRunCareContractConclusion(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, careContractConclusionTimeout)
	defer cancel()

	concluded, err := s.careContractConclusion(s.withUnitOfWork(ctx))
	if err != nil {
		recordJobCommandFailure(ctx, err)
		s.getLogger().Error("care-contract-conclusion: run failed",
			slog.Int("concluded", concluded),
			slog.String("error", err.Error()),
		)
		return
	}
	if concluded > 0 {
		s.getLogger().Info("care-contract-conclusion: contracts concluded",
			slog.Int("concluded", concluded),
		)
	}
}
