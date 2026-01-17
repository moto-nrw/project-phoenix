package scheduler

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/logger"
)

// AuthCleanup exposes the cleanup routines required from the auth service.
type AuthCleanup interface {
	CleanupExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
	CleanupExpiredRateLimits(ctx context.Context) (int, error)
}

// InvitationCleaner exposes the cleanup routine required from the invitation service.
type InvitationCleaner interface {
	CleanupExpiredInvitations(ctx context.Context) (int, error)
}

// CleanupJob represents a single cleanup task that can be executed.
type CleanupJob struct {
	Description string
	Run         func(context.Context) (int, error)
}

// scheduleCleanupTask schedules the daily cleanup task
func (s *Scheduler) scheduleCleanupTask() {
	// Check if cleanup is enabled
	if !s.config.CleanupEnabled {
		logger.Logger.Info("Cleanup scheduler is disabled (set CLEANUP_SCHEDULER_ENABLED=true to enable)")
		return
	}

	task := &ScheduledTask{
		Name:     "visit-cleanup",
		Schedule: s.config.CleanupSchedule,
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runDailyTask(task, func() { s.executeCleanup(task) })
}

// executeCleanup executes the cleanup task
func (s *Scheduler) executeCleanup(task *ScheduledTask) {
	if !task.tryStart() {
		logger.Logger.Warn("Cleanup task already running, skipping")
		return
	}
	defer task.finish(24 * time.Hour)

	logger.Logger.Info("Starting scheduled visit cleanup")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.CleanupTimeoutMinutes)*time.Minute)
	defer cancel()

	result, err := s.cleanupService.CleanupExpiredVisits(ctx)
	if err != nil {
		logger.Logger.WithError(err).Error("Scheduled cleanup failed")
		return
	}

	duration := time.Since(startTime)
	logger.Logger.WithFields(map[string]interface{}{
		"duration":           duration.Round(time.Second).String(),
		"students_processed": result.StudentsProcessed,
		"records_deleted":    result.RecordsDeleted,
		"success":            result.Success,
	}).Info("Scheduled cleanup completed")

	if len(result.Errors) > 0 {
		logger.Logger.WithField("error_count", len(result.Errors)).Warn("Cleanup completed with errors")
		for i, err := range result.Errors {
			if i < 10 { // Log first 10 errors
				logger.Logger.WithFields(map[string]interface{}{
					"student_id": err.StudentID,
					"error":      err.Error,
				}).Warn("Cleanup error")
			}
		}
		if len(result.Errors) > 10 {
			logger.Logger.WithField("additional_errors", len(result.Errors)-10).Warn("Additional errors not shown")
		}
	}
}

// scheduleTokenCleanupTask schedules hourly token cleanup
func (s *Scheduler) scheduleTokenCleanupTask() {
	task := &ScheduledTask{
		Name:     "token-cleanup",
		Schedule: "1h", // Run every hour
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runTokenCleanupTask(task)
}

// runTokenCleanupTask runs the token cleanup task on schedule
func (s *Scheduler) runTokenCleanupTask(task *ScheduledTask) {
	defer s.wg.Done()

	logger.Logger.Info("Token cleanup task scheduled to run every hour")

	// Run immediately on startup
	s.executeTokenCleanup(task)

	// Then run every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeTokenCleanup(task)
		case <-s.done:
			return
		}
	}
}

// executeTokenCleanup executes the token cleanup task
func (s *Scheduler) executeTokenCleanup(task *ScheduledTask) {
	if !task.tryStart() {
		return
	}
	defer task.finish(time.Hour)

	logger.Logger.Info("Running scheduled token cleanup")
	startTime := time.Now()

	// Use reflection to call CleanupExpiredTokens method
	if err := s.RunCleanupJobs(); err != nil {
		logger.Logger.WithError(err).Error("Token cleanup failed")
		return
	}

	duration := time.Since(startTime)
	logger.Logger.WithField("duration", duration.Round(time.Millisecond).String()).Info("Token cleanup completed")
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	if len(s.cleanupJobs) == 0 {
		logger.Logger.Info("No cleanup jobs registered; skipping token cleanup")
		return nil
	}

	ctx := context.Background()
	var firstErr error

	for _, job := range s.cleanupJobs {
		if job.Run == nil {
			continue
		}

		count, err := job.Run(ctx)
		if err != nil {
			logger.Logger.WithField("job", job.Description).WithError(err).Error("Cleanup job failed")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		logger.Logger.WithFields(map[string]interface{}{
			"job":             job.Description,
			"records_removed": count,
		}).Info("Cleanup job completed")
	}

	return firstErr
}

// buildCleanupJobs constructs the set of cleanup jobs so other runners can reuse the same registry.
func buildCleanupJobs(authService AuthCleanup, invitationService InvitationCleaner) []CleanupJob {
	var jobs []CleanupJob

	if authService != nil {
		jobs = append(jobs,
			CleanupJob{
				Description: "Auth token cleanup",
				Run: func(ctx context.Context) (int, error) {
					return authService.CleanupExpiredTokens(ctx)
				},
			},
			CleanupJob{
				Description: "Password reset token cleanup",
				Run: func(ctx context.Context) (int, error) {
					return authService.CleanupExpiredPasswordResetTokens(ctx)
				},
			},
			CleanupJob{
				Description: "Password reset rate limit cleanup",
				Run: func(ctx context.Context) (int, error) {
					return authService.CleanupExpiredRateLimits(ctx)
				},
			},
		)
	}

	if invitationService != nil {
		jobs = append(jobs, CleanupJob{
			Description: "Invitation cleanup",
			Run: func(ctx context.Context) (int, error) {
				return invitationService.CleanupExpiredInvitations(ctx)
			},
		})
	}

	return jobs
}
