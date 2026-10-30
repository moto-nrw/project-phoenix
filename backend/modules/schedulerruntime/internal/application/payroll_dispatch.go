package application

import (
	"context"
	"log/slog"
	"time"
)

const (
	payrollDispatchStartupDelay = 40 * time.Second
	// payrollDispatchInterval is how soon after a month close the file
	// leaves, and how often a mailed link's result is read.
	payrollDispatchInterval = time.Minute
	payrollDispatchTimeout  = 5 * time.Minute
)

// schedulePayrollDispatchTask sends the month files of every school to its
// payroll office after the Monatsabschluss (#4805). It also runs for schools
// that no longer book the time tracking or are inactive: the purge of
// expired payroll files is the table's retention.
func (s *Scheduler) schedulePayrollDispatchTask() {
	if s.payrollDispatch == nil {
		s.getLogger().Info("payroll dispatch not configured")
		return
	}
	s.registerTask("payroll-dispatch", "1m-poll", s.runPayrollDispatchTask)
}

func (s *Scheduler) runPayrollDispatchTask(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in payroll dispatch task",
		"payroll dispatch using interval polling",
		payrollDispatchStartupDelay,
		func() time.Duration { return payrollDispatchInterval },
		s.checkAndRunPayrollDispatch,
	)
}

func (s *Scheduler) checkAndRunPayrollDispatch(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, payrollDispatchTimeout)
	defer cancel()
	if err := s.forEachTenantIncludingInactive(ctx, "payroll-dispatch", s.runPayrollDispatchForTenant); err != nil {
		s.getLogger().Error("payroll dispatch failed",
			slog.String("error", err.Error()),
		)
	}
}

// runPayrollDispatchForTenant never fails the tenant loop: every step of a
// dispatch commits on its own, and a failed step is already recorded as a
// failed attempt that the next pass retries.
func (s *Scheduler) runPayrollDispatchForTenant(tenantCtx context.Context) error {
	finished, err := s.payrollDispatch(tenantCtx)
	if finished > 0 {
		s.getLogger().Info("payroll dispatches finished",
			slog.Int("finished", finished),
		)
	}
	if err != nil {
		s.getLogger().Error("payroll dispatch incomplete",
			slog.String("error", err.Error()),
		)
	}
	return nil
}
