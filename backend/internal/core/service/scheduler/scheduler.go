package scheduler

import (
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
)

// Scheduler manages scheduled tasks
// It coordinates cleanup jobs, session management, and token maintenance.
type Scheduler struct {
	activeService     active.Service
	cleanupService    active.CleanupService
	authCleanup       AuthCleanup
	invitationCleanup InvitationCleaner
	cleanupJobs       []CleanupJob
	tasks             map[string]*ScheduledTask
	mu                sync.RWMutex
	// done signals goroutines to stop when closed (replaces stored context)
	done   chan struct{}
	wg     sync.WaitGroup
	config Config
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
func NewScheduler(activeService active.Service, cleanupService active.CleanupService, authService AuthCleanup, invitationService InvitationCleaner, config Config) *Scheduler {
	return &Scheduler{
		activeService:     activeService,
		cleanupService:    cleanupService,
		authCleanup:       authService,
		invitationCleanup: invitationService,
		cleanupJobs:       buildCleanupJobs(authService, invitationService),
		tasks:             make(map[string]*ScheduledTask),
		done:              make(chan struct{}),
		config:            config,
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	logger.Logger.Info("Starting scheduler service")

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
	logger.Logger.Info("Stopping scheduler service")
	close(s.done)
	s.wg.Wait()
	logger.Logger.Info("Scheduler service stopped")
}
