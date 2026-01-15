package scheduler

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/services/active"
)


// parseScheduledTime parses a HH:MM time string into hour and minute components.
// Returns an error if the format is invalid.
func parseScheduledTime(timeStr string) (hour, minute int, err error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}

	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in time: %s", timeStr)
	}

	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute in time: %s", timeStr)
	}

	return hour, minute, nil
}

// calculateNextRun calculates the next run time for a daily task at the given hour and minute.
// If the time has already passed today, it schedules for tomorrow.
func calculateNextRun(hour, minute int) time.Time {
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(nextRun) {
		nextRun = nextRun.Add(24 * time.Hour)
	}
	return nextRun
}

// runDailyTask is a generic runner for tasks that execute once per day at a scheduled time.
// It handles parsing the schedule, waiting for the first run, and running on a 24-hour cycle.
func (s *Scheduler) runDailyTask(task *ScheduledTask, execute func()) {
	defer s.wg.Done()

	hour, minute, err := parseScheduledTime(task.Schedule)
	if err != nil {
		logging.Logger.WithField("task", task.Name).WithError(err).Error("Invalid scheduled time")
		return
	}

	nextRun := calculateNextRun(hour, minute)
	initialWait := time.Until(nextRun)
	logging.Logger.WithFields(map[string]interface{}{
		"task":     task.Name,
		"wait":     initialWait.Round(time.Minute).String(),
		"next_run": nextRun.Format("2006-01-02 15:04:05"),
	}).Info("Task scheduled")

	select {
	case <-time.After(initialWait):
		execute()
	case <-s.done:
		return
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			execute()
		case <-s.done:
			return
		}
	}
}

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

// Scheduler manages scheduled tasks
type Scheduler struct {
	activeService     active.Service
	cleanupService    active.CleanupService
	authCleanup       AuthCleanup
	invitationCleanup InvitationCleaner
	cleanupJobs       []CleanupJob
	tasks             map[string]*ScheduledTask
	mu                sync.RWMutex
	// done signals goroutines to stop when closed (replaces stored context)
	done chan struct{}
	wg   sync.WaitGroup

	// Session cleanup configuration (parsed once during initialization)
	sessionCleanupIntervalMinutes    int
	sessionAbandonedThresholdMinutes int
}

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
	Name     string
	Schedule string // cron-like schedule or duration
	LastRun  time.Time
	NextRun  time.Time
	Running  bool
	mu       sync.Mutex
}

// tryStart attempts to acquire the task lock for execution.
// Returns true if the lock was acquired, false if task is already running.
// Caller MUST call task.finish() when done if tryStart returns true.
func (t *ScheduledTask) tryStart() bool {
	t.mu.Lock()
	if t.Running {
		t.mu.Unlock()
		return false
	}
	t.Running = true
	t.LastRun = time.Now()
	t.mu.Unlock()
	return true
}

// finish releases the task lock and sets the next run time.
func (t *ScheduledTask) finish(nextInterval time.Duration) {
	t.mu.Lock()
	t.Running = false
	t.NextRun = time.Now().Add(nextInterval)
	t.mu.Unlock()
}

// NewScheduler creates a new scheduler
func NewScheduler(activeService active.Service, cleanupService active.CleanupService, authService AuthCleanup, invitationService InvitationCleaner) *Scheduler {
	return &Scheduler{
		activeService:     activeService,
		cleanupService:    cleanupService,
		authCleanup:       authService,
		invitationCleanup: invitationService,
		cleanupJobs:       buildCleanupJobs(authService, invitationService),
		tasks:             make(map[string]*ScheduledTask),
		done:              make(chan struct{}),
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	logging.Logger.Info("Starting scheduler service")

	// Schedule daily data cleanup at 2 AM
	s.scheduleCleanupTask()

	// Schedule daily session end at configurable time (default 6 PM)
	s.scheduleSessionEndTask()

	// Schedule token cleanup every hour
	s.scheduleTokenCleanupTask()

	// Schedule abandoned session cleanup
	s.scheduleSessionCleanupTask()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	logging.Logger.Info("Stopping scheduler service")
	close(s.done)
	s.wg.Wait()
	logging.Logger.Info("Scheduler service stopped")
}

// scheduleCleanupTask schedules the daily cleanup task
func (s *Scheduler) scheduleCleanupTask() {
	// Check if cleanup is enabled
	if os.Getenv("CLEANUP_SCHEDULER_ENABLED") != "true" {
		logging.Logger.Info("Cleanup scheduler is disabled (set CLEANUP_SCHEDULER_ENABLED=true to enable)")
		return
	}

	// Get scheduled time from env or default to 2 AM
	scheduledTime := os.Getenv("CLEANUP_SCHEDULER_TIME")
	if scheduledTime == "" {
		scheduledTime = "02:00"
	}

	task := &ScheduledTask{
		Name:     "visit-cleanup",
		Schedule: scheduledTime,
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
		logging.Logger.Warn("Cleanup task already running, skipping")
		return
	}
	defer task.finish(24 * time.Hour)

	logging.Logger.Info("Starting scheduled visit cleanup")
	startTime := time.Now()

	// Get timeout from env or default to 30 minutes
	timeoutMinutes := 30
	if timeoutStr := os.Getenv("CLEANUP_SCHEDULER_TIMEOUT_MINUTES"); timeoutStr != "" {
		if parsed, err := strconv.Atoi(timeoutStr); err == nil && parsed > 0 {
			timeoutMinutes = parsed
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	result, err := s.cleanupService.CleanupExpiredVisits(ctx)
	if err != nil {
		logging.Logger.WithError(err).Error("Scheduled cleanup failed")
		return
	}

	duration := time.Since(startTime)
	logging.Logger.WithFields(map[string]interface{}{
		"duration":           duration.Round(time.Second).String(),
		"students_processed": result.StudentsProcessed,
		"records_deleted":    result.RecordsDeleted,
		"success":            result.Success,
	}).Info("Scheduled cleanup completed")

	if len(result.Errors) > 0 {
		logging.Logger.WithField("error_count", len(result.Errors)).Warn("Cleanup completed with errors")
		for i, err := range result.Errors {
			if i < 10 { // Log first 10 errors
				logging.Logger.WithFields(map[string]interface{}{
					"student_id": err.StudentID,
					"error":      err.Error,
				}).Warn("Cleanup error")
			}
		}
		if len(result.Errors) > 10 {
			logging.Logger.WithField("additional_errors", len(result.Errors)-10).Warn("Additional errors not shown")
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

	logging.Logger.Info("Token cleanup task scheduled to run every hour")

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

	logging.Logger.Info("Running scheduled token cleanup")
	startTime := time.Now()

	// Use reflection to call CleanupExpiredTokens method
	if err := s.RunCleanupJobs(); err != nil {
		logging.Logger.WithError(err).Error("Token cleanup failed")
		return
	}

	duration := time.Since(startTime)
	logging.Logger.WithField("duration", duration.Round(time.Millisecond).String()).Info("Token cleanup completed")
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	if len(s.cleanupJobs) == 0 {
		logging.Logger.Info("No cleanup jobs registered; skipping token cleanup")
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
			logging.Logger.WithField("job", job.Description).WithError(err).Error("Cleanup job failed")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		logging.Logger.WithFields(map[string]interface{}{
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

// scheduleSessionEndTask schedules the daily session end task
func (s *Scheduler) scheduleSessionEndTask() {
	// Check if session end is enabled (default enabled)
	if os.Getenv("SESSION_END_SCHEDULER_ENABLED") == "false" {
		logging.Logger.Info("Session end scheduler is disabled (set SESSION_END_SCHEDULER_ENABLED=true to enable)")
		return
	}

	// Get scheduled time from env or default to 6 PM
	scheduledTime := os.Getenv("SESSION_END_TIME")
	if scheduledTime == "" {
		scheduledTime = "18:00"
	}

	task := &ScheduledTask{
		Name:     "session-end",
		Schedule: scheduledTime,
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
		logging.Logger.Warn("Session end task already running, skipping")
		return
	}
	defer task.finish(24 * time.Hour)

	logging.Logger.Info("Starting scheduled session end")
	startTime := time.Now()

	// Get timeout from env or default to 10 minutes
	timeoutMinutes := 10
	if timeoutStr := os.Getenv("SESSION_END_TIMEOUT_MINUTES"); timeoutStr != "" {
		if parsed, err := strconv.Atoi(timeoutStr); err == nil && parsed > 0 {
			timeoutMinutes = parsed
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	// Call the active service to end all daily sessions
	result, err := s.activeService.EndDailySessions(ctx)
	if err != nil {
		logging.Logger.WithError(err).Error("Scheduled session end failed")
		return
	}

	duration := time.Since(startTime)
	logging.Logger.WithFields(map[string]interface{}{
		"duration":          duration.Round(time.Second).String(),
		"sessions_ended":    result.SessionsEnded,
		"visits_ended":      result.VisitsEnded,
		"supervisors_ended": result.SupervisorsEnded,
		"success":           result.Success,
	}).Info("Scheduled session end completed")

	if len(result.Errors) > 0 {
		logging.Logger.WithField("error_count", len(result.Errors)).Warn("Session end completed with errors")
		for i, errMsg := range result.Errors {
			if i < 10 { // Log first 10 errors
				logging.Logger.WithFields(map[string]interface{}{
					"error_index": i + 1,
					"error":       errMsg,
				}).Warn("Session end error")
			}
		}
		if len(result.Errors) > 10 {
			logging.Logger.WithField("additional_errors", len(result.Errors)-10).Warn("Additional errors not shown")
		}
	}
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Check if session cleanup is enabled (default enabled)
	if os.Getenv("SESSION_CLEANUP_ENABLED") == "false" {
		logging.Logger.Info("Session cleanup is disabled (set SESSION_CLEANUP_ENABLED=true to enable)")
		return
	}

	// Parse and store configuration once during initialization
	s.sessionCleanupIntervalMinutes = 15
	if envInterval := os.Getenv("SESSION_CLEANUP_INTERVAL_MINUTES"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			s.sessionCleanupIntervalMinutes = parsed
		}
	}

	s.sessionAbandonedThresholdMinutes = 60
	if envThreshold := os.Getenv("SESSION_ABANDONED_THRESHOLD_MINUTES"); envThreshold != "" {
		if parsed, err := strconv.Atoi(envThreshold); err == nil && parsed > 0 {
			s.sessionAbandonedThresholdMinutes = parsed
		}
	}

	task := &ScheduledTask{
		Name:     "session-cleanup",
		Schedule: strconv.Itoa(s.sessionCleanupIntervalMinutes) + "m",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	// Capture configuration values before starting goroutine to prevent data race.
	// These values are passed as parameters to avoid unsynchronized reads of struct fields.
	intervalMinutes := s.sessionCleanupIntervalMinutes
	thresholdMinutes := s.sessionAbandonedThresholdMinutes

	s.wg.Add(1)
	go s.runSessionCleanupTask(task, intervalMinutes, thresholdMinutes)
}

// runSessionCleanupTask runs the session cleanup task at configured intervals.
// Configuration values are passed as parameters to avoid data races with struct fields.
func (s *Scheduler) runSessionCleanupTask(task *ScheduledTask, intervalMinutes, thresholdMinutes int) {
	defer s.wg.Done()

	interval := time.Duration(intervalMinutes) * time.Minute
	logging.Logger.WithField("interval_minutes", intervalMinutes).Info("Session cleanup task scheduled")

	// Run immediately on startup (after brief delay to let other services initialize)
	time.Sleep(30 * time.Second)
	s.executeSessionCleanup(task, intervalMinutes, thresholdMinutes)

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
		logging.Logger.WithError(err).Error("Session cleanup failed")
		return
	}

	if count > 0 {
		logging.Logger.WithFields(map[string]interface{}{
			"sessions_cleaned":     count,
			"threshold_minutes":    thresholdMinutes,
		}).Info("Abandoned sessions cleaned up")
	}
}
