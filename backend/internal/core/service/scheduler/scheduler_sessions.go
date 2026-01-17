package scheduler

import (
	"context"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/logger"
)

// scheduleSessionEndTask schedules the daily session end task
func (s *Scheduler) scheduleSessionEndTask() {
	// Check if session end is enabled (default enabled)
	if !s.config.SessionEndEnabled {
		logger.Logger.Info("Session end scheduler is disabled (set SESSION_END_SCHEDULER_ENABLED=true to enable)")
		return
	}

	task := &ScheduledTask{
		Name:     "session-end",
		Schedule: s.config.SessionEndSchedule,
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runDailyTask(task, func() { s.executeSessionEnd(task) })
}

// executeSessionEnd executes the session end task
func (s *Scheduler) executeSessionEnd(task *ScheduledTask) {
	if !task.tryStart() {
		logger.Logger.Warn("Session end task already running, skipping")
		return
	}
	defer task.finish(24 * time.Hour)

	logger.Logger.Info("Starting scheduled session end")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.SessionEndTimeoutMinutes)*time.Minute)
	defer cancel()

	// Call the active service to end all daily sessions
	result, err := s.activeService.EndDailySessions(ctx)
	if err != nil {
		logger.Logger.WithError(err).Error("Scheduled session end failed")
		return
	}

	duration := time.Since(startTime)
	logger.Logger.WithFields(map[string]interface{}{
		"duration":          duration.Round(time.Second).String(),
		"sessions_ended":    result.SessionsEnded,
		"visits_ended":      result.VisitsEnded,
		"supervisors_ended": result.SupervisorsEnded,
		"success":           result.Success,
	}).Info("Scheduled session end completed")

	if len(result.Errors) > 0 {
		logger.Logger.WithField("error_count", len(result.Errors)).Warn("Session end completed with errors")
		for i, errMsg := range result.Errors {
			if i < 10 { // Log first 10 errors
				logger.Logger.WithFields(map[string]interface{}{
					"error_index": i + 1,
					"error":       errMsg,
				}).Warn("Session end error")
			}
		}
		if len(result.Errors) > 10 {
			logger.Logger.WithField("additional_errors", len(result.Errors)-10).Warn("Additional errors not shown")
		}
	}
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Check if session cleanup is enabled (default enabled)
	if !s.config.SessionCleanupEnabled {
		logger.Logger.Info("Session cleanup is disabled (set SESSION_CLEANUP_ENABLED=true to enable)")
		return
	}

	task := &ScheduledTask{
		Name:     "session-cleanup",
		Schedule: strconv.Itoa(s.config.SessionCleanupIntervalMinutes) + "m",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	// Capture configuration values before starting goroutine to prevent data race.
	// These values are passed as parameters to avoid unsynchronized reads of struct fields.
	intervalMinutes := s.config.SessionCleanupIntervalMinutes
	thresholdMinutes := s.config.SessionAbandonedThresholdMinutes

	s.wg.Add(1)
	go s.runSessionCleanupTask(task, intervalMinutes, thresholdMinutes)
}

// runSessionCleanupTask runs the session cleanup task at configured intervals.
// Configuration values are passed as parameters to avoid data races with struct fields.
func (s *Scheduler) runSessionCleanupTask(task *ScheduledTask, intervalMinutes, thresholdMinutes int) {
	defer s.wg.Done()

	interval := time.Duration(intervalMinutes) * time.Minute
	logger.Logger.WithField("interval_minutes", intervalMinutes).Info("Session cleanup task scheduled")

	// Run immediately on startup (after brief delay to let other services initialize)
	select {
	case <-time.After(30 * time.Second):
		s.executeSessionCleanup(task, intervalMinutes, thresholdMinutes)
	case <-s.done:
		return
	}

	// Then run at configured interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeSessionCleanup(task, intervalMinutes, thresholdMinutes)
		case <-s.done:
			return
		}
	}
}

// executeSessionCleanup executes the session cleanup task.
// Configuration values are passed as parameters to avoid data races with struct fields.
func (s *Scheduler) executeSessionCleanup(task *ScheduledTask, intervalMinutes, thresholdMinutes int) {
	if !task.tryStart() {
		return
	}
	defer task.finish(time.Duration(intervalMinutes) * time.Minute)

	// Add timeout to prevent cleanup from blocking shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	threshold := time.Duration(thresholdMinutes) * time.Minute

	// Call the active service cleanup method
	count, err := s.activeService.CleanupAbandonedSessions(ctx, threshold)
	if err != nil {
		logger.Logger.WithError(err).Error("Session cleanup failed")
		return
	}

	if count > 0 {
		logger.Logger.WithFields(map[string]interface{}{
			"sessions_cleaned":  count,
			"threshold_minutes": thresholdMinutes,
		}).Info("Abandoned sessions cleaned up")
	}
}
