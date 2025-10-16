package services

import (
	"context"
	"log"
	"sync"
	"time"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// SessionCleanupService provides background cleanup for abandoned sessions
type SessionCleanupService struct {
	activeService activeSvc.Service
	logger        *log.Logger
	ticker        *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
	isRunning     bool
	mu            sync.Mutex
}

// NewSessionCleanupService creates a new session cleanup service
func NewSessionCleanupService(activeService activeSvc.Service, logger *log.Logger) *SessionCleanupService {
	return &SessionCleanupService{
		activeService: activeService,
		logger:        logger,
		isRunning:     false,
	}
}

// Start begins the background cleanup process
func (s *SessionCleanupService) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		s.logger.Println("Session cleanup service is already running")
		return
	}

	s.logger.Println("Starting session cleanup service...")

	// Create new context for this session
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.isRunning = true
	s.mu.Unlock()

	// Check for abandoned sessions every 15 minutes (not aggressive)
	s.ticker = time.NewTicker(15 * time.Minute)

	go func() {
		// Run initial cleanup after 5 minutes to avoid startup interference
		initialDelay := time.NewTimer(5 * time.Minute)

		for {
			select {
			case <-initialDelay.C:
				// Run first cleanup
				s.cleanupAbandonedSessions()
				initialDelay.Stop()

			case <-s.ticker.C:
				s.cleanupAbandonedSessions()

			case <-s.ctx.Done():
				s.logger.Println("Session cleanup service stopped")
				return
			}
		}
	}()

	s.logger.Println("Session cleanup service started successfully")
}

// Stop halts the background cleanup process
func (s *SessionCleanupService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.logger.Println("Stopping session cleanup service...")
	s.isRunning = false

	if s.ticker != nil {
		s.ticker.Stop()
	}

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil // Prevent double-cancel
	}

	s.logger.Println("Session cleanup service stopped")
}

// IsRunning returns whether the service is currently running
func (s *SessionCleanupService) IsRunning() bool {
	return s.isRunning
}

// cleanupAbandonedSessions performs the actual cleanup of abandoned sessions
func (s *SessionCleanupService) cleanupAbandonedSessions() {
	s.logger.Println("Starting cleanup of abandoned sessions...")

	// Clean up sessions that have been active for > 2 hours (safety net)
	// This catches edge cases where devices fail to notify server
	abandonedThreshold := 2 * time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := s.activeService.CleanupAbandonedSessions(ctx, abandonedThreshold)
	if err != nil {
		s.logger.Printf("Failed to cleanup abandoned sessions: %v", err)
		return
	}

	if count > 0 {
		s.logger.Printf("Cleaned up %d abandoned sessions", count)
	} else {
		s.logger.Println("No abandoned sessions found")
	}
}

// RunManualCleanup allows manual triggering of session cleanup
func (s *SessionCleanupService) RunManualCleanup(threshold time.Duration) (int, error) {
	s.logger.Printf("Running manual cleanup with threshold: %v", threshold)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	count, err := s.activeService.CleanupAbandonedSessions(ctx, threshold)
	if err != nil {
		s.logger.Printf("Manual cleanup failed: %v", err)
		return 0, err
	}

	s.logger.Printf("Manual cleanup completed: %d sessions cleaned", count)
	return count, nil
}

// GetStatus returns the current status of the cleanup service
func (s *SessionCleanupService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":           s.isRunning,
		"check_interval":    "15 minutes",
		"abandon_threshold": "2 hours",
	}

	if s.ticker != nil && s.isRunning {
		status["next_cleanup"] = "within 15 minutes"
	}

	return status
}
