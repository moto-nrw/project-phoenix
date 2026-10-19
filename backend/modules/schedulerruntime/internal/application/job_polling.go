package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

func (s *Scheduler) startWorkerJob(ctx context.Context, operation string) (context.Context, error) {
	if s.workerTracer.StartJob == nil {
		return ctx, nil
	}
	return s.workerTracer.StartJob(ctx, operation)
}

func (s *Scheduler) traceWorkerFailure(ctx context.Context, operation, outcome string, err error) bool {
	if s.workerTracer.Failure != nil {
		s.workerTracer.Failure(ctx, operation, outcome, err)
		return true
	}
	return false
}

func (s *Scheduler) workerLogger(ctx context.Context) *slog.Logger {
	if s.workerTracer.Logger != nil {
		return s.workerTracer.Logger(ctx)
	}
	return s.getLogger()
}

func (s *Scheduler) observeWorkerRun(jobID ports.JobID, outcome string, duration time.Duration) {
	if s.workerTracer.Run != nil {
		s.workerTracer.Run(jobID, outcome, duration)
	}
}

func (s *Scheduler) withUnitOfWork(ctx context.Context) context.Context {
	if s.tenantRuntime == nil {
		return ctx
	}
	batchEvidence, _ := ctx.Value(batchRuntimeEvidenceKey{}).(*batchRuntimeEvidence)
	if s.unitOfWorkObserver == nil && batchEvidence == nil {
		return s.tenantRuntime.Attach(ctx, nil)
	}
	return s.tenantRuntime.Attach(ctx, func(kind, result string, duration time.Duration, retries int) {
		if batchEvidence != nil {
			batchEvidence.observe(kind, duration, retries)
		}
		if s.unitOfWorkObserver != nil {
			s.unitOfWorkObserver("worker", kind, result, duration, retries)
		}
	})
}

func (s *Scheduler) observeTenantRuntime(outcome string) {
	if s.tenantRuntimeObserver != nil {
		s.tenantRuntimeObserver("worker", outcome)
	}
}

// forEachTenant executes fn for each active tenant inside its tenant runtime.
// Missing runtime wiring fails closed before fn can reach a repository.
// Active tenant IDs share the same minute snapshot as settings-aware jobs so
// concurrent polling goroutines do not repeat the platform.schools query.
func (s *Scheduler) forEachTenant(ctx context.Context, opName string, fn func(ctx context.Context) error) error {
	if s.tenantRuntime == nil || (s.minuteSnapshotLoader == nil && s.schoolRepo == nil) {
		err := fmt.Errorf("tenant runtime is not configured for %s: %w", opName, errTenantRuntimeRequired)
		recordJobCommandFailure(ctx, err)
		s.observeTenantRuntime("missing_tenant")
		return err
	}

	minuteSnapshot, err := s.getMinuteSnapshot(ctx)
	if err != nil && (minuteSnapshot == nil || len(minuteSnapshot.tenantIDs) == 0) {
		recordJobCommandFailure(ctx, err)
		s.observeTenantRuntime("transaction_failure")
		return fmt.Errorf("load active tenants for %s: %w", opName, err)
	}
	result := s.runTenantBatches(ctx, minuteSnapshot.tenantIDs, opName, adaptTenantCommand(func(txCtx context.Context, _ int64) error {
		return fn(txCtx)
	}))
	return result.Err
}

// forEachTenantIncludingInactive runs a maintenance operation for every
// non-deleted tenant. It is reserved for recovery work that must continue
// after a school has been deactivated.
func (s *Scheduler) forEachTenantIncludingInactive(ctx context.Context, opName string, fn func(ctx context.Context) error) error {
	if s.tenantRuntime == nil || (s.allTenantIDsLoader == nil && s.schoolRepo == nil) {
		err := fmt.Errorf("tenant runtime is not configured for %s: %w", opName, errTenantRuntimeRequired)
		recordJobCommandFailure(ctx, err)
		s.observeTenantRuntime("missing_tenant")
		return err
	}

	ctx = s.withUnitOfWork(ctx)
	var tenantIDs []int64
	if s.allTenantIDsLoader != nil {
		var err error
		tenantIDs, err = s.allTenantIDsLoader(ctx)
		if err != nil {
			recordJobCommandFailure(ctx, err)
			s.observeTenantRuntime("transaction_failure")
			return fmt.Errorf("load tenants for %s: %w", opName, err)
		}
	} else {
		if err := s.tenantRuntime.WithinAdmin(ctx, func(txCtx context.Context) error {
			var listErr error
			tenantIDs, listErr = s.schoolRepo.ListNonDeletedTenantIDs(txCtx)
			return listErr
		}); err != nil {
			recordJobCommandFailure(ctx, err)
			s.observeTenantRuntime("transaction_failure")
			return fmt.Errorf("load tenants for %s: %w", opName, err)
		}
	}
	result := s.runTenantBatches(ctx, tenantIDs, opName, adaptTenantCommand(func(txCtx context.Context, _ int64) error {
		return fn(txCtx)
	}))
	return result.Err
}

// forEachTenantSettings executes fn for each active tenant, passing tenant ID for settings resolution.
// Missing runtime wiring skips work rather than invoking fn as tenant zero.
// Production jobs share one cross-tenant settings snapshot per minute.
func (s *Scheduler) forEachTenantSettings(ctx context.Context, opName string, fn func(ctx context.Context, tenantID int64) error) []int64 {
	if s.tenantRuntime == nil || (s.minuteSnapshotLoader == nil && s.schoolRepo == nil) {
		recordJobCommandFailure(ctx, fmt.Errorf("%w for %s", errTenantRuntimeRequired, opName))
		s.observeTenantRuntime("missing_tenant")
		s.getLogger().Error("tenant runtime is not configured",
			slog.String("entry_point", "worker"),
			slog.String("operation", opName),
		)
		return nil
	}

	minuteSnapshot, err := s.getMinuteSnapshot(ctx)
	if err != nil {
		// Older unit-test fakes implement only the narrow per-key resolver.
		// Production SettingsService always implements the batch loader.
		if errors.Is(err, errSchedulerSettingsBatchUnsupported) && minuteSnapshot != nil {
			return s.forEachKnownTenant(ctx, minuteSnapshot.tenantIDs, opName, fn)
		}
		recordJobCommandFailure(ctx, err)
		s.observeTenantRuntime("transaction_failure")
		s.getLogger().Error("scheduler settings snapshot unavailable",
			slog.String("operation", opName),
			slog.String("error", err.Error()),
		)
		return nil
	}

	source, _ := s.settings.(ports.SettingsSnapshotSource)
	return s.forEachKnownTenant(ctx, minuteSnapshot.tenantIDs, opName, func(txCtx context.Context, tenantID int64) error {
		snapshot := minuteSnapshot.settings[tenantID]
		if snapshot == nil || source == nil {
			return fmt.Errorf("settings snapshot missing for tenant %d", tenantID)
		}
		boundCtx, err := source.BindSettingsSnapshot(txCtx, snapshot)
		if err != nil {
			return fmt.Errorf("bind settings snapshot for tenant %d: %w", tenantID, err)
		}
		return fn(boundCtx, tenantID)
	})
}

// runMinutePolling is the shared runner for tasks that check per-tenant
// settings once per minute. It checks immediately on startup so the current
// minute isn't missed after a restart, then aligns to the minute boundary so
// ticks land at HH:MM:00. runJobCheck contains job panics so the next tick
// still runs; panicName labels only panics outside a job run. startupMsg is
// passed verbatim so the per-task log output stays byte-identical (Loki
// dashboards match on it).
func (s *Scheduler) runMinutePolling(task *ScheduledTask, panicName, startupMsg string, check func(context.Context, *ScheduledTask)) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("%s: %v", panicName, r)
			s.getLogger().Error("goroutine panic recovered",
				slog.String("job_id", task.Name),
				slog.String("error", err.Error()),
			)
			reportJobPanic(startJobRunReport(context.Background(), task.Name), r)
		}
	}()

	s.getLogger().Info(startupMsg)
	var acquired <-chan struct{}
	if s.leadership != nil {
		acquired = s.leadership.nextAcquisition()
	}

	// Immediate check on startup so we don't miss the current minute after a restart.
	s.runJobCheck(task, check)

	// Align to the next minute boundary so ticks land at HH:MM:00.
	var aligned bool
	acquired, aligned = s.waitUntilNextMinute(acquired, func() { s.runJobCheck(task, check) })
	if !aligned {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runJobCheck(task, check)
		case <-acquired:
			acquired = s.leadership.nextAcquisition()
			s.runJobCheck(task, check)
		case <-s.done:
			return
		}
	}
}

// runIntervalPolling is the shared runner for tasks that tick at a fixed or
// settings-driven interval. The startup delay honors s.done so shutdown during
// boot stays responsive; interval() is re-resolved on every tick so admins can
// change the cadence without a restart. As in runMinutePolling, panicName
// labels only panics outside a job run. startupMsg and startupAttrs are
// passed verbatim so log output stays byte-identical.
func (s *Scheduler) runIntervalPolling(task *ScheduledTask, panicName, startupMsg string, startupDelay time.Duration, interval func() time.Duration, check func(context.Context, *ScheduledTask), startupAttrs ...any) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("%s: %v", panicName, r)
			s.getLogger().Error("goroutine panic recovered",
				slog.String("job_id", task.Name),
				slog.String("error", err.Error()),
			)
			reportJobPanic(startJobRunReport(context.Background(), task.Name), r)
		}
	}()

	s.getLogger().Info(startupMsg, startupAttrs...)
	var acquired <-chan struct{}
	if s.leadership != nil {
		acquired = s.leadership.nextAcquisition()
	}

	select {
	case <-time.After(startupDelay):
	case <-s.done:
		return
	}
	s.runJobCheck(task, check)

	for {
		select {
		case <-time.After(interval()):
			s.runJobCheck(task, check)
		case <-acquired:
			acquired = s.leadership.nextAcquisition()
			s.runJobCheck(task, check)
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) runJobCheck(task *ScheduledTask, check func(context.Context, *ScheduledTask)) {
	ctx, leads := s.jobContext()
	if !leads {
		return
	}
	if traced, err := s.startWorkerJob(ctx, task.Name); err == nil {
		ctx = traced
	} else {
		s.getLogger().Error("worker job trace setup failed",
			slog.String("job_id", task.Name),
			slog.String("error", err.Error()),
		)
	}
	// Sentry sees the run once it failed for good or panicked (#3640); its
	// failed attempts collect as breadcrumbs until then.
	ctx = startJobRunReport(ctx, task.Name)
	failures := &jobCommandFailures{}
	ctx = context.WithValue(ctx, jobCommandFailuresKey{}, failures)
	ctx = context.WithValue(ctx, workerJobIDKey{}, ports.JobID(task.Name))
	started := time.Now()
	defer func() {
		duration := time.Since(started)
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("worker job panic: %v", recovered)
			s.observeWorkerRun(ports.JobID(task.Name), "panic", duration)
			s.traceWorkerFailure(ctx, task.Name, "panic", err)
			s.workerLogger(ctx).ErrorContext(ctx, "worker job run failed",
				slog.String("job_id", task.Name),
				slog.Duration("duration", duration),
			)
			// Report and swallow the panic so only this run fails; re-panicking
			// would end the polling loop until the next restart (#3597).
			reportJobPanic(ctx, recovered)
			return
		}
		if commandErr := failures.result(); commandErr != nil {
			s.observeWorkerRun(ports.JobID(task.Name), "failed", duration)
			s.traceWorkerFailure(ctx, task.Name, "command_failure", commandErr)
			s.workerLogger(ctx).ErrorContext(ctx, "worker job command failed",
				slog.String("job_id", task.Name),
				slog.Duration("duration", duration),
				slog.String("error", commandErr.Error()),
			)
			if !stoppedByShutdown(ctx, commandErr) {
				reportJobRunFailure(ctx, commandErr)
			}
			return
		}
		s.observeWorkerRun(ports.JobID(task.Name), "completed", duration)
		s.workerLogger(ctx).DebugContext(ctx, "worker job run completed",
			slog.String("job_id", task.Name),
			slog.Duration("duration", duration),
		)
	}()
	check(ctx, task)
}

// stoppedByShutdown reports whether the run only failed because the
// scheduler stopped under it. Like a request the client canceled, that is no
// defect and no Sentry event.
func stoppedByShutdown(ctx context.Context, err error) bool {
	return ctx.Err() != nil && onlyCancellationErrors(err)
}

// errors.Is on errors.Join would hide a real failure beside a cancellation.
func onlyCancellationErrors(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		parts := joined.Unwrap()
		if len(parts) == 0 {
			return false
		}
		for _, part := range parts {
			if !onlyCancellationErrors(part) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return onlyCancellationErrors(wrapped)
	}
	return errors.Is(err, context.Canceled)
}
