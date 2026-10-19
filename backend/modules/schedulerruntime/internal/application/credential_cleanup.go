package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// scheduleTokenCleanupTask schedules hourly token cleanup
func (s *Scheduler) scheduleTokenCleanupTask() {
	s.registerTask("token-cleanup", "1h", s.runTokenCleanupTask)
}

// runTokenCleanupTask runs the token cleanup task on schedule
func (s *Scheduler) runTokenCleanupTask(task *ScheduledTask) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in token cleanup task: %v", r)
			s.getLogger().Error("goroutine panic recovered",
				slog.String("job_id", task.Name),
				slog.String("error", err.Error()),
			)
			reportJobPanic(startJobRunReport(context.Background(), task.Name), r)
		}
	}()

	s.getLogger().Info("token cleanup task scheduled to run every hour")
	var acquired <-chan struct{}
	if s.leadership != nil {
		acquired = s.leadership.nextAcquisition()
	}

	// Run immediately on startup
	s.runJobCheck(task, s.executeTokenCleanup)

	// Then run every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runJobCheck(task, s.executeTokenCleanup)
		case <-acquired:
			acquired = s.leadership.nextAcquisition()
			s.runJobCheck(task, s.executeTokenCleanup)
		case <-s.done:
			return
		}
	}
}

// executeTokenCleanup executes the token cleanup task
func (s *Scheduler) executeTokenCleanup(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.LastRun = time.Now()
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.NextRun = time.Now().Add(time.Hour)
		task.mu.Unlock()
	}()

	if err := s.runCleanupJobs(ctx); err != nil {
		// runCleanupJobs left a breadcrumb for every failed cleanup job.
		addJobCommandFailure(ctx, err)
	}
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	ctx, err := s.startWorkerJob(s.lifecycleContext(), "token-cleanup")
	if err != nil {
		return fmt.Errorf("%w: %v", errWorkerTraceStart, err)
	}
	return s.runCleanupJobs(ctx)
}

func (s *Scheduler) runCleanupJobs(ctx context.Context) error {
	started := time.Now()
	logger := s.workerLogger(ctx)
	logger.InfoContext(ctx, "running scheduled token cleanup")
	if len(s.cleanupJobs) == 0 {
		logger.InfoContext(ctx, "no cleanup jobs registered, skipping token cleanup")
		return nil
	}

	if s.tenantRuntime == nil {
		if !s.traceWorkerFailure(ctx, "token-cleanup", "missing_tenant", errTenantRuntimeRequired) {
			s.observeTenantRuntime("missing_tenant")
			logger.ErrorContext(ctx, "runtime operation failed",
				slog.String("entry_point", "worker"),
				slog.String("operation", "token-cleanup"),
				slog.String("outcome", "missing_tenant"),
			)
		}
		return errTenantRuntimeRequired
	}
	ctx = s.withUnitOfWork(ctx)
	var firstErr error

	for _, job := range s.cleanupJobs {
		if job.Run == nil {
			continue
		}

		if err := s.runAdministrativeCleanup(ctx, job, logger); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		logger.InfoContext(ctx, "token cleanup completed", slog.Duration("duration", time.Since(started).Round(time.Millisecond)))
	}

	return firstErr
}

// buildCleanupJobs constructs the set of cleanup jobs so other runners can reuse the same registry.
// operatorRetentionJobs lists the hourly operator retention jobs in their
// required order. A missing side leaves its job out.
func operatorRetentionJobs(r ports.OperatorRetention) []CleanupJob {
	var jobs []CleanupJob
	if r.Ledger != nil {
		jobs = append(jobs, CleanupJob{Description: "Operator audit log retention", Run: r.Ledger.DeleteExpiredOperatorAuditLog})
	}
	if r.Accounts != nil {
		jobs = append(jobs, CleanupJob{Description: "Deactivated operator cleanup", Run: r.Accounts.DeleteDeactivatedOperators})
	}
	return jobs
}

func accountRetentionJobs(retention ports.AccountRetention) []CleanupJob {
	if isNilDependency(retention) {
		return nil
	}
	return []CleanupJob{{Description: "Unused account erasure", Run: retention.EraseUnusedAccounts}}
}

func buildCleanupJobs(authService ports.AuthCleanup, invitationService ports.InvitationCleaner, emailChangeCleaner ports.EmailChangeTokenCleaner, operatorInvitationCleaner ports.OperatorInvitationCleaner) []CleanupJob {
	var jobs []CleanupJob

	if authService != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Auth token cleanup",
			Run:         authService.CleanupExpiredTokens,
		})
	}

	if authService != nil {
		jobs = append(jobs,
			CleanupJob{
				Description: "Password reset token cleanup",
				Run:         authService.DeleteSpentPasswordResetTokens,
			},
			CleanupJob{
				Description: "Password reset rate limit cleanup",
				Run:         authService.DeleteStalePasswordResetWindows,
			},
		)
	}

	if invitationService != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Invitation cleanup",
			Run:         invitationService.DeleteExpiredSchoolInvitations,
		})
	}

	if emailChangeCleaner != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Email change token cleanup",
			Run:         emailChangeCleaner.CleanupOperatorEmailChanges,
		})
	}

	if operatorInvitationCleaner != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Operator invitation token cleanup",
			Run:         operatorInvitationCleaner.DeleteExpiredOperatorInvitations,
		})
	}
	if operatorInvitationCleaner != nil {
		jobs = append(jobs, signInDataCleanupJobs(operatorInvitationCleaner)...)
	}

	return jobs
}

// signInDataCleanupJobs deletes the short-lived sign-in data after its fixed
// window. Operator sessions go at their expiry, as account sessions do.
func signInDataCleanupJobs(cleaner ports.SignInDataCleaner) []CleanupJob {
	return []CleanupJob{
		{Description: "Guardian invitation cleanup", Run: cleaner.DeleteExpiredGuardianInvitations},
		{Description: "MFA email code cleanup", Run: cleaner.DeleteExpiredMFAEmailChallenges},
		{Description: "Passkey session cleanup", Run: cleaner.DeleteExpiredPasskeySessions},
		{Description: "Operator session cleanup", Run: func(ctx context.Context) (int, error) {
			return cleaner.DeleteExpiredOperatorSessions(ctx, time.Now())
		}},
	}
}

func (s *Scheduler) runAdministrativeCleanup(ctx context.Context, job CleanupJob, logger *slog.Logger) error {
	var count int
	err := s.tenantRuntime.WithinAdmin(ctx, func(txCtx context.Context) error {
		var runErr error
		count, runErr = job.Run(txCtx)
		return runErr
	})
	if err != nil {
		recordStandingJobFailure(ctx, 0, "cleanup job failed", err, map[string]any{"cleanup_job": job.Description})
		if !s.traceWorkerFailure(ctx, job.Description, "transaction_failure", err) {
			logger.ErrorContext(ctx, "cleanup job failed", slog.String("job", job.Description))
		}
		return err
	}

	logger.InfoContext(ctx, "cleanup job completed",
		slog.String("job", job.Description),
		slog.Int("records_deleted", count))
	return nil
}
