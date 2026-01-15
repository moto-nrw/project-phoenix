package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/services/active"
)

// Log format constants to avoid string duplication
const (
	fmtAndMoreErrors = "  ... and %d more errors"
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
		log.Printf("Invalid scheduled time for %s: %v", task.Name, err)
		return
	}

	nextRun := calculateNextRun(hour, minute)
	initialWait := time.Until(nextRun)
	log.Printf("Scheduled %s task will run in %v (at %v)", task.Name, initialWait.Round(time.Minute), nextRun.Format("2006-01-02 15:04:05"))

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
	log.Println("Starting scheduler service...")

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
	log.Println("Stopping scheduler service...")
	close(s.done)
	s.wg.Wait()
	log.Println("Scheduler service stopped")
}

// scheduleCleanupTask schedules the daily cleanup task
func (s *Scheduler) scheduleCleanupTask() {
	// Check if cleanup is enabled
	if os.Getenv("CLEANUP_SCHEDULER_ENABLED") != "true" {
		log.Println("Cleanup scheduler is disabled (set CLEANUP_SCHEDULER_ENABLED=true to enable)")
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
		log.Println("Cleanup task already running, skipping...")
		return
	}
	defer task.finish(24 * time.Hour)

	log.Println("Starting scheduled visit cleanup...")
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
		log.Printf("ERROR: Scheduled cleanup failed: %v", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("Scheduled cleanup completed in %v: processed %d students, deleted %d records, success: %v",
		duration.Round(time.Second),
		result.StudentsProcessed,
		result.RecordsDeleted,
		result.Success,
	)

	if len(result.Errors) > 0 {
		log.Printf("Cleanup completed with %d errors", len(result.Errors))
		for i, err := range result.Errors {
			if i < 10 { // Log first 10 errors
				log.Printf("  - Student %d: %s", err.StudentID, err.Error)
			}
		}
		if len(result.Errors) > 10 {
			log.Printf(fmtAndMoreErrors, len(result.Errors)-10)
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

	log.Println("Token cleanup task scheduled to run every hour")

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

	log.Println("Running scheduled token cleanup...")
	startTime := time.Now()

	// Use reflection to call CleanupExpiredTokens method
	if err := s.RunCleanupJobs(); err != nil {
		log.Printf("ERROR: Token cleanup failed: %v", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("Token cleanup completed in %v", duration.Round(time.Millisecond))
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	if len(s.cleanupJobs) == 0 {
		log.Println("No cleanup jobs registered; skipping token cleanup")
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
			log.Printf("ERROR: %s failed: %v", job.Description, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		log.Printf("%s removed %d records", job.Description, count)
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
		log.Println("Session end scheduler is disabled (set SESSION_END_SCHEDULER_ENABLED=true to enable)")
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
		log.Println("Session end task already running, skipping...")
		return
	}
	defer task.finish(24 * time.Hour)

	log.Println("Starting scheduled session end...")
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
		log.Printf("ERROR: Scheduled session end failed: %v", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("Scheduled session end completed in %v: ended %d sessions, %d visits, %d supervisors, success: %v",
		duration.Round(time.Second),
		result.SessionsEnded,
		result.VisitsEnded,
		result.SupervisorsEnded,
		result.Success,
	)

	if len(result.Errors) > 0 {
		log.Printf("Session end completed with %d errors", len(result.Errors))
		for i, errMsg := range result.Errors {
			if i < 10 { // Log first 10 errors
				log.Printf("  - Error %d: %s", i+1, errMsg)
			}
		}
		if len(result.Errors) > 10 {
			log.Printf(fmtAndMoreErrors, len(result.Errors)-10)
		}
	}
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Check if session cleanup is enabled (default enabled)
	if os.Getenv("SESSION_CLEANUP_ENABLED") == "false" {
		log.Println("Session cleanup is disabled (set SESSION_CLEANUP_ENABLED=true to enable)")
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
	log.Printf("Session cleanup task scheduled to run every %d minutes", intervalMinutes)

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
		log.Printf("ERROR: Session cleanup failed: %v", err)
		return
	}

	if count > 0 {
		log.Printf("Session cleanup: cleaned up %d abandoned sessions (threshold: %d minutes)", count, thresholdMinutes)
	}
}
