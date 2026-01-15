package scheduler

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Services for Cleanup Jobs
// =============================================================================

type fakeAuthCleanup struct {
	mu              sync.Mutex
	tokenCalls      int
	passwordCalls   int
	rateLimitCalls  int
	tokenResult     int
	passwordResult  int
	rateLimitResult int
	tokenErr        error
	passwordErr     error
	rateLimitErr    error
}

func (f *fakeAuthCleanup) CleanupExpiredTokens(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenCalls++
	return f.tokenResult, f.tokenErr
}

func (f *fakeAuthCleanup) CleanupExpiredPasswordResetTokens(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.passwordCalls++
	return f.passwordResult, f.passwordErr
}

func (f *fakeAuthCleanup) CleanupExpiredRateLimits(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rateLimitCalls++
	return f.rateLimitResult, f.rateLimitErr
}

type fakeInvitationCleaner struct {
	mu      sync.Mutex
	calls   int
	result  int
	callErr error
}

func (f *fakeInvitationCleaner) CleanupExpiredInvitations(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.result, f.callErr
}

// =============================================================================
// NewScheduler Tests
// =============================================================================

func TestNewScheduler(t *testing.T) {
	auth := &fakeAuthCleanup{}
	invitations := &fakeInvitationCleaner{}

	s := NewScheduler(nil, nil, auth, invitations)

	require.NotNil(t, s)
	assert.NotNil(t, s.tasks)
	assert.NotNil(t, s.done)
	assert.Len(t, s.cleanupJobs, 4) // 3 auth + 1 invitation
}

func TestNewScheduler_NilServices(t *testing.T) {
	s := NewScheduler(nil, nil, nil, nil)

	require.NotNil(t, s)
	assert.Empty(t, s.cleanupJobs)
}

func TestNewScheduler_OnlyAuthService(t *testing.T) {
	auth := &fakeAuthCleanup{}

	s := NewScheduler(nil, nil, auth, nil)

	require.NotNil(t, s)
	assert.Len(t, s.cleanupJobs, 3) // 3 auth jobs only
}

func TestNewScheduler_OnlyInvitationService(t *testing.T) {
	invitations := &fakeInvitationCleaner{}

	s := NewScheduler(nil, nil, nil, invitations)

	require.NotNil(t, s)
	assert.Len(t, s.cleanupJobs, 1) // 1 invitation job only
}

// =============================================================================
// Start/Stop Lifecycle Tests
// =============================================================================

func TestScheduler_StartStop(t *testing.T) {
	// Disable all scheduled tasks to test pure lifecycle
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "false"))
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "false"))
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "false"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
	}()

	s := NewScheduler(nil, nil, nil, nil)

	// Start should not panic
	assert.NotPanics(t, func() {
		s.Start()
	})

	// Stop should not panic and should complete
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete within timeout")
	}
}

func TestScheduler_StopWithoutStart(t *testing.T) {
	s := NewScheduler(nil, nil, nil, nil)

	// Stop without start should not panic
	assert.NotPanics(t, func() {
		s.Stop()
	})
}

func TestScheduler_StartWithTokenCleanupOnly(t *testing.T) {
	// Enable only token cleanup (runs immediately then every hour)
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "false"))
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "false"))
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "false"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
	}()

	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}

	s := NewScheduler(nil, nil, auth, nil)
	s.Start()

	// Give token cleanup task time to register and run once
	time.Sleep(100 * time.Millisecond)

	// Verify task was registered
	s.mu.RLock()
	_, hasTokenCleanup := s.tasks["token-cleanup"]
	s.mu.RUnlock()
	assert.True(t, hasTokenCleanup, "token-cleanup task should be registered")

	// Stop and wait
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete within timeout")
	}
}

// =============================================================================
// RunCleanupJobs Tests
// =============================================================================

func TestRunCleanupJobsExecutesAllJobs(t *testing.T) {
	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}
	invitations := &fakeInvitationCleaner{result: 4}

	s := NewScheduler(nil, nil, auth, invitations)

	if err := s.RunCleanupJobs(); err != nil {
		t.Fatalf("RunCleanupJobs() returned error: %v", err)
	}

	if auth.tokenCalls != 1 || auth.passwordCalls != 1 || auth.rateLimitCalls != 1 {
		t.Fatalf("expected auth cleanup jobs to be called once each, got tokens=%d passwords=%d rate=%d",
			auth.tokenCalls, auth.passwordCalls, auth.rateLimitCalls)
	}

	if invitations.calls != 1 {
		t.Fatalf("expected invitation cleanup to be called once, got %d", invitations.calls)
	}
}

func TestRunCleanupJobsReturnsFirstErrorAndContinues(t *testing.T) {
	expectedErr := errors.New("rate limit cleanup failed")

	auth := &fakeAuthCleanup{
		rateLimitErr: expectedErr,
	}
	invitations := &fakeInvitationCleaner{}

	s := NewScheduler(nil, nil, auth, invitations)

	err := s.RunCleanupJobs()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if auth.tokenCalls != 1 || auth.passwordCalls != 1 || auth.rateLimitCalls != 1 {
		t.Fatalf("expected auth cleanup jobs to be called once each despite error, got tokens=%d passwords=%d rate=%d",
			auth.tokenCalls, auth.passwordCalls, auth.rateLimitCalls)
	}

	if invitations.calls != 1 {
		t.Fatalf("expected invitation cleanup to run after auth error, got %d", invitations.calls)
	}
}

func TestRunCleanupJobs_NoJobs(t *testing.T) {
	s := NewScheduler(nil, nil, nil, nil)

	// Should not error when no jobs
	err := s.RunCleanupJobs()
	assert.NoError(t, err)
}

func TestRunCleanupJobs_NilRunFunc(t *testing.T) {
	s := &Scheduler{
		cleanupJobs: []CleanupJob{
			{Description: "nil job", Run: nil},
			{Description: "valid job", Run: func(_ context.Context) (int, error) { return 1, nil }},
		},
	}

	// Should skip nil Run functions without error
	err := s.RunCleanupJobs()
	assert.NoError(t, err)
}

func TestRunCleanupJobs_MultipleErrors(t *testing.T) {
	auth := &fakeAuthCleanup{
		tokenErr:     errors.New("token error"),
		passwordErr:  errors.New("password error"),
		rateLimitErr: errors.New("rate limit error"),
	}

	s := NewScheduler(nil, nil, auth, nil)

	err := s.RunCleanupJobs()

	// Should return first error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token error")

	// All jobs should still have been called
	assert.Equal(t, 1, auth.tokenCalls)
	assert.Equal(t, 1, auth.passwordCalls)
	assert.Equal(t, 1, auth.rateLimitCalls)
}

func TestRunCleanupJobs_Concurrent(t *testing.T) {
	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}

	s := NewScheduler(nil, nil, auth, nil)

	// Run cleanup jobs concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.RunCleanupJobs()
		}()
	}

	wg.Wait()

	// Each goroutine should have called all jobs
	auth.mu.Lock()
	assert.Equal(t, 5, auth.tokenCalls)
	assert.Equal(t, 5, auth.passwordCalls)
	assert.Equal(t, 5, auth.rateLimitCalls)
	auth.mu.Unlock()
}

// =============================================================================
// buildCleanupJobs Tests
// =============================================================================

func TestBuildCleanupJobs_AllServices(t *testing.T) {
	auth := &fakeAuthCleanup{}
	invitations := &fakeInvitationCleaner{}

	jobs := buildCleanupJobs(auth, invitations)

	assert.Len(t, jobs, 4)
	assert.Equal(t, "Auth token cleanup", jobs[0].Description)
	assert.Equal(t, "Password reset token cleanup", jobs[1].Description)
	assert.Equal(t, "Password reset rate limit cleanup", jobs[2].Description)
	assert.Equal(t, "Invitation cleanup", jobs[3].Description)
}

func TestBuildCleanupJobs_NoServices(t *testing.T) {
	jobs := buildCleanupJobs(nil, nil)
	assert.Empty(t, jobs)
}

func TestBuildCleanupJobs_OnlyAuth(t *testing.T) {
	auth := &fakeAuthCleanup{}

	jobs := buildCleanupJobs(auth, nil)

	assert.Len(t, jobs, 3)
}

func TestBuildCleanupJobs_OnlyInvitations(t *testing.T) {
	invitations := &fakeInvitationCleaner{}

	jobs := buildCleanupJobs(nil, invitations)

	assert.Len(t, jobs, 1)
	assert.Equal(t, "Invitation cleanup", jobs[0].Description)
}

func TestBuildCleanupJobs_JobsAreCallable(t *testing.T) {
	auth := &fakeAuthCleanup{tokenResult: 5}
	invitations := &fakeInvitationCleaner{result: 3}

	jobs := buildCleanupJobs(auth, invitations)
	ctx := context.Background()

	// All jobs should be callable
	for _, job := range jobs {
		require.NotNil(t, job.Run)
		count, err := job.Run(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	}
}

// =============================================================================
// ScheduledTask Tests
// =============================================================================

func TestScheduledTask_ConcurrentAccess(t *testing.T) {
	task := &ScheduledTask{Name: "concurrent-test"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task.mu.Lock()
			task.Running = !task.Running
			task.LastRun = time.Now()
			task.mu.Unlock()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestScheduledTask_Fields(t *testing.T) {
	now := time.Now()
	task := &ScheduledTask{
		Name:     "test-task",
		Schedule: "02:00",
		LastRun:  now,
		NextRun:  now.Add(24 * time.Hour),
		Running:  true,
	}

	assert.Equal(t, "test-task", task.Name)
	assert.Equal(t, "02:00", task.Schedule)
	assert.Equal(t, now, task.LastRun)
	assert.Equal(t, now.Add(24*time.Hour), task.NextRun)
	assert.True(t, task.Running)
}

// =============================================================================
// CleanupJob Tests
// =============================================================================

func TestCleanupJob_Fields(t *testing.T) {
	called := false
	job := CleanupJob{
		Description: "Test cleanup",
		Run: func(_ context.Context) (int, error) {
			called = true
			return 5, nil
		},
	}

	assert.Equal(t, "Test cleanup", job.Description)
	assert.NotNil(t, job.Run)

	count, err := job.Run(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.True(t, called)
}

func TestCleanupJob_RunReturnsError(t *testing.T) {
	expectedErr := errors.New("cleanup failed")
	job := CleanupJob{
		Description: "Failing cleanup",
		Run: func(_ context.Context) (int, error) {
			return 0, expectedErr
		},
	}

	count, err := job.Run(context.Background())
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// Environment Variable Tests
// =============================================================================

func TestScheduler_DisabledByEnvVars(t *testing.T) {
	// Disable all tasks via environment
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "false"))
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "false"))
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "false"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
	}()

	s := NewScheduler(nil, nil, nil, nil)
	s.Start()

	// Give tasks time to register
	time.Sleep(50 * time.Millisecond)

	// Only token-cleanup should be registered (it's always enabled)
	s.mu.RLock()
	taskCount := len(s.tasks)
	_, hasVisitCleanup := s.tasks["visit-cleanup"]
	_, hasSessionEnd := s.tasks["session-end"]
	_, hasSessionCleanup := s.tasks["session-cleanup"]
	s.mu.RUnlock()

	assert.Equal(t, 1, taskCount, "Only token-cleanup should be registered")
	assert.False(t, hasVisitCleanup, "visit-cleanup should be disabled")
	assert.False(t, hasSessionEnd, "session-end should be disabled")
	assert.False(t, hasSessionCleanup, "session-cleanup should be disabled")

	s.Stop()
}

func TestScheduler_DefaultEnvValues(t *testing.T) {
	// Clear all env vars to test defaults
	_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
	_ = os.Unsetenv("CLEANUP_SCHEDULER_TIME")
	_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
	_ = os.Unsetenv("SESSION_END_TIME")
	_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
	_ = os.Unsetenv("SESSION_CLEANUP_INTERVAL_MINUTES")
	_ = os.Unsetenv("SESSION_ABANDONED_THRESHOLD_MINUTES")

	s := NewScheduler(nil, nil, nil, nil)

	// Default values should be set
	assert.Equal(t, 0, s.sessionCleanupIntervalMinutes) // Not set until Start()
	assert.Equal(t, 0, s.sessionAbandonedThresholdMinutes)
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestAuthCleanup_InterfaceCompliance(t *testing.T) {
	// Verify fakeAuthCleanup implements AuthCleanup
	var _ AuthCleanup = &fakeAuthCleanup{}
}

func TestInvitationCleaner_InterfaceCompliance(t *testing.T) {
	// Verify fakeInvitationCleaner implements InvitationCleaner
	var _ InvitationCleaner = &fakeInvitationCleaner{}
}
