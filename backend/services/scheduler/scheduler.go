package scheduler

import (
	"context"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/services/active"
)

// Scheduler manages scheduled tasks
type Scheduler struct {
	activeService  active.Service
	cleanupService active.CleanupService
	authService    interface{} // Using interface to avoid circular dependency
	tasks          map[string]*ScheduledTask
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
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
func NewScheduler(activeService active.Service, cleanupService active.CleanupService, authService interface{}) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		activeService:  activeService,
		cleanupService: cleanupService,
		authService:    authService,
		tasks:          make(map[string]*ScheduledTask),
		ctx:            ctx,
		cancel:         cancel,
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

	// Schedule checkout processing every minute
	s.scheduleCheckoutProcessingTask()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler service...")
	s.cancel()
	s.wg.Wait()
	log.Println("Scheduler service stopped")
}

// scheduleCleanupTask schedules the daily cleanup task
func (s *Scheduler) scheduleCleanupTask() {
	// Check if cleanup is enabled
	if enabled := os.Getenv("CLEANUP_SCHEDULER_ENABLED"); enabled != "true" {
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
	go s.runCleanupTask(task)
}

// runCleanupTask runs the cleanup task on schedule
func (s *Scheduler) runCleanupTask(task *ScheduledTask) {
	defer s.wg.Done()

	// Parse scheduled time
	parts := strings.Split(task.Schedule, ":")
	if len(parts) != 2 {
		log.Printf("Invalid scheduled time format: %s (expected HH:MM)", task.Schedule)
		return
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		log.Printf("Invalid hour in scheduled time: %s", task.Schedule)
		return
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		log.Printf("Invalid minute in scheduled time: %s", task.Schedule)
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
	log.Printf("Scheduled cleanup task will run in %v (at %v)", initialWait.Round(time.Minute), nextRun.Format("2006-01-02 15:04:05"))

	select {
	case <-time.After(initialWait):
		// Run immediately at scheduled time
		s.executeCleanup(task)
	case <-s.ctx.Done():
		return
	}

	// Then run every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeCleanup(task)
		case <-s.ctx.Done():
			return
		}
	}
}

// executeCleanup executes the cleanup task
func (s *Scheduler) executeCleanup(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		log.Println("Cleanup task already running, skipping...")
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
			log.Printf("  ... and %d more errors", len(result.Errors)-10)
		}
	}
}

// GetTaskStatus returns the status of all scheduled tasks
func (s *Scheduler) GetTaskStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make(map[string]interface{})
	for name, task := range s.tasks {
		task.mu.Lock()
		status[name] = map[string]interface{}{
			"name":     task.Name,
			"schedule": task.Schedule,
			"lastRun":  task.LastRun,
			"nextRun":  task.NextRun,
			"running":  task.Running,
		}
		task.mu.Unlock()
	}

	return status
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
		case <-s.ctx.Done():
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

	log.Println("Running scheduled token cleanup...")
	startTime := time.Now()

	// Use reflection to call CleanupExpiredTokens method
	if s.authService != nil {
		method := reflect.ValueOf(s.authService).MethodByName("CleanupExpiredTokens")
		if method.IsValid() {
			ctx := context.Background()
			ctxValue := reflect.ValueOf(ctx)
			results := method.Call([]reflect.Value{ctxValue})

			if len(results) == 2 {
				count := results[0].Int()
				errInterface := results[1].Interface()

				if errInterface != nil {
					if err, ok := errInterface.(error); ok && err != nil {
						log.Printf("ERROR: Token cleanup failed: %v", err)
						return
					}
				}

				duration := time.Since(startTime)
				log.Printf("Token cleanup completed in %v: deleted %d expired tokens",
					duration.Round(time.Millisecond), count)
			}
		}
	}
}

// scheduleSessionEndTask schedules the daily session end task
func (s *Scheduler) scheduleSessionEndTask() {
	// Check if session end is enabled (default enabled)
	if enabled := os.Getenv("SESSION_END_SCHEDULER_ENABLED"); enabled == "false" {
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
	go s.runSessionEndTask(task)
}

// runSessionEndTask runs the session end task on schedule
func (s *Scheduler) runSessionEndTask(task *ScheduledTask) {
	defer s.wg.Done()

	// Parse scheduled time
	parts := strings.Split(task.Schedule, ":")
	if len(parts) != 2 {
		log.Printf("Invalid session end time format: %s (expected HH:MM)", task.Schedule)
		return
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		log.Printf("Invalid hour in session end time: %s", task.Schedule)
		return
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		log.Printf("Invalid minute in session end time: %s", task.Schedule)
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
	log.Printf("Scheduled session end task will run in %v (at %v)", initialWait.Round(time.Minute), nextRun.Format("2006-01-02 15:04:05"))

	select {
	case <-time.After(initialWait):
		// Run immediately at scheduled time
		s.executeSessionEnd(task)
	case <-s.ctx.Done():
		return
	}

	// Then run every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeSessionEnd(task)
		case <-s.ctx.Done():
			return
		}
	}
}

// executeSessionEnd executes the session end task
func (s *Scheduler) executeSessionEnd(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		log.Println("Session end task already running, skipping...")
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
	log.Printf("Scheduled session end completed in %v: ended %d sessions, %d visits, success: %v",
		duration.Round(time.Second),
		result.SessionsEnded,
		result.VisitsEnded,
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
			log.Printf("  ... and %d more errors", len(result.Errors)-10)
		}
	}
}

// scheduleCheckoutProcessingTask schedules the scheduled checkout processing task
func (s *Scheduler) scheduleCheckoutProcessingTask() {
	// Check if scheduled checkout processing is enabled (default enabled)
	if enabled := os.Getenv("SCHEDULED_CHECKOUT_ENABLED"); enabled == "false" {
		log.Println("Scheduled checkout processing is disabled (set SCHEDULED_CHECKOUT_ENABLED=true to enable)")
		return
	}

	task := &ScheduledTask{
		Name:     "scheduled-checkout-processing",
		Schedule: "1m", // Run every minute
	}

	s.mu.Lock()
	s.tasks[task.Name] = task
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runCheckoutProcessingTask(task)
}

// runCheckoutProcessingTask runs the checkout processing task every minute
func (s *Scheduler) runCheckoutProcessingTask(task *ScheduledTask) {
	defer s.wg.Done()

	log.Println("Scheduled checkout processing task scheduled to run every minute")

	// Run immediately on startup
	s.executeCheckoutProcessing(task)

	// Then run every minute
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeCheckoutProcessing(task)
		case <-s.ctx.Done():
			return
		}
	}
}

// executeCheckoutProcessing executes the checkout processing task
func (s *Scheduler) executeCheckoutProcessing(task *ScheduledTask) {
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
		task.NextRun = time.Now().Add(time.Minute)
		task.mu.Unlock()
	}()

	ctx := context.Background()

	// Process due scheduled checkouts
	result, err := s.activeService.ProcessDueScheduledCheckouts(ctx)
	if err != nil {
		log.Printf("ERROR: Scheduled checkout processing failed: %v", err)
		return
	}

	// Only log if there were checkouts to process
	if result.CheckoutsExecuted > 0 {
		if result.Success {
			log.Printf("Scheduled checkout processing: processed %d checkouts (%d visits ended, %d attendance updated)",
				result.CheckoutsExecuted, result.VisitsEnded, result.AttendanceUpdated)
		} else {
			log.Printf("Scheduled checkout processing: partial success - processed %d checkouts with %d errors",
				result.CheckoutsExecuted, len(result.Errors))
			for i, errMsg := range result.Errors {
				if i < 5 { // Log first 5 errors
					log.Printf("  - Error: %s", errMsg)
				}
			}
			if len(result.Errors) > 5 {
				log.Printf("  ... and %d more errors", len(result.Errors)-5)
			}
		}
	}
}
