package scheduler

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/services/active"
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

// WorkSessionCleaner exposes the cleanup routine for stale work sessions.
type WorkSessionCleaner interface {
	CleanupOpenSessions(ctx context.Context) (int, error)
}

// BreakAutoEnder exposes the method to auto-end expired breaks.
type BreakAutoEnder interface {
	AutoEndExpiredBreaks(ctx context.Context) (int, error)
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	activeService      active.Service
	cleanupService     active.CleanupService
	authCleanup        AuthCleanup
	invitationCleanup  InvitationCleaner
	workSessionCleanup WorkSessionCleaner
	breakAutoEnder     BreakAutoEnder
	cleanupJobs        []CleanupJob
	tasks              map[string]*ScheduledTask
	mu                 sync.RWMutex
	logger             *slog.Logger
	// done signals goroutines to stop when closed (replaces stored context)
	done chan struct{}
	wg   sync.WaitGroup

	// Session cleanup configuration (parsed once during initialization)
	sessionCleanupIntervalMinutes    int
	sessionAbandonedThresholdMinutes int

	// Break auto-end configuration (parsed once during initialization)
	breakAutoEndIntervalSeconds int
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

// NewScheduler creates a new scheduler
func NewScheduler(activeService active.Service, cleanupService active.CleanupService, authService AuthCleanup, invitationService InvitationCleaner, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		activeService:     activeService,
		cleanupService:    cleanupService,
		authCleanup:       authService,
		invitationCleanup: invitationService,
		cleanupJobs:       buildCleanupJobs(authService, invitationService),
		tasks:             make(map[string]*ScheduledTask),
		done:              make(chan struct{}),
		logger:            logger,
	}
}

// getLogger returns the scheduler's logger, falling back to slog.Default() if nil.
func (s *Scheduler) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// SetWorkSessionCleaner sets the work session cleanup service (optional).
func (s *Scheduler) SetWorkSessionCleaner(wsc WorkSessionCleaner) {
	s.workSessionCleanup = wsc
}

// SetBreakAutoEnder sets the break auto-end service (optional).
func (s *Scheduler) SetBreakAutoEnder(bae BreakAutoEnder) {
	s.breakAutoEnder = bae
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.getLogger().Info("starting scheduler service")

	// Schedule daily data cleanup at 2 AM
	s.scheduleCleanupTask()

	// Schedule daily session end at configurable time (default 6 PM)
	s.scheduleSessionEndTask()

	// Schedule token cleanup every hour
	s.scheduleTokenCleanupTask()

	// Schedule abandoned session cleanup
	s.scheduleSessionCleanupTask()

	// Schedule break auto-end task
	s.scheduleBreakAutoEndTask()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.getLogger().Info("stopping scheduler service")
	close(s.done)
	s.wg.Wait()
	s.getLogger().Info("scheduler service stopped")
}

// scheduleCleanupTask schedules the daily cleanup task
func (s *Scheduler) scheduleCleanupTask() {
	// Check if cleanup is enabled
	if os.Getenv("CLEANUP_SCHEDULER_ENABLED") != "true" {
		s.getLogger().Info("cleanup scheduler is disabled")
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
	go s.runCleanupTask(task)
}

// runCleanupTask runs the cleanup task on schedule
func (s *Scheduler) runCleanupTask(task *ScheduledTask) {
	defer s.wg.Done()

	// Parse scheduled time
	parts := strings.Split(task.Schedule, ":")
	if len(parts) != 2 {
		s.getLogger().Error("invalid scheduled time format (expected HH:MM)",
			slog.String("schedule", task.Schedule))
		return
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		s.getLogger().Error("invalid hour in scheduled time",
			slog.String("schedule", task.Schedule))
		return
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		s.getLogger().Error("invalid minute in scheduled time",
			slog.String("schedule", task.Schedule))
		return
	}

	// Calculate time until scheduled time
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(nextRun) {
		// If it's already past scheduled time today, schedule for tomorrow
		nextRun = nextRun.Add(24 * time.Hour)
	}

	// Wait until first run
	initialWait := time.Until(nextRun)
	s.getLogger().Info("scheduled cleanup task will run",
		slog.Duration("in", initialWait.Round(time.Minute)),
		slog.String("at", nextRun.Format("2006-01-02 15:04:05")))

	select {
	case <-time.After(initialWait):
		// Run immediately at scheduled time
		s.executeCleanup(task)
	case <-s.done:
		return
	}

	// Then run every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeCleanup(task)
		case <-s.done:
			return
		}
	}
}

// executeCleanup executes the cleanup task
func (s *Scheduler) executeCleanup(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		s.getLogger().Warn("cleanup task already running, skipping")
		return
	}
	task.Running = true
	task.LastRun = time.Now()
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.NextRun = time.Now().Add(24 * time.Hour)
		task.mu.Unlock()
	}()

	s.getLogger().Info("starting scheduled cleanup (visits + supervisors)")
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
		s.getLogger().Error("scheduled cleanup failed", "error", err)
		return
	}

	duration := time.Since(startTime)
	s.getLogger().Info("scheduled cleanup completed",
		slog.Duration("duration", duration.Round(time.Second)),
		slog.Int("students_processed", result.StudentsProcessed),
		slog.Int64("records_deleted", result.RecordsDeleted),
		slog.Bool("success", result.Success))

	if len(result.Errors) > 0 {
		s.getLogger().Warn("cleanup completed with errors",
			slog.Int("error_count", len(result.Errors)))
		for i, err := range result.Errors {
			if i < 10 { // Log first 10 errors
				s.getLogger().Warn("cleanup error",
					slog.Int64("student_id", err.StudentID),
					slog.String("error", err.Error))
			}
		}
		if len(result.Errors) > 10 {
			s.getLogger().Warn("additional cleanup errors",
				slog.Int("count", len(result.Errors)-10))
		}
	}

	// Clean up stale supervisor records from previous days
	supervisorResult, err := s.cleanupService.CleanupStaleSupervisors(ctx)
	if err != nil {
		s.getLogger().Error("scheduled supervisor cleanup failed", "error", err)
	} else {
		s.getLogger().Info("supervisor cleanup completed",
			slog.Int("records_closed", supervisorResult.RecordsClosed),
			slog.Int("staff_affected", supervisorResult.StaffAffected),
			slog.Bool("success", supervisorResult.Success))
	}

	// Clean up open work sessions from previous days (auto-checkout at end of day)
	if s.workSessionCleanup != nil {
		closedCount, wsErr := s.workSessionCleanup.CleanupOpenSessions(ctx)
		if wsErr != nil {
			s.getLogger().Error("work session cleanup failed", "error", wsErr)
		} else if closedCount > 0 {
			s.getLogger().Info("work session cleanup completed",
				slog.Int("sessions_closed", closedCount))
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

	s.getLogger().Info("token cleanup task scheduled to run every hour")

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

	s.getLogger().Info("running scheduled token cleanup")
	startTime := time.Now()

	// Use reflection to call CleanupExpiredTokens method
	if err := s.RunCleanupJobs(); err != nil {
		s.getLogger().Error("token cleanup failed", "error", err)
		return
	}

	duration := time.Since(startTime)
	s.getLogger().Info("token cleanup completed",
		slog.Duration("duration", duration.Round(time.Millisecond)))
}

// RunCleanupJobs executes all token-related cleanup tasks in sequence.
func (s *Scheduler) RunCleanupJobs() error {
	if len(s.cleanupJobs) == 0 {
		s.getLogger().Info("no cleanup jobs registered, skipping token cleanup")
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
			s.getLogger().Error("cleanup job failed",
				slog.String("job", job.Description),
				"error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		s.getLogger().Info("cleanup job completed",
			slog.String("job", job.Description),
			slog.Int("records_deleted", count))
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
		s.getLogger().Info("session end scheduler is disabled")
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
	go s.runSessionEndTask(task)
}

// runSessionEndTask runs the session end task on schedule
func (s *Scheduler) runSessionEndTask(task *ScheduledTask) {
	defer s.wg.Done()

	// Parse scheduled time
	parts := strings.Split(task.Schedule, ":")
	if len(parts) != 2 {
		s.getLogger().Error("invalid session end time format (expected HH:MM)",
			slog.String("schedule", task.Schedule))
		return
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		s.getLogger().Error("invalid hour in session end time",
			slog.String("schedule", task.Schedule))
		return
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		s.getLogger().Error("invalid minute in session end time",
			slog.String("schedule", task.Schedule))
		return
	}

	// Calculate time until scheduled time
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(nextRun) {
		// If it's already past scheduled time today, schedule for tomorrow
		nextRun = nextRun.Add(24 * time.Hour)
	}

	// Wait until first run
	initialWait := time.Until(nextRun)
	s.getLogger().Info("scheduled session end task will run",
		slog.Duration("in", initialWait.Round(time.Minute)),
		slog.String("at", nextRun.Format("2006-01-02 15:04:05")))

	select {
	case <-time.After(initialWait):
		// Run immediately at scheduled time
		s.executeSessionEnd(task)
	case <-s.done:
		return
	}

	// Then run every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeSessionEnd(task)
		case <-s.done:
			return
		}
	}
}

// executeSessionEnd executes the session end task
func (s *Scheduler) executeSessionEnd(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		s.getLogger().Warn("session end task already running, skipping")
		return
	}
	task.Running = true
	task.LastRun = time.Now()
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.Running = false
		task.NextRun = time.Now().Add(24 * time.Hour)
		task.mu.Unlock()
	}()

	s.getLogger().Info("starting scheduled session end")
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
		s.getLogger().Error("scheduled session end failed", "error", err)
		return
	}

	duration := time.Since(startTime)
	s.getLogger().Info("scheduled session end completed",
		slog.Duration("duration", duration.Round(time.Second)),
		slog.Int("sessions_ended", result.SessionsEnded),
		slog.Int("visits_ended", result.VisitsEnded),
		slog.Int("supervisors_ended", result.SupervisorsEnded),
		slog.Bool("success", result.Success))

	if len(result.Errors) > 0 {
		s.getLogger().Warn("session end completed with errors",
			slog.Int("error_count", len(result.Errors)))
		for i, errMsg := range result.Errors {
			if i < 10 { // Log first 10 errors
				s.getLogger().Warn("session end error",
					slog.Int("error_number", i+1),
					slog.String("message", errMsg))
			}
		}
		if len(result.Errors) > 10 {
			s.getLogger().Warn("additional session end errors",
				slog.Int("count", len(result.Errors)-10))
		}
	}
}

// scheduleSessionCleanupTask schedules the abandoned session cleanup task
func (s *Scheduler) scheduleSessionCleanupTask() {
	// Check if session cleanup is enabled (default enabled)
	if os.Getenv("SESSION_CLEANUP_ENABLED") == "false" {
		s.getLogger().Info("session cleanup is disabled")
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
	s.getLogger().Info("session cleanup task scheduled",
		slog.Int("interval_minutes", intervalMinutes))

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
		task.NextRun = time.Now().Add(time.Duration(intervalMinutes) * time.Minute)
		task.mu.Unlock()
	}()

	// Add timeout to prevent cleanup from blocking shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	threshold := time.Duration(thresholdMinutes) * time.Minute

	// Call the active service cleanup method
	count, err := s.activeService.CleanupAbandonedSessions(ctx, threshold)
	if err != nil {
		s.getLogger().Error("session cleanup failed", "error", err)
		return
	}

	if count > 0 {
		s.getLogger().Info("session cleanup completed",
			slog.Int("abandoned_sessions", count),
			slog.Int("threshold_minutes", thresholdMinutes))
	}
}

// scheduleBreakAutoEndTask schedules the break auto-end task
func (s *Scheduler) scheduleBreakAutoEndTask() {
	// Check if break auto-end is enabled (skip if no service configured)
	if s.breakAutoEnder == nil {
		s.getLogger().Info("break auto-end not configured (no BreakAutoEnder service)")
		return
	}

	// Check if explicitly disabled
	if os.Getenv("BREAK_AUTO_END_ENABLED") == "false" {
		s.getLogger().Info("break auto-end is disabled")
		return
	}

	// Parse interval from env (default 60 seconds)
	s.breakAutoEndIntervalSeconds = 60
	if envInterval := os.Getenv("BREAK_AUTO_END_INTERVAL_SECONDS"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			s.breakAutoEndIntervalSeconds = parsed
		}
	}

	task := &ScheduledTask{
		Name:     "break-auto-end",
		Schedule: strconv.Itoa(s.breakAutoEndIntervalSeconds) + "s",
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	// Capture configuration value before starting goroutine to prevent data race
	intervalSeconds := s.breakAutoEndIntervalSeconds

	s.wg.Add(1)
	go s.runBreakAutoEndTask(task, intervalSeconds)
}

// runBreakAutoEndTask runs the break auto-end task at configured intervals.
func (s *Scheduler) runBreakAutoEndTask(task *ScheduledTask, intervalSeconds int) {
	defer s.wg.Done()

	interval := time.Duration(intervalSeconds) * time.Second
	s.getLogger().Info("break auto-end task scheduled",
		slog.Int("interval_seconds", intervalSeconds))

	// Run immediately on startup (after brief delay)
	time.Sleep(10 * time.Second)
	s.executeBreakAutoEnd(task, intervalSeconds)

	// Then run at configured interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeBreakAutoEnd(task, intervalSeconds)
		case <-s.done:
			return
		}
	}
}

// executeBreakAutoEnd executes the break auto-end task.
func (s *Scheduler) executeBreakAutoEnd(task *ScheduledTask, intervalSeconds int) {
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
		task.NextRun = time.Now().Add(time.Duration(intervalSeconds) * time.Second)
		task.mu.Unlock()
	}()

	// Add timeout to prevent blocking shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := s.breakAutoEnder.AutoEndExpiredBreaks(ctx)
	if err != nil {
		s.getLogger().Error("break auto-end failed", "error", err)
		return
	}

	if count > 0 {
		s.getLogger().Info("break auto-end completed",
			slog.Int("breaks_ended", count))
	}
}
