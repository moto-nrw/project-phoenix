package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	t.Parallel()

	auth := &fakeAuthCleanup{}
	invitations := &fakeInvitationCleaner{}

	s := newUnitScheduler(nil, nil, auth, invitations, nil, nil, slog.Default())

	require.NotNil(t, s)
	assert.NotNil(t, s.tasks)
	assert.NotNil(t, s.done)
	assert.Len(t, s.cleanupJobs, 4) // 3 auth + 1 invitation
}

func TestNewScheduler_NilServices(t *testing.T) {
	t.Parallel()

	s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())

	require.NotNil(t, s)
	assert.Empty(t, s.cleanupJobs)
}

func TestNewScheduler_OnlyAuthService(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{}

	s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

	require.NotNil(t, s)
	assert.Len(t, s.cleanupJobs, 3) // 3 auth jobs only
}

func TestNewScheduler_OnlyInvitationService(t *testing.T) {
	t.Parallel()

	invitations := &fakeInvitationCleaner{}

	s := newUnitScheduler(nil, nil, nil, invitations, nil, nil, slog.Default())

	require.NotNil(t, s)
	assert.Len(t, s.cleanupJobs, 1) // 1 invitation job only
}

func TestIsoWeekdayMatches(t *testing.T) {
	t.Parallel()

	monday := time.Date(2024, time.January, 1, 12, 0, 0, 0, timezone.Berlin)
	sunday := time.Date(2024, time.January, 7, 12, 0, 0, 0, timezone.Berlin)

	assert.True(t, isoWeekdayMatches(1, monday))
	assert.False(t, isoWeekdayMatches(2, monday))
	assert.True(t, isoWeekdayMatches(7, sunday))
	assert.False(t, isoWeekdayMatches(1, sunday))
}

// =============================================================================
// Start/Stop Lifecycle Tests
// =============================================================================

func TestScheduler_StartStop(t *testing.T) {
	t.Parallel()

	s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
	s.getenv = testEnv(
		"CLEANUP_SCHEDULER_ENABLED", "false",
		"SESSION_END_SCHEDULER_ENABLED", "false",
		"SESSION_CLEANUP_ENABLED", "false",
	)

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
	t.Parallel()

	s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())

	// Stop without start should not panic
	assert.NotPanics(t, func() {
		s.Stop()
	})
}

func TestScheduler_StartWithTokenCleanupOnly(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		auth := &fakeAuthCleanup{
			tokenResult:     1,
			passwordResult:  2,
			rateLimitResult: 3,
		}

		s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())
		s.getenv = testEnv(
			"CLEANUP_SCHEDULER_ENABLED", "false",
			"SESSION_END_SCHEDULER_ENABLED", "false",
			"SESSION_CLEANUP_ENABLED", "false",
		)
		s.Start()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was registered
		s.mu.RLock()
		_, hasTokenCleanup := s.tasks["token-cleanup"]
		s.mu.RUnlock()
		assert.True(t, hasTokenCleanup, "token-cleanup task should be registered")

		// Stop scheduler
		s.Stop()
	})
}

// =============================================================================
// RunCleanupJobs Tests
// =============================================================================

func TestRunCleanupJobsExecutesAllJobs(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}
	invitations := &fakeInvitationCleaner{result: 4}

	s := newUnitScheduler(nil, nil, auth, invitations, nil, nil, slog.Default())

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
	t.Parallel()

	expectedErr := errors.New("rate limit cleanup failed")

	auth := &fakeAuthCleanup{
		rateLimitErr: expectedErr,
	}
	invitations := &fakeInvitationCleaner{}

	s := newUnitScheduler(nil, nil, auth, invitations, nil, nil, slog.Default())

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
	t.Parallel()

	s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())

	// Should not error when no jobs
	err := s.RunCleanupJobs()
	assert.NoError(t, err)
}

func TestRunCleanupJobsRejectsMissingRuntime(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{}
	var outcomes []string
	s := newScheduler(WorkerDependencies{
		Logger:      slog.Default(),
		AuthCleanup: auth,
		TenantRuntimeObserver: func(entryPoint, outcome string) {
			assert.Equal(t, "worker", entryPoint)
			outcomes = append(outcomes, outcome)
		},
	})

	err := s.RunCleanupJobs()

	assert.ErrorIs(t, err, tenant.ErrRuntimeRequired)
	assert.Zero(t, auth.tokenCalls)
	assert.Zero(t, auth.passwordCalls)
	assert.Zero(t, auth.rateLimitCalls)
	assert.Equal(t, []string{"missing_tenant"}, outcomes)
}

func TestRunCleanupJobs_NilRunFunc(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupJobs: []CleanupJob{
			{Description: "nil job", Run: nil},
			{Description: "valid job", Run: func(_ context.Context) (int, error) { return 1, nil }},
		}})

	// Should skip nil Run functions without error
	err := s.RunCleanupJobs()
	assert.NoError(t, err)
}

func TestRunCleanupJobs_MultipleErrors(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{
		tokenErr:     errors.New("token error"),
		passwordErr:  errors.New("password error"),
		rateLimitErr: errors.New("rate limit error"),
	}

	s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

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
	t.Parallel()

	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}

	s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

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
	t.Parallel()

	auth := &fakeAuthCleanup{}
	invitations := &fakeInvitationCleaner{}

	jobs := buildCleanupJobs(auth, invitations, nil, nil)

	assert.Len(t, jobs, 4)
	assert.Equal(t, "Auth token cleanup", jobs[0].Description)
	assert.Equal(t, "Password reset token cleanup", jobs[1].Description)
	assert.Equal(t, "Password reset rate limit cleanup", jobs[2].Description)
	assert.Equal(t, "Invitation cleanup", jobs[3].Description)
}

func TestBuildCleanupJobs_NoServices(t *testing.T) {
	t.Parallel()

	jobs := buildCleanupJobs(nil, nil, nil, nil)
	assert.Empty(t, jobs)
}

func TestBuildCleanupJobs_OnlyAuth(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{}

	jobs := buildCleanupJobs(auth, nil, nil, nil)

	assert.Len(t, jobs, 3)
}

func TestBuildCleanupJobs_OnlyInvitations(t *testing.T) {
	t.Parallel()

	invitations := &fakeInvitationCleaner{}

	jobs := buildCleanupJobs(nil, invitations, nil, nil)

	assert.Len(t, jobs, 1)
	assert.Equal(t, "Invitation cleanup", jobs[0].Description)
}

func TestBuildCleanupJobs_JobsAreCallable(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{tokenResult: 5}
	invitations := &fakeInvitationCleaner{result: 3}

	jobs := buildCleanupJobs(auth, invitations, nil, nil)
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
	t.Parallel()
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
		s.getenv = testEnv(
			"CLEANUP_SCHEDULER_ENABLED", "false",
			"SESSION_END_SCHEDULER_ENABLED", "false",
			"SESSION_CLEANUP_ENABLED", "false",
			"STATUS_FLAG_CLEAR_ENABLED", "false",
		)
		s.Start()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

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
	})
}

func TestScheduler_DefaultEnvValues(t *testing.T) {
	t.Parallel()
	s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())

	// Default values should be set
	assert.Equal(t, 0, s.sessionCleanupIntervalMinutes) // Not set until Start()
	assert.Equal(t, 0, s.sessionAbandonedThresholdMinutes)
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestAuthCleanup_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	// Verify fakeAuthCleanup implements AuthCleanup
	var _ AuthCleanup = &fakeAuthCleanup{}
}

func TestInvitationCleaner_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	// Verify fakeInvitationCleaner implements InvitationCleaner
	var _ InvitationCleaner = &fakeInvitationCleaner{}
}

// =============================================================================
// Time Parsing and Task State Tests
// Note: Execute functions require full interface implementations which are complex
// to mock. These tests focus on the testable aspects: time parsing, task state
// management, and scheduler lifecycle.
// =============================================================================

func TestScheduleCleanupTask_InvalidTimeFormat(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "invalid")

		// Schedule cleanup task with invalid time
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Task should be registered even with invalid time (will fail silently in goroutine)
		s.mu.RLock()
		_, hasTask := s.tasks["visit-cleanup"]
		s.mu.RUnlock()
		assert.True(t, hasTask)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_InvalidHour(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "25:00")

		// Schedule cleanup task with invalid hour
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_InvalidMinute(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "02:99")

		// Schedule cleanup task with invalid minute
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_NonNumericHour(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "aa:00")

		// Schedule cleanup task with non-numeric hour
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_NonNumericMinute(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "02:bb")

		// Schedule cleanup task with non-numeric minute
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_NegativeHour(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "-1:00")

		// Schedule cleanup task with negative hour
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_NegativeMinute(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "02:-5")

		// Schedule cleanup task with negative minute
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_InvalidTimeFormat(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "invalid")

		// Schedule session end task with invalid time
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_InvalidHour(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "30:00")

		// Schedule session end task with invalid hour
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_InvalidMinute(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "18:99")

		// Schedule session end task with invalid minute
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_NonNumericHour(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "xx:00")

		// Schedule session end task with non-numeric hour
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_NonNumericMinute(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "18:yy")

		// Schedule session end task with non-numeric minute
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_NegativeHour(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "-2:00")

		// Schedule session end task with negative hour
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_NegativeMinute(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "18:-3")

		// Schedule session end task with negative minute
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

// NOTE: Session cleanup task tests are skipped because they spawn goroutines
// with a 30-second delay before execution, which makes them flaky when the test
// suite takes longer than 30 seconds. The configuration parsing logic is covered
// by the Scheduler_DefaultEnvValues test and the general scheduler lifecycle tests.
// To fully test session cleanup execution, you would need to inject mock active.Service
// interfaces which requires significant refactoring of the scheduler package.

func TestScheduleSessionCleanupTask_Disabled(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{
		tasks: make(map[string]*ScheduledTask),
		done:  make(chan struct{})})
	s.getenv = testEnv("SESSION_CLEANUP_ENABLED", "false")

	// Schedule session cleanup task (should be disabled)
	s.scheduleSessionCleanupTask()

	// Verify task was NOT created (disabled)
	s.mu.RLock()
	_, hasTask := s.tasks["session-cleanup"]
	s.mu.RUnlock()
	assert.False(t, hasTask, "Session cleanup task should not be created when disabled")
}

// =============================================================================
// Mock Active Service for Execute Tests
// =============================================================================

type mockActiveService struct {
	mu                       sync.Mutex
	endDailySessionsCalls    int
	endDailySessionsResult   *activeService.DailySessionCleanupResult
	endDailySessionsErr      error
	cleanupAbandonedCalls    int
	cleanupAbandonedResult   int
	cleanupAbandonedErr      error
	cleanupAbandonedDuration time.Duration
}

func (m *mockActiveService) GetRoomsByIDs(_ context.Context, _ []int64) ([]*facilities.Room, error) {
	return nil, nil
}

func (m *mockActiveService) GetActiveGroupVisitsWithDisplay(_ context.Context, _ int64) ([]*active.VisitWithStudentDisplay, error) {
	return nil, nil
}

func (m *mockActiveService) ConfirmDailyCheckout(_ context.Context, _, _ int64, _ string) (*activeService.DailyCheckoutResult, error) {
	return nil, nil
}

func (m *mockActiveService) HasOpenAttendanceOn(_ context.Context, _ timezone.Date) (bool, error) {
	return false, nil
}

func (m *mockActiveService) EndDailySessions(_ context.Context) (*activeService.DailySessionCleanupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endDailySessionsCalls++
	return m.endDailySessionsResult, m.endDailySessionsErr
}

func (m *mockActiveService) CleanupAbandonedSessions(_ context.Context, olderThan time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupAbandonedCalls++
	m.cleanupAbandonedDuration = olderThan
	return m.cleanupAbandonedResult, m.cleanupAbandonedErr
}

// Implement remaining Service interface methods (not used by scheduler)
func (m *mockActiveService) GetActiveGroup(_ context.Context, _ int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) CreateActiveGroup(_ context.Context, _ *active.Group) error { return nil }
func (m *mockActiveService) UpdateActiveGroup(_ context.Context, _ *active.Group) error { return nil }
func (m *mockActiveService) DeleteActiveGroup(_ context.Context, _ int64) error         { return nil }
func (m *mockActiveService) ListActiveGroups(_ context.Context, _ *base.QueryOptions) ([]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) FindActiveGroupsByRoomID(_ context.Context, _ int64) ([]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) FindDeviceActiveGroupInRoom(_ context.Context, _, _ int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) FindActiveGroupsByGroupID(_ context.Context, _ int64) ([]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) FindActiveGroupsByTimeRange(_ context.Context, _, _ time.Time) ([]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) EndActiveGroupSession(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) GetActiveGroupWithVisits(_ context.Context, _ int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) GetActiveGroupWithSupervisors(_ context.Context, _ int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) GetVisit(_ context.Context, _ int64) (*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) CreateVisit(_ context.Context, _ *active.Visit) error { return nil }
func (m *mockActiveService) UpdateVisit(_ context.Context, _ *active.Visit) error { return nil }
func (m *mockActiveService) DeleteVisit(_ context.Context, _ int64) error         { return nil }
func (m *mockActiveService) ListVisits(_ context.Context, _ *base.QueryOptions) ([]*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) FindVisitsByStudentID(_ context.Context, _ int64) ([]*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) FindVisitsByActiveGroupID(_ context.Context, _ int64) ([]*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) FindVisitsByTimeRange(_ context.Context, _, _ time.Time) ([]*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) EndVisit(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) GetStudentCurrentVisit(_ context.Context, _ int64) (*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) GetStudentsCurrentVisits(_ context.Context, _ []int64) (map[int64]*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) GetStudentCurrentVisitWithRoom(_ context.Context, _ int64) (*active.Visit, error) {
	return nil, nil
}
func (m *mockActiveService) GetGroupSupervisor(_ context.Context, _ int64) (*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) CreateGroupSupervisor(_ context.Context, _ *active.GroupSupervisor) error {
	return nil
}
func (m *mockActiveService) UpdateGroupSupervisor(_ context.Context, _ *active.GroupSupervisor) error {
	return nil
}
func (m *mockActiveService) DeleteGroupSupervisor(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) ListGroupSupervisors(_ context.Context, _ *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) FindSupervisorsByStaffID(_ context.Context, _ int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) FindSupervisorsByActiveGroupID(_ context.Context, _ int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) FindSupervisorsByActiveGroupIDs(_ context.Context, _ []int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) EndSupervision(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) GetStaffActiveSupervisions(_ context.Context, _ int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) GetAllActiveSupervisions(_ context.Context) ([]*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) GetCombinedGroup(_ context.Context, _ int64) (*active.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveService) CreateCombinedGroup(_ context.Context, _ *active.CombinedGroup) error {
	return nil
}
func (m *mockActiveService) UpdateCombinedGroup(_ context.Context, _ *active.CombinedGroup) error {
	return nil
}
func (m *mockActiveService) DeleteCombinedGroup(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) ListCombinedGroups(_ context.Context, _ *base.QueryOptions) ([]*active.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveService) FindActiveCombinedGroups(_ context.Context) ([]*active.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveService) FindCombinedGroupsByTimeRange(_ context.Context, _, _ time.Time) ([]*active.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveService) EndCombinedGroup(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) GetCombinedGroupWithGroups(_ context.Context, _ int64) (*active.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveService) CreateCombinedGroupWithGroups(_ context.Context, _ *active.CombinedGroup, _ []int64) error {
	return nil
}
func (m *mockActiveService) AddGroupToCombination(_ context.Context, _, _ int64) error { return nil }
func (m *mockActiveService) RemoveGroupFromCombination(_ context.Context, _, _ int64) error {
	return nil
}
func (m *mockActiveService) GetGroupMappingsByActiveGroupID(_ context.Context, _ int64) ([]*active.GroupMapping, error) {
	return nil, nil
}
func (m *mockActiveService) GetGroupMappingsByCombinedGroupID(_ context.Context, _ int64) ([]*active.GroupMapping, error) {
	return nil, nil
}
func (m *mockActiveService) StartActivitySession(_ context.Context, _, _, _ int64, _ *int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) StartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) CheckActivityConflict(_ context.Context, _, _ int64) (*activeService.ActivityConflictInfo, error) {
	return nil, nil
}
func (m *mockActiveService) EndActivitySession(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) ForceStartActivitySession(_ context.Context, _, _, _ int64, _ *int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) ForceStartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) GetDeviceCurrentSession(_ context.Context, _ int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) UpdateActiveGroupSupervisors(_ context.Context, _ int64, _ []int64) (*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) ProcessSessionTimeout(_ context.Context, _ int64) (*activeService.TimeoutResult, error) {
	return nil, nil
}
func (m *mockActiveService) UpdateSessionActivity(_ context.Context, _ int64) error { return nil }
func (m *mockActiveService) ValidateSessionTimeout(_ context.Context, _ int64, _ int) error {
	return nil
}
func (m *mockActiveService) GetSessionTimeoutInfo(_ context.Context, _ int64) (*activeService.SessionTimeoutInfo, error) {
	return nil, nil
}
func (m *mockActiveService) CountActiveVisitsByRoomID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockActiveService) CountActiveVisitsByActiveGroupID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockActiveService) ListStudentsPresentInRoom(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}
func (m *mockActiveService) ListStudentsInTransit(_ context.Context) ([]int64, error) {
	return nil, nil
}
func (m *mockActiveService) ListStudentsPresentToday(_ context.Context) ([]int64, error) {
	return nil, nil
}
func (m *mockActiveService) AssignTransitStudentsToActiveGroup(_ context.Context, _ []int64, _ int64) (*activeService.TransitAssignResult, error) {
	return nil, nil
}
func (m *mockActiveService) MoveStudentsToActiveGroup(_ context.Context, _ []int64, _ int64) (*activeService.StudentMoveResult, error) {
	return nil, nil
}
func (m *mockActiveService) MoveStudentsToActiveGroupAuthorized(_ context.Context, _ []int64, _ int64, _ activeService.StudentMoveAuthorization) (*activeService.StudentMoveResult, error) {
	return nil, nil
}
func (m *mockActiveService) MoveStudentsToTransit(_ context.Context, _ []int64) (*activeService.StudentMoveResult, error) {
	return nil, nil
}
func (m *mockActiveService) MoveStudentsToTransitAuthorized(_ context.Context, _ []int64, _ activeService.StudentMoveAuthorization) (*activeService.StudentMoveResult, error) {
	return nil, nil
}
func (m *mockActiveService) GetDashboardAnalytics(_ context.Context) (*activeService.DashboardAnalytics, error) {
	return nil, nil
}
func (m *mockActiveService) GetActiveGroupsByIDs(_ context.Context, _ []int64) (map[int64]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) GetStudentAttendanceStatus(_ context.Context, _ int64) (*activeService.AttendanceStatus, error) {
	return nil, nil
}
func (m *mockActiveService) GetStudentsAttendanceStatuses(_ context.Context, _ []int64) (map[int64]*activeService.AttendanceStatus, error) {
	return nil, nil
}
func (m *mockActiveService) ToggleStudentAttendance(_ context.Context, _, _, _ int64, _ bool) (*activeService.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveService) CheckInStudent(_ context.Context, _, _, _ int64, _ bool) (*activeService.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveService) CheckOutStudent(_ context.Context, _, _ int64, _ bool) (*activeService.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveService) CheckOutStudentFromDevice(_ context.Context, _, _ int64) (*activeService.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveService) ProcessSchoolCheckinBatch(_ context.Context, _ []int64, _ int64, _ string) (*activeService.SchoolCheckinBatchResult, error) {
	return nil, nil
}
func (m *mockActiveService) CheckTeacherStudentAccess(_ context.Context, _, _ int64) (bool, error) {
	return false, nil
}
func (m *mockActiveService) GetUnclaimedActiveGroups(_ context.Context) ([]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) ClaimActiveGroup(_ context.Context, _, _ int64, _ string) (*active.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveService) GetCrossTenantStudents(_ context.Context, _ int64) ([]active.CrossTenantStudent, error) {
	return nil, nil
}
func (m *mockActiveService) GetTrackingIndicators(_ context.Context, _ []int64, _ []string) (map[int64][]bool, error) {
	return nil, nil
}
func (m *mockActiveService) SetSettingsService(_ activeService.SettingsResolver) {}
func (m *mockActiveService) GetPresenceMode(_ context.Context) string            { return "detailed" }

// =============================================================================
// Mock Cleanup Service for Execute Tests
// =============================================================================

type mockCleanupService struct {
	mu                     sync.Mutex
	cleanupCalls           int
	cleanupResult          *activeService.CleanupResult
	cleanupErr             error
	studentCalls           int
	studentErr             error
	retentionCalls         int
	retentionErr           error
	previewCalls           int
	previewErr             error
	attendanceCalls        int
	attendanceResult       *activeService.AttendanceCleanupResult
	attendanceErr          error
	attendancePreviewCalls int
	attendancePreviewErr   error
}

func (m *mockCleanupService) CleanupExpiredVisits(_ context.Context) (*activeService.CleanupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupCalls++
	return m.cleanupResult, m.cleanupErr
}

func (m *mockCleanupService) CleanupVisitsForStudent(_ context.Context, _ int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.studentCalls++
	return 0, m.studentErr
}

func (m *mockCleanupService) GetRetentionStatistics(_ context.Context) (*activeService.RetentionStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retentionCalls++
	return nil, m.retentionErr
}

func (m *mockCleanupService) PreviewCleanup(_ context.Context) (*activeService.CleanupPreview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.previewCalls++
	return nil, m.previewErr
}

func (m *mockCleanupService) CleanupStaleAttendance(_ context.Context) (*activeService.AttendanceCleanupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attendanceCalls++
	return m.attendanceResult, m.attendanceErr
}

func (m *mockCleanupService) PreviewAttendanceCleanup(_ context.Context) (*activeService.AttendanceCleanupPreview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attendancePreviewCalls++
	return nil, m.attendancePreviewErr
}

func (m *mockCleanupService) CleanupStaleSupervisors(_ context.Context) (*activeService.SupervisorCleanupResult, error) {
	return &activeService.SupervisorCleanupResult{Success: true}, nil
}

func (m *mockCleanupService) PreviewSupervisorCleanup(_ context.Context) (*activeService.SupervisorCleanupPreview, error) {
	return &activeService.SupervisorCleanupPreview{}, nil
}

// =============================================================================
// Execute Tests
// =============================================================================

func TestExecuteSessionEndForTenant_Success(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded:    5,
			VisitsEnded:      20,
			SupervisorsEnded: 3,
			Success:          true,
		},
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	ok, err := s.executeSessionEndForTenant(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, ok, "session end should report success")

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEndForTenant_Error(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{
		endDailySessionsErr: errors.New("session end failed"),
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	// Session-end errors are swallowed (other tenants must still run) but
	// reported as ok=false so the day-mark is not set.
	ok, err := s.executeSessionEndForTenant(context.Background(), 0)
	require.NoError(t, err)
	assert.False(t, ok, "failed session end must not report success")

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEndForTenant_WithErrors(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded:    5,
			VisitsEnded:      20,
			SupervisorsEnded: 3,
			Success:          true,
			Errors:           []string{"error 1", "error 2"},
		},
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	ok, err := s.executeSessionEndForTenant(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, ok)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEndForTenant_WithManyErrors(t *testing.T) {
	t.Parallel()

	var errors []string
	for i := 0; i < 15; i++ {
		errors = append(errors, "error")
	}

	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded: 5,
			Success:       true,
			Errors:        errors,
		},
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	ok, err := s.executeSessionEndForTenant(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, ok)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestCheckAndRunSessionEnd_AlreadyRunning(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	task := &ScheduledTask{Name: "test-session-end", Running: true}

	// Check session end (should skip because already running)
	s.checkAndRunSessionEnd(context.Background(), task)

	// Verify service was NOT called
	activeSvc.mu.Lock()
	assert.Equal(t, 0, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEndForTenant_CustomTimeout(t *testing.T) {
	t.Parallel()
	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded: 5,
			Success:       true,
		},
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})
	s.getenv = testEnv("SESSION_END_TIMEOUT_MINUTES", "30")

	ok, err := s.executeSessionEndForTenant(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, ok)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteTokenCleanup_Success(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{
		tokenResult:     5,
		passwordResult:  3,
		rateLimitResult: 2,
	}

	s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

	task := &ScheduledTask{Name: "token-cleanup"}

	// Execute token cleanup
	s.executeTokenCleanup(context.Background(), task)

	// Verify all cleanup jobs were called
	auth.mu.Lock()
	assert.Equal(t, 1, auth.tokenCalls)
	assert.Equal(t, 1, auth.passwordCalls)
	assert.Equal(t, 1, auth.rateLimitCalls)
	auth.mu.Unlock()

	// Verify task state
	task.mu.Lock()
	assert.False(t, task.Running, "Task should not be running after completion")
	task.mu.Unlock()
}

func TestExecuteTokenCleanup_AlreadyRunning(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{}

	s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

	task := &ScheduledTask{Name: "token-cleanup", Running: true}

	// Execute token cleanup (should skip because already running)
	s.executeTokenCleanup(context.Background(), task)

	// Verify no cleanup jobs were called
	auth.mu.Lock()
	assert.Equal(t, 0, auth.tokenCalls)
	auth.mu.Unlock()
}

func TestExecuteTokenCleanup_Error(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{
		tokenErr: errors.New("token cleanup failed"),
	}

	s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

	task := &ScheduledTask{Name: "token-cleanup"}

	// Execute token cleanup (should handle error gracefully)
	s.executeTokenCleanup(context.Background(), task)

	// Verify cleanup was attempted
	auth.mu.Lock()
	assert.Equal(t, 1, auth.tokenCalls)
	auth.mu.Unlock()
}

func TestCheckAndRunSessionCleanup_Success(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{
		cleanupAbandonedResult: 5,
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	task := &ScheduledTask{Name: "session-cleanup"}

	// Run session cleanup (threshold falls back to the 60-minute default)
	s.checkAndRunSessionCleanup(context.Background(), task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.cleanupAbandonedCalls)
	assert.Equal(t, 60*time.Minute, activeSvc.cleanupAbandonedDuration)
	activeSvc.mu.Unlock()

	// Verify task state
	task.mu.Lock()
	assert.False(t, task.Running, "Task should not be running after completion")
	task.mu.Unlock()
}

func TestCheckAndRunSessionCleanup_NoAbandoned(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{
		cleanupAbandonedResult: 0,
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})
	s.getenv = testEnv("SESSION_ABANDONED_THRESHOLD_MINUTES", "30")

	task := &ScheduledTask{Name: "session-cleanup"}

	// Run session cleanup
	s.checkAndRunSessionCleanup(context.Background(), task)

	// Verify service was called with the env-configured threshold
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.cleanupAbandonedCalls)
	assert.Equal(t, 30*time.Minute, activeSvc.cleanupAbandonedDuration)
	activeSvc.mu.Unlock()
}

func TestCheckAndRunSessionCleanup_Error(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{
		cleanupAbandonedErr: errors.New("cleanup failed"),
	}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	task := &ScheduledTask{Name: "session-cleanup"}

	// Run session cleanup (should handle error gracefully)
	s.checkAndRunSessionCleanup(context.Background(), task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.cleanupAbandonedCalls)
	activeSvc.mu.Unlock()
}

func TestCheckAndRunSessionCleanup_AlreadyRunning(t *testing.T) {
	t.Parallel()

	activeSvc := &mockActiveService{}

	s := unitScheduler(&Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{})})

	task := &ScheduledTask{Name: "session-cleanup", Running: true}

	// Run session cleanup (should skip because already running)
	s.checkAndRunSessionCleanup(context.Background(), task)

	// Verify service was NOT called
	activeSvc.mu.Lock()
	assert.Equal(t, 0, activeSvc.cleanupAbandonedCalls)
	activeSvc.mu.Unlock()
}

// =============================================================================
// Configuration Parsing Tests
// =============================================================================

func TestScheduleSessionCleanupTask_CustomInterval(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			activeService: &mockActiveService{},
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})
		s.getenv = testEnv("SESSION_CLEANUP_ENABLED", "true", "SESSION_CLEANUP_INTERVAL_MINUTES", "30")

		// Schedule session cleanup task
		s.scheduleSessionCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify config was parsed
		assert.Equal(t, 30, s.sessionCleanupIntervalMinutes)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionCleanupTask_CustomThreshold(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			activeService: &mockActiveService{},
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})
		s.getenv = testEnv("SESSION_CLEANUP_ENABLED", "true", "SESSION_ABANDONED_THRESHOLD_MINUTES", "120")

		// Schedule session cleanup task
		s.scheduleSessionCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify config was parsed
		assert.Equal(t, 120, s.sessionAbandonedThresholdMinutes)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionCleanupTask_InvalidInterval(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			activeService: &mockActiveService{},
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})
		s.getenv = testEnv("SESSION_CLEANUP_ENABLED", "true", "SESSION_CLEANUP_INTERVAL_MINUTES", "invalid")

		// Schedule session cleanup task
		s.scheduleSessionCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify default was used (invalid config should fallback to default)
		assert.Equal(t, 15, s.sessionCleanupIntervalMinutes)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionCleanupTask_NegativeInterval(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			activeService: &mockActiveService{},
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})
		s.getenv = testEnv("SESSION_CLEANUP_ENABLED", "true", "SESSION_CLEANUP_INTERVAL_MINUTES", "-5")

		// Schedule session cleanup task
		s.scheduleSessionCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify default was used (negative should fallback to default)
		assert.Equal(t, 15, s.sessionCleanupIntervalMinutes)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleCleanupTask_CustomTime(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "03:30")

		// Schedule cleanup task
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was created with custom schedule
		s.mu.RLock()
		task, hasTask := s.tasks["visit-cleanup"]
		s.mu.RUnlock()
		assert.True(t, hasTask)
		// With per-tenant minute-polling, the schedule is now descriptive (not HH:MM)
		assert.Equal(t, "1m-poll", task.Schedule)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_CustomTime(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})
		s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "true", "SESSION_END_TIME", "17:00")

		// Schedule session end task
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was created with custom schedule
		s.mu.RLock()
		task, hasTask := s.tasks["session-end"]
		s.mu.RUnlock()
		assert.True(t, hasTask)
		// With per-tenant minute-polling, the schedule is now descriptive (not HH:MM)
		assert.Equal(t, "1m-poll", task.Schedule)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_DefaultEnabled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{})})

		// Schedule session end task (should be enabled by default)
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was created
		s.mu.RLock()
		_, hasTask := s.tasks["session-end"]
		s.mu.RUnlock()
		assert.True(t, hasTask, "Session end should be enabled by default")

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionCleanupTask_DefaultEnabled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			activeService: &mockActiveService{},
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})

		// Schedule session cleanup task (should be enabled by default)
		s.scheduleSessionCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was created
		s.mu.RLock()
		_, hasTask := s.tasks["session-cleanup"]
		s.mu.RUnlock()
		assert.True(t, hasTask, "Session cleanup should be enabled by default")

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

// =============================================================================
// Interface Verification Tests
// =============================================================================

func TestMockActiveService_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ activeService.Service = &mockActiveService{}
}

func TestMockCleanupService_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ activeService.CleanupService = &mockCleanupService{}
}

// =============================================================================
// Goroutine Run Loop Tests (synctest)
// =============================================================================

func TestRunCleanupTask_DefaultScheduleTime(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		cleanupSvc := &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{
				StudentsProcessed: 5,
				RecordsDeleted:    25,
				Success:           true,
			},
		}

		s := unitScheduler(&Scheduler{
			cleanupService: cleanupSvc,
			tasks:          make(map[string]*ScheduledTask),
			done:           make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true")

		// Schedule cleanup task (should use default "02:00")
		s.scheduleCleanupTask()

		// Verify task was created with default schedule
		s.mu.RLock()
		task, hasTask := s.tasks["visit-cleanup"]
		s.mu.RUnlock()
		assert.True(t, hasTask)
		// With per-tenant minute-polling, the schedule is now descriptive (not HH:MM)
		assert.Equal(t, "1m-poll", task.Schedule, "Should use polling schedule for per-tenant support")

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestRunCleanupTask_ExecutesOnSchedule(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		cleanupSvc := &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{
				StudentsProcessed: 10,
				RecordsDeleted:    100,
				Success:           true,
			},
		}

		s := unitScheduler(&Scheduler{
			cleanupService: cleanupSvc,
			tasks:          make(map[string]*ScheduledTask),
			done:           make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "02:00")

		// Schedule cleanup task (spawns goroutine)
		// Task is scheduled for 02:00, and synctest starts at 01:00:00
		s.scheduleCleanupTask()

		// Sleep for more than 1 hour to trigger the scheduled cleanup
		// In synctest, this will advance fake time past 02:00
		time.Sleep(2 * time.Hour)

		// Wait for goroutines to process the scheduled task
		synctest.Wait()

		// Verify cleanup was called at least once
		cleanupSvc.mu.Lock()
		assert.GreaterOrEqual(t, cleanupSvc.cleanupCalls, 1, "Cleanup should execute after scheduled time")
		cleanupSvc.mu.Unlock()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestRunCleanupTask_StopsOnDone(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		cleanupSvc := &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{Success: true},
		}

		s := unitScheduler(&Scheduler{
			cleanupService: cleanupSvc,
			tasks:          make(map[string]*ScheduledTask),
			done:           make(chan struct{})})
		s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "true", "CLEANUP_SCHEDULER_TIME", "02:00")

		// Schedule cleanup task
		s.scheduleCleanupTask()

		// Close done channel before scheduled time fires
		close(s.done)

		// Wait for goroutine to exit
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - goroutine exited cleanly
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Goroutine did not exit after done channel closed")
		}
	})
}

func TestRunSessionEndTask_ExecutesOnSchedule(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		activeSvc := &mockActiveService{
			endDailySessionsResult: &activeService.DailySessionCleanupResult{
				SessionsEnded:    5,
				VisitsEnded:      20,
				SupervisorsEnded: 3,
				Success:          true,
			},
		}

		s := unitScheduler(&Scheduler{
			activeService: activeSvc,
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})
		s.getenv = testEnv("SESSION_END_TIME", "18:00")

		// Schedule session end task
		// Task is scheduled for 18:00, and synctest starts at 01:00:00
		s.scheduleSessionEndTask()

		// Sleep for more than 17 hours to trigger the scheduled session end
		// In synctest, this will advance fake time past 18:00
		time.Sleep(18 * time.Hour)

		// Wait for goroutines to process the scheduled task
		synctest.Wait()

		// Verify service was called at least once
		activeSvc.mu.Lock()
		assert.GreaterOrEqual(t, activeSvc.endDailySessionsCalls, 1, "Session end should execute after scheduled time")
		activeSvc.mu.Unlock()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestRunSessionEndTask_StopsOnDone(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		activeSvc := &mockActiveService{
			endDailySessionsResult: &activeService.DailySessionCleanupResult{Success: true},
		}

		s := unitScheduler(&Scheduler{
			activeService: activeSvc,
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})
		s.getenv = testEnv("SESSION_END_TIME", "18:00")

		// Schedule session end task
		s.scheduleSessionEndTask()

		// Close done channel before scheduled time fires
		close(s.done)

		// Wait for goroutine to exit
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - goroutine exited cleanly
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Goroutine did not exit after done channel closed")
		}
	})
}

func TestRunSessionCleanupTask_ExecutesAfterDelay(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		activeSvc := &mockActiveService{
			cleanupAbandonedResult: 5,
		}

		s := unitScheduler(&Scheduler{
			activeService: activeSvc,
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})

		// Schedule session cleanup task (has 30-second initial delay)
		s.scheduleSessionCleanupTask()

		// Sleep for more than 30 seconds to trigger the initial cleanup
		// In synctest, this will advance fake time past the 30-second delay
		time.Sleep(1 * time.Minute)

		// Verify cleanup was called (should execute after 30s delay)
		activeSvc.mu.Lock()
		assert.GreaterOrEqual(t, activeSvc.cleanupAbandonedCalls, 1, "Session cleanup should execute after initial delay")
		activeSvc.mu.Unlock()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestRunSessionCleanupTask_StopsDuringStartupDelay(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{done: make(chan struct{})}
		s.wg.Add(1)
		go s.runSessionCleanupTaskPolling(&ScheduledTask{Name: "session-cleanup"})

		synctest.Wait()
		close(s.done)

		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("session cleanup did not stop during startup delay")
		}
	})
}

func TestRunTokenCleanupTask_TickerRepeat(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		auth := &fakeAuthCleanup{
			tokenResult:     1,
			passwordResult:  2,
			rateLimitResult: 3,
		}

		s := newUnitScheduler(nil, nil, auth, nil, nil, nil, slog.Default())

		// Schedule token cleanup task (runs immediately, then every hour)
		s.scheduleTokenCleanupTask()

		// Small sleep to allow initial execution to complete
		time.Sleep(100 * time.Millisecond)

		// Verify first call happened (runs immediately on startup)
		auth.mu.Lock()
		firstCallCount := auth.tokenCalls
		auth.mu.Unlock()
		assert.GreaterOrEqual(t, firstCallCount, 1, "Token cleanup should run immediately")

		// Sleep for more than 1 hour to trigger ticker
		// In synctest, this will advance fake time and fire the ticker
		time.Sleep(2 * time.Hour)

		// Yield to let the token cleanup goroutine execute after its ticker fires
		time.Sleep(1 * time.Second)

		// Verify second call happened
		auth.mu.Lock()
		secondCallCount := auth.tokenCalls
		auth.mu.Unlock()
		assert.GreaterOrEqual(t, secondCallCount, 2, "Token cleanup should repeat after ticker interval")

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestRunSessionCleanupTask_StopsOnDoneAfterSleep(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		activeSvc := &mockActiveService{
			cleanupAbandonedResult: 3,
		}

		s := unitScheduler(&Scheduler{
			activeService: activeSvc,
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{})})

		// Schedule session cleanup task (has 30-second initial delay, then runs every 15 min by default)
		s.scheduleSessionCleanupTask()

		// Wait for initial sleep (30s) to complete and first cleanup to execute
		// In synctest, this advances fake time past the initial delay
		time.Sleep(35 * time.Second)

		// Verify first cleanup executed
		activeSvc.mu.Lock()
		firstCallCount := activeSvc.cleanupAbandonedCalls
		activeSvc.mu.Unlock()
		assert.GreaterOrEqual(t, firstCallCount, 1, "Initial cleanup should execute after 30s delay")

		// Now close done channel to trigger the ticker select's done case
		close(s.done)

		// Wait for goroutine to exit
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - goroutine exited cleanly via ticker's done case
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Goroutine did not exit after done channel closed")
		}
	})
}

type mockBreakAutoEnder struct {
	calls int
}

func (m *mockBreakAutoEnder) AutoEndExpiredBreaks(_ context.Context) (int, error) {
	m.calls++
	return 0, nil
}

func TestWaitUntilNextMinute_ShutdownDuringWait(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{done: make(chan struct{}), logger: slog.Default()})
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(s.done)
		}()

		result := s.waitUntilNextMinute()
		assert.False(t, result, "should return false when shutdown signal fires during wait")
	})
}

func TestScheduleCleanupTask_DisabledByEnv(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{
		done:   make(chan struct{}),
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask)})
	s.getenv = testEnv("CLEANUP_SCHEDULER_ENABLED", "false")

	s.scheduleCleanupTask()
	assert.Empty(t, s.tasks, "cleanup task should not be registered when disabled")
}

func TestScheduleSessionEndTask_DisabledByEnv(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{
		done:   make(chan struct{}),
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask)})
	s.getenv = testEnv("SESSION_END_SCHEDULER_ENABLED", "false")

	s.scheduleSessionEndTask()
	assert.Empty(t, s.tasks, "session end task should not be registered when disabled")
}

func TestExecuteCleanupForTenant_ReturnsFalseOnError(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupErr: errors.New("db error"),
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	assert.False(t, result)
}

func TestExecuteCleanupForTenant_ReturnsTrueOnSuccess(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{},
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	assert.True(t, result)
}

func TestExecuteCleanupForTenant_ReturnsFalseOnFeedbackFailure(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("feedback database unavailable")
	s := unitScheduler(&Scheduler{
		cleanupService:  &mockCleanupService{cleanupResult: &activeService.CleanupResult{}},
		feedbackCleaner: &fakeFeedbackCleaner{err: wantErr},
		logger:          slog.Default(),
	})

	assert.False(t, s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t)))
}

func TestExecuteCleanupForTenant_AttendanceError(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{},
			attendanceErr: errors.New("attendance db error"),
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	// Returns true because primary cleanup (expired visits) succeeded
	assert.True(t, result)
}

func TestExecuteCleanupForTenant_AttendancePartialFailure(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{},
			attendanceResult: &activeService.AttendanceCleanupResult{
				Success:       false,
				RecordsClosed: 3,
				Errors:        []string{"record 1 failed", "record 2 failed"},
			},
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	assert.True(t, result)
}

func TestExecuteCleanupForTenant_AttendanceSuccess(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{},
			attendanceResult: &activeService.AttendanceCleanupResult{
				Success:          true,
				RecordsClosed:    5,
				StudentsAffected: 3,
			},
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	assert.True(t, result)
}

func TestExecuteCleanupForTenant_AttendanceNoRecords(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupResult: &activeService.CleanupResult{},
			attendanceResult: &activeService.AttendanceCleanupResult{
				Success:       true,
				RecordsClosed: 0,
			},
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	assert.True(t, result)
}

func TestExecuteCleanupForTenant_AttendanceNilResult(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		cleanupService: &mockCleanupService{
			cleanupResult:    &activeService.CleanupResult{},
			attendanceResult: nil,
		},
		logger: slog.Default()})

	result := s.executeCleanupForTenant(context.Background(), testpkg.Tenant(t))
	assert.True(t, result)
}

func TestTimeMatchesNow(t *testing.T) {
	t.Parallel()

	now := time.Now()
	nowStr := now.Format("15:04")
	assert.True(t, timeMatchesNow(nowStr))
	assert.False(t, timeMatchesNow("99:99"))
	assert.False(t, timeMatchesNow("invalid"))
	assert.False(t, timeMatchesNow("25:00"))
	assert.False(t, timeMatchesNow("12:60"))
}

func TestWasRunToday_NotRun(t *testing.T) {
	t.Parallel()

	var m sync.Map
	assert.False(t, wasRunToday(&m, 1))
}

func TestWasRunToday_RanToday(t *testing.T) {
	t.Parallel()

	var m sync.Map
	m.Store(int64(1), time.Now())
	assert.True(t, wasRunToday(&m, 1))
}

func TestWasRunToday_RanYesterday(t *testing.T) {
	t.Parallel()

	var m sync.Map
	tenantID := testpkg.UniqueTestTenantID(t)
	m.Store(tenantID, time.Now().Add(-25*time.Hour))
	assert.False(t, wasRunToday(&m, tenantID))
}

func TestWasRunToday_InvalidType(t *testing.T) {
	t.Parallel()

	var m sync.Map
	tenantID := testpkg.UniqueTestTenantID(t)
	m.Store(tenantID, "not a time")
	assert.False(t, wasRunToday(&m, tenantID))
}

func TestWasRunToday_DifferentTenant(t *testing.T) {
	t.Parallel()

	var m sync.Map
	m.Store(int64(1), time.Now())
	assert.False(t, wasRunToday(&m, 2))
}

func TestResolveStringSetting_NoSettings(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{logger: slog.Default()})
	val := s.resolveStringSetting(context.Background(), "key", "NONEXISTENT_ENV", "fallback")
	assert.Equal(t, "fallback", val)
}

func TestResolveBoolSetting_NoSettings(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{logger: slog.Default()})
	val := s.resolveBoolSetting(context.Background(), "key", "NONEXISTENT_ENV", true)
	assert.True(t, val)
}

func TestResolveIntSetting_NoSettings(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{logger: slog.Default()})
	val := s.resolveIntSetting(context.Background(), "key", "NONEXISTENT_ENV", 42)
	assert.Equal(t, 42, val)
}

func TestResolveStringSetting_FromEnv(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.getenv = testEnv("TEST_RESOLVE_STR", "from_env")
	val := s.resolveStringSetting(context.Background(), "key", "TEST_RESOLVE_STR", "default")
	assert.Equal(t, "from_env", val)
}

func TestResolveBoolSetting_FromEnv(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.getenv = testEnv("TEST_RESOLVE_BOOL", "true")
	val := s.resolveBoolSetting(context.Background(), "key", "TEST_RESOLVE_BOOL", false)
	assert.True(t, val)
}

func TestResolveIntSetting_FromEnv(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.getenv = testEnv("TEST_RESOLVE_INT", "99")
	val := s.resolveIntSetting(context.Background(), "key", "TEST_RESOLVE_INT", 10)
	assert.Equal(t, 99, val)
}

func TestResolveIntSetting_InvalidEnv(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.getenv = testEnv("TEST_RESOLVE_INT_BAD", "notanumber")
	val := s.resolveIntSetting(context.Background(), "key", "TEST_RESOLVE_INT_BAD", 10)
	assert.Equal(t, 10, val)
}

func TestScheduleBreakAutoEndTask_NilBreakAutoEnder(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		done:   make(chan struct{}),
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask)})

	s.scheduleBreakAutoEndTask()
	assert.Empty(t, s.tasks, "should not register task without break auto-ender")
}

func TestScheduleBreakAutoEndTask_CustomInterval(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{
		done:           make(chan struct{}),
		logger:         slog.Default(),
		tasks:          make(map[string]*ScheduledTask),
		wg:             sync.WaitGroup{},
		breakAutoEnder: &mockBreakAutoEnder{}})
	s.getenv = testEnv("BREAK_AUTO_END_INTERVAL_SECONDS", "30")

	s.scheduleBreakAutoEndTask()
	defer close(s.done)
	assert.Equal(t, 30, s.breakAutoEndIntervalSeconds)
}

// =============================================================================
// buildCleanupJobs with EmailChangeTokenCleaner tests
// =============================================================================

type fakeEmailChangeCleaner struct {
	result int
	err    error
	called bool
}

func (f *fakeEmailChangeCleaner) CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error) {
	f.called = true
	return f.result, f.err
}

func TestEmailChangeTokenCleaner_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	var _ EmailChangeTokenCleaner = &fakeEmailChangeCleaner{}
}

func TestBuildCleanupJobs_WithEmailChangeCleaner(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthCleanup{}
	invitations := &fakeInvitationCleaner{}
	cleaner := &fakeEmailChangeCleaner{result: 7}

	jobs := buildCleanupJobs(auth, invitations, cleaner, nil)

	assert.Len(t, jobs, 5)
	assert.Equal(t, "Email change token cleanup", jobs[4].Description)

	count, err := jobs[4].Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 7, count)
	assert.True(t, cleaner.called)
}

func TestBuildCleanupJobs_EmailChangeCleanerPropagatesError(t *testing.T) {
	t.Parallel()

	cleaner := &fakeEmailChangeCleaner{err: fmt.Errorf("cleanup failed")}

	jobs := buildCleanupJobs(nil, nil, cleaner, nil)

	assert.Len(t, jobs, 1)
	_, err := jobs[0].Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cleanup failed")
}

// =============================================================================
// Timetable Materialization Tests (WP-B8)
// =============================================================================

// fakeMaterializer is a test double for scheduleSvc.MaterializationService.
// Call counts + recorded arguments let tests assert cadence and window math
// without depending on the real service's repositories.
type fakeMaterializer struct {
	mu               sync.Mutex
	materializeCalls int
	resolveCalls     int
	lastFrom         timezone.Date
	lastTo           timezone.Date
	lastWeeksAhead   int
	lastSource       scheduleSvc.MaterializationSource
	returnErr        error
	returnResult     *scheduleSvc.MaterializationResult
}

func (f *fakeMaterializer) MaterializeForTenant(_ context.Context, from, to timezone.Date, source scheduleSvc.MaterializationSource) (*scheduleSvc.MaterializationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.materializeCalls++
	f.lastFrom = from
	f.lastTo = to
	f.lastSource = source
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	if f.returnResult != nil {
		return f.returnResult, nil
	}
	return &scheduleSvc.MaterializationResult{From: from, To: to}, nil
}

func (f *fakeMaterializer) ResolveWindow(baseDate timezone.Date, weeksAhead int) (timezone.Date, timezone.Date) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolveCalls++
	f.lastWeeksAhead = weeksAhead
	// Return a deterministic window so the caller has valid dates to log.
	from := baseDate.AddDays(7)
	to := from.AddDays(weeksAhead*7 - 1)
	return from, to
}

func (f *fakeMaterializer) DetectEditedInWindow(_ context.Context, _ int64, _, _ timezone.Date, _ bool) ([]scheduleSvc.EditedOccurrence, error) {
	return nil, nil
}

// fakeSettingsResolver implements SettingsResolver for deterministic tests.
// Tests populate boolValues/intValues/stringValues with the keys they care
// about; anything else returns the zero value with HasTenantOverride=false so
// resolveXSetting falls through to the default.
type fakeSettingsResolver struct {
	mu           sync.Mutex
	boolValues   map[string]bool
	intValues    map[string]int
	stringValues map[string]string
	// If set, HasTenantOverride returns this error instead of examining the maps.
	overrideErr error
}

func (f *fakeSettingsResolver) HasTenantOverride(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.overrideErr != nil {
		return false, f.overrideErr
	}
	if _, ok := f.boolValues[key]; ok {
		return true, nil
	}
	if _, ok := f.intValues[key]; ok {
		return true, nil
	}
	if _, ok := f.stringValues[key]; ok {
		return true, nil
	}
	return false, nil
}

func (f *fakeSettingsResolver) ResolveBool(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.boolValues[key]; ok {
		return v, nil
	}
	return false, fmt.Errorf("no value for key %s", key)
}

func (f *fakeSettingsResolver) ResolveInt(_ context.Context, key string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.intValues[key]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("no value for key %s", key)
}

func (f *fakeSettingsResolver) ResolveString(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.stringValues[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("no value for key %s", key)
}

const materializationTestWeekday = 1

func fixedMaterializationTime() time.Time {
	return time.Date(2024, time.January, 1, 12, 0, 0, 0, timezone.Berlin)
}

func TestScheduleMaterializationTask_NilMaterializer(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		done:   make(chan struct{}),
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask)})

	s.scheduleMaterializationTask()
	assert.Empty(t, s.tasks, "no task should register without a materializer")
}

func TestScheduleMaterializationTask_RegistersTask(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		done:         make(chan struct{}),
		logger:       slog.Default(),
		tasks:        make(map[string]*ScheduledTask),
		materializer: &fakeMaterializer{}})

	// Pre-close done so the spawned goroutine exits promptly after the
	// startup check and waitUntilNextMinute returns false.
	close(s.done)

	s.scheduleMaterializationTask()
	s.wg.Wait()

	s.mu.RLock()
	_, hasTask := s.tasks["timetable-materialization"]
	s.mu.RUnlock()
	assert.True(t, hasTask, "materialization task should be registered")
}

func TestCheckAndRunMaterialization_AlreadyRunning(t *testing.T) {
	t.Parallel()

	m := &fakeMaterializer{}
	s := unitScheduler(&Scheduler{
		logger:       slog.Default(),
		materializer: m})

	task := &ScheduledTask{Name: "test", Running: true}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 0, m.materializeCalls, "must skip work when task already running")
}

func TestCheckAndRunMaterialization_EnabledByDefault(t *testing.T) {
	t.Parallel()

	// With no tenant override on the enabled key, resolveBoolSetting returns
	// the defaultVal (true) — materializer runs on the configured weekday.
	m := &fakeMaterializer{}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday: materializationTestWeekday,
			},
		}})

	task := &ScheduledTask{Name: "test"}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 1, m.materializeCalls, "default-enabled tenant must invoke materializer")
	// Task running flag must be cleared via the deferred unlock.
	task.mu.Lock()
	assert.False(t, task.Running, "Running flag must be cleared after run")
	task.mu.Unlock()
}

func TestCheckAndRunMaterialization_EnabledWrongWeekday(t *testing.T) {
	t.Parallel()

	m := &fakeMaterializer{}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyTimetableMaterializationEnabled: true,
			},
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday: materializationTestWeekday + 1,
			},
		}})

	task := &ScheduledTask{Name: "test"}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 0, m.materializeCalls, "wrong weekday must skip materialization")
}

func TestCheckAndRunMaterialization_WasRunToday(t *testing.T) {
	t.Parallel()

	m := &fakeMaterializer{}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyTimetableMaterializationEnabled: true,
			},
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday: materializationTestWeekday,
			},
		}})

	// Seed lastMaterialization with today's timestamp — simulates a prior run.
	// Unit scheduler composition supplies one validated tenant.
	s.lastMaterialization.Store(schedulerUnitTenantID, fixedMaterializationTime())
	task := &ScheduledTask{Name: "test"}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 0, m.materializeCalls, "must skip when already ran today")
}

func TestCheckAndRunMaterialization_HappyPath(t *testing.T) {
	t.Parallel()

	m := &fakeMaterializer{
		returnResult: &scheduleSvc.MaterializationResult{
			InstancesCreated: 7,
			CandidatesRaced:  2,
			DurationMS:       123,
		},
	}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyTimetableMaterializationEnabled: true,
			},
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday:    materializationTestWeekday,
				configModel.KeyTimetableMaterializationWeeksAhead: 3,
			},
		}})

	task := &ScheduledTask{Name: "test"}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 1, m.materializeCalls, "materializer must be called exactly once")
	assert.Equal(t, 1, m.resolveCalls, "ResolveWindow must be called exactly once")
	assert.Equal(t, 3, m.lastWeeksAhead, "weeks-ahead setting must propagate to ResolveWindow")
	assert.Equal(t, scheduleSvc.MaterializationSourceScheduler, m.lastSource,
		"source tag must be scheduler (not manual) for scheduled runs")

	// Verify lastMaterialization was stamped so the next poll skips.
	_, ok := s.lastMaterialization.Load(schedulerUnitTenantID)
	assert.True(t, ok, "lastMaterialization must record that tenant ran today")
}

func TestCheckAndRunMaterialization_ZeroCounters(t *testing.T) {
	t.Parallel()

	// When the result has zero created and zero raced, the success info log
	// is suppressed — but the call still counts and the today-mark is set.
	m := &fakeMaterializer{
		returnResult: &scheduleSvc.MaterializationResult{
			InstancesCreated: 0,
			CandidatesRaced:  0,
		},
	}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyTimetableMaterializationEnabled: true,
			},
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday: materializationTestWeekday,
			},
		}})

	task := &ScheduledTask{Name: "test"}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 1, m.materializeCalls)
}

func TestCheckAndRunMaterialization_MaterializerError(t *testing.T) {
	t.Parallel()

	m := &fakeMaterializer{
		returnErr: errors.New("materialization exploded"),
	}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyTimetableMaterializationEnabled: true,
			},
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday: materializationTestWeekday,
			},
		}})

	task := &ScheduledTask{Name: "test"}

	assert.NotPanics(t, func() {
		s.checkAndRunMaterializationWithContext(context.Background(), task)
	})
	assert.Equal(t, 1, m.materializeCalls, "materializer is invoked even if it errors")
}

func TestCheckAndRunMaterialization_OnlyRacedCounter(t *testing.T) {
	t.Parallel()

	// Triggers the "successful completion" info log branch where
	// InstancesCreated==0 but CandidatesRaced>0.
	m := &fakeMaterializer{
		returnResult: &scheduleSvc.MaterializationResult{
			CandidatesRaced: 4,
			DurationMS:      55,
		},
	}
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		materializer:       m,
		materializationNow: fixedMaterializationTime,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyTimetableMaterializationEnabled: true,
			},
			intValues: map[string]int{
				configModel.KeyTimetableMaterializationWeekday: materializationTestWeekday,
			},
		}})

	task := &ScheduledTask{Name: "test"}

	s.checkAndRunMaterializationWithContext(context.Background(), task)

	assert.Equal(t, 1, m.materializeCalls)
}

func TestRunMaterializationTaskPolling_ExitsOnDone(t *testing.T) {
	t.Parallel()

	// Pre-close done before launching so waitUntilNextMinute returns false
	// immediately after the startup checkAndRunMaterialization completes.
	m := &fakeMaterializer{}
	s := unitScheduler(&Scheduler{
		done:         make(chan struct{}),
		logger:       slog.Default(),
		tasks:        make(map[string]*ScheduledTask),
		materializer: m})

	close(s.done)
	task := &ScheduledTask{Name: "timetable-materialization"}

	finished := make(chan struct{})
	s.wg.Add(1)
	go func() {
		s.runMaterializationTaskPolling(task)
		close(finished)
	}()

	select {
	case <-finished:
		// Goroutine exited cleanly on the done signal.
	case <-time.After(2 * time.Second):
		t.Fatal("runMaterializationTaskPolling did not exit on done signal")
	}
}

func TestRunMaterializationTaskPolling_TickerFires(t *testing.T) {
	t.Parallel()

	// Use synctest to drive the ticker past waitUntilNextMinute and into the
	// 60-second poll loop. The ticker branch + done branch both get exercised.
	synctest.Test(t, func(t *testing.T) {
		m := &fakeMaterializer{}
		s := unitScheduler(&Scheduler{
			done:         make(chan struct{}),
			logger:       slog.Default(),
			tasks:        make(map[string]*ScheduledTask),
			materializer: m,
			settings: &fakeSettingsResolver{
				boolValues: map[string]bool{
					configModel.KeyTimetableMaterializationEnabled: false,
				},
			}})

		task := &ScheduledTask{Name: "timetable-materialization"}

		s.wg.Add(1)
		go s.runMaterializationTaskPolling(task)

		// Advance fake time past waitUntilNextMinute (up to 1 min) plus a
		// couple of ticker intervals so the ticker.C branch executes.
		time.Sleep(3 * time.Minute)
		synctest.Wait()

		// checkAndRunMaterialization was called at startup + on each tick.
		// With the enabled=false override, all calls short-circuit before
		// the materializer — we assert on the cleanup path instead.
		m.mu.Lock()
		assert.Equal(t, 0, m.materializeCalls, "disabled tenant must not call materializer")
		m.mu.Unlock()

		close(s.done)
		s.wg.Wait()
	})
}
