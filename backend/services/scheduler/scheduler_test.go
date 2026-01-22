package scheduler

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

// =============================================================================
// NewScheduler Tests
// =============================================================================

func TestNewScheduler(t *testing.T) {
	auth := &fakeAuthCleanup{}

	s := NewScheduler(nil, nil, auth)

	require.NotNil(t, s)
	assert.NotNil(t, s.tasks)
	assert.NotNil(t, s.done)
	assert.Len(t, s.cleanupJobs, 3) // 3 auth jobs
}

func TestNewScheduler_NilServices(t *testing.T) {
	s := NewScheduler(nil, nil, nil)

	require.NotNil(t, s)
	assert.Empty(t, s.cleanupJobs)
}

func TestNewScheduler_OnlyAuthService(t *testing.T) {
	auth := &fakeAuthCleanup{}

	s := NewScheduler(nil, nil, auth)

	require.NotNil(t, s)
	assert.Len(t, s.cleanupJobs, 3) // 3 auth jobs only
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

	s := NewScheduler(nil, nil, nil)

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
	s := NewScheduler(nil, nil, nil)

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

	synctest.Test(t, func(t *testing.T) {
		auth := &fakeAuthCleanup{
			tokenResult:     1,
			passwordResult:  2,
			rateLimitResult: 3,
		}

		s := NewScheduler(nil, nil, auth)
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
	auth := &fakeAuthCleanup{
		tokenResult:     1,
		passwordResult:  2,
		rateLimitResult: 3,
	}

	s := NewScheduler(nil, nil, auth)

	if err := s.RunCleanupJobs(); err != nil {
		t.Fatalf("RunCleanupJobs() returned error: %v", err)
	}

	if auth.tokenCalls != 1 || auth.passwordCalls != 1 || auth.rateLimitCalls != 1 {
		t.Fatalf("expected auth cleanup jobs to be called once each, got tokens=%d passwords=%d rate=%d",
			auth.tokenCalls, auth.passwordCalls, auth.rateLimitCalls)
	}
}

func TestRunCleanupJobsReturnsFirstErrorAndContinues(t *testing.T) {
	expectedErr := errors.New("rate limit cleanup failed")

	auth := &fakeAuthCleanup{
		rateLimitErr: expectedErr,
	}

	s := NewScheduler(nil, nil, auth)

	err := s.RunCleanupJobs()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if auth.tokenCalls != 1 || auth.passwordCalls != 1 || auth.rateLimitCalls != 1 {
		t.Fatalf("expected auth cleanup jobs to be called once each despite error, got tokens=%d passwords=%d rate=%d",
			auth.tokenCalls, auth.passwordCalls, auth.rateLimitCalls)
	}
}

func TestRunCleanupJobs_NoJobs(t *testing.T) {
	s := NewScheduler(nil, nil, nil)

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

	s := NewScheduler(nil, nil, auth)

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

	s := NewScheduler(nil, nil, auth)

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

	jobs := buildCleanupJobs(auth)

	assert.Len(t, jobs, 3)
	assert.Equal(t, "Auth token cleanup", jobs[0].Description)
	assert.Equal(t, "Password reset token cleanup", jobs[1].Description)
	assert.Equal(t, "Password reset rate limit cleanup", jobs[2].Description)
}

func TestBuildCleanupJobs_NoServices(t *testing.T) {
	jobs := buildCleanupJobs(nil)
	assert.Empty(t, jobs)
}

func TestBuildCleanupJobs_OnlyAuth(t *testing.T) {
	auth := &fakeAuthCleanup{}

	jobs := buildCleanupJobs(auth)

	assert.Len(t, jobs, 3)
}

func TestBuildCleanupJobs_JobsAreCallable(t *testing.T) {
	auth := &fakeAuthCleanup{tokenResult: 5}

	jobs := buildCleanupJobs(auth)
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

func TestScheduledTask_ConcurrentAccess(_ *testing.T) {
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

	synctest.Test(t, func(t *testing.T) {
		s := NewScheduler(nil, nil, nil)
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
	// Clear all env vars to test defaults
	_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
	_ = os.Unsetenv("CLEANUP_SCHEDULER_TIME")
	_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
	_ = os.Unsetenv("SESSION_END_TIME")
	_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
	_ = os.Unsetenv("SESSION_CLEANUP_INTERVAL_MINUTES")
	_ = os.Unsetenv("SESSION_ABANDONED_THRESHOLD_MINUTES")

	s := NewScheduler(nil, nil, nil)

	// Default values should be set
	assert.Equal(t, 0, s.sessionCleanupIntervalMinutes) // Not set until Start()
	assert.Equal(t, 0, s.sessionAbandonedThresholdMinutes)
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestAuthCleanup_InterfaceCompliance(_ *testing.T) {
	// Verify fakeAuthCleanup implements AuthCleanup
	var _ AuthCleanup = &fakeAuthCleanup{}
}

// =============================================================================
// Time Parsing and Task State Tests
// Note: Execute functions require full interface implementations which are complex
// to mock. These tests focus on the testable aspects: time parsing, task state
// management, and scheduler lifecycle.
// =============================================================================

func TestScheduleCleanupTask_InvalidTimeFormat(t *testing.T) {
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_TIME", "invalid"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("CLEANUP_SCHEDULER_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_TIME", "25:00"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("CLEANUP_SCHEDULER_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_TIME", "02:99"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("CLEANUP_SCHEDULER_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

		// Schedule cleanup task with invalid minute
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_InvalidTimeFormat(t *testing.T) {
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_END_TIME", "invalid"))
	defer func() {
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_END_TIME", "30:00"))
	defer func() {
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_END_TIME", "18:99"))
	defer func() {
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

		// Schedule session end task with invalid minute
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
	// Test that session cleanup can be disabled via env var
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "false"))
	defer func() {
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
	}()

	s := &Scheduler{
		tasks: make(map[string]*ScheduledTask),
		done:  make(chan struct{}),
	}

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
func (m *mockActiveService) WithTx(_ bun.Tx) interface{} {
	return m
}
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
func (m *mockActiveService) GetActiveGroupsCount(_ context.Context) (int, error) { return 0, nil }
func (m *mockActiveService) GetTotalVisitsCount(_ context.Context) (int, error)  { return 0, nil }
func (m *mockActiveService) GetActiveVisitsCount(_ context.Context) (int, error) { return 0, nil }
func (m *mockActiveService) GetRoomUtilization(_ context.Context, _ int64) (float64, error) {
	return 0, nil
}
func (m *mockActiveService) GetStudentAttendanceRate(_ context.Context, _ int64) (float64, error) {
	return 0, nil
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
func (m *mockActiveService) CheckTeacherStudentAccess(_ context.Context, _, _ int64) (bool, error) {
	return false, nil
}
func (m *mockActiveService) GetUnclaimedActiveGroups(_ context.Context) ([]*active.Group, error) {
	return nil, nil
}
func (m *mockActiveService) ClaimActiveGroup(_ context.Context, _, _ int64, _ string) (*active.GroupSupervisor, error) {
	return nil, nil
}

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
	return nil, m.attendanceErr
}

func (m *mockCleanupService) PreviewAttendanceCleanup(_ context.Context) (*activeService.AttendanceCleanupPreview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attendancePreviewCalls++
	return nil, m.attendancePreviewErr
}

// =============================================================================
// Execute Tests
// =============================================================================

func TestExecuteCleanup_Success(t *testing.T) {
	cleanupSvc := &mockCleanupService{
		cleanupResult: &activeService.CleanupResult{
			StudentsProcessed: 10,
			RecordsDeleted:    100,
			Success:           true,
		},
	}

	s := &Scheduler{
		cleanupService: cleanupSvc,
		done:           make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-cleanup"}

	// Execute cleanup
	s.executeCleanup(task)

	// Verify cleanup was called
	cleanupSvc.mu.Lock()
	assert.Equal(t, 1, cleanupSvc.cleanupCalls)
	cleanupSvc.mu.Unlock()

	// Verify task state
	task.mu.Lock()
	assert.False(t, task.Running, "Task should not be running after completion")
	assert.False(t, task.LastRun.IsZero(), "LastRun should be set")
	task.mu.Unlock()
}

func TestExecuteCleanup_Error(t *testing.T) {
	cleanupSvc := &mockCleanupService{
		cleanupErr: errors.New("cleanup failed"),
	}

	s := &Scheduler{
		cleanupService: cleanupSvc,
		done:           make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-cleanup"}

	// Execute cleanup (should handle error gracefully)
	s.executeCleanup(task)

	// Verify cleanup was called
	cleanupSvc.mu.Lock()
	assert.Equal(t, 1, cleanupSvc.cleanupCalls)
	cleanupSvc.mu.Unlock()
}

func TestExecuteCleanup_WithErrors(t *testing.T) {
	cleanupSvc := &mockCleanupService{
		cleanupResult: &activeService.CleanupResult{
			StudentsProcessed: 10,
			RecordsDeleted:    90,
			Success:           true,
			Errors: []activeService.CleanupError{
				{StudentID: 1, Error: "error 1"},
				{StudentID: 2, Error: "error 2"},
			},
		},
	}

	s := &Scheduler{
		cleanupService: cleanupSvc,
		done:           make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-cleanup"}

	// Execute cleanup
	s.executeCleanup(task)

	// Verify cleanup was called
	cleanupSvc.mu.Lock()
	assert.Equal(t, 1, cleanupSvc.cleanupCalls)
	cleanupSvc.mu.Unlock()
}

func TestExecuteCleanup_WithManyErrors(t *testing.T) {
	// Test that more than 10 errors are truncated in logging
	var errors []activeService.CleanupError
	for i := 0; i < 15; i++ {
		errors = append(errors, activeService.CleanupError{StudentID: int64(i), Error: "error"})
	}

	cleanupSvc := &mockCleanupService{
		cleanupResult: &activeService.CleanupResult{
			StudentsProcessed: 20,
			RecordsDeleted:    50,
			Success:           true,
			Errors:            errors,
		},
	}

	s := &Scheduler{
		cleanupService: cleanupSvc,
		done:           make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-cleanup"}

	// Execute cleanup
	s.executeCleanup(task)

	// Verify cleanup was called
	cleanupSvc.mu.Lock()
	assert.Equal(t, 1, cleanupSvc.cleanupCalls)
	cleanupSvc.mu.Unlock()
}

func TestExecuteCleanup_AlreadyRunning(t *testing.T) {
	cleanupSvc := &mockCleanupService{}

	s := &Scheduler{
		cleanupService: cleanupSvc,
		done:           make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-cleanup", Running: true}

	// Execute cleanup (should skip because already running)
	s.executeCleanup(task)

	// Verify cleanup was NOT called
	cleanupSvc.mu.Lock()
	assert.Equal(t, 0, cleanupSvc.cleanupCalls)
	cleanupSvc.mu.Unlock()
}

func TestExecuteCleanup_CustomTimeout(t *testing.T) {
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_TIMEOUT_MINUTES", "60"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_TIMEOUT_MINUTES")
	}()

	cleanupSvc := &mockCleanupService{
		cleanupResult: &activeService.CleanupResult{
			StudentsProcessed: 5,
			RecordsDeleted:    50,
			Success:           true,
		},
	}

	s := &Scheduler{
		cleanupService: cleanupSvc,
		done:           make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-cleanup"}

	// Execute cleanup
	s.executeCleanup(task)

	// Verify cleanup was called
	cleanupSvc.mu.Lock()
	assert.Equal(t, 1, cleanupSvc.cleanupCalls)
	cleanupSvc.mu.Unlock()
}

func TestExecuteSessionEnd_Success(t *testing.T) {
	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded:    5,
			VisitsEnded:      20,
			SupervisorsEnded: 3,
			Success:          true,
		},
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-session-end"}

	// Execute session end
	s.executeSessionEnd(task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()

	// Verify task state
	task.mu.Lock()
	assert.False(t, task.Running, "Task should not be running after completion")
	task.mu.Unlock()
}

func TestExecuteSessionEnd_Error(t *testing.T) {
	activeSvc := &mockActiveService{
		endDailySessionsErr: errors.New("session end failed"),
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-session-end"}

	// Execute session end (should handle error gracefully)
	s.executeSessionEnd(task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEnd_WithErrors(t *testing.T) {
	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded:    5,
			VisitsEnded:      20,
			SupervisorsEnded: 3,
			Success:          true,
			Errors:           []string{"error 1", "error 2"},
		},
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-session-end"}

	// Execute session end
	s.executeSessionEnd(task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEnd_WithManyErrors(t *testing.T) {
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

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-session-end"}

	// Execute session end
	s.executeSessionEnd(task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEnd_AlreadyRunning(t *testing.T) {
	activeSvc := &mockActiveService{}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-session-end", Running: true}

	// Execute session end (should skip because already running)
	s.executeSessionEnd(task)

	// Verify service was NOT called
	activeSvc.mu.Lock()
	assert.Equal(t, 0, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionEnd_CustomTimeout(t *testing.T) {
	require.NoError(t, os.Setenv("SESSION_END_TIMEOUT_MINUTES", "30"))
	defer func() {
		_ = os.Unsetenv("SESSION_END_TIMEOUT_MINUTES")
	}()

	activeSvc := &mockActiveService{
		endDailySessionsResult: &activeService.DailySessionCleanupResult{
			SessionsEnded: 5,
			Success:       true,
		},
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "test-session-end"}

	// Execute session end
	s.executeSessionEnd(task)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.endDailySessionsCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteTokenCleanup_Success(t *testing.T) {
	auth := &fakeAuthCleanup{
		tokenResult:     5,
		passwordResult:  3,
		rateLimitResult: 2,
	}

	s := NewScheduler(nil, nil, auth)

	task := &ScheduledTask{Name: "token-cleanup"}

	// Execute token cleanup
	s.executeTokenCleanup(task)

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
	auth := &fakeAuthCleanup{}

	s := NewScheduler(nil, nil, auth)

	task := &ScheduledTask{Name: "token-cleanup", Running: true}

	// Execute token cleanup (should skip because already running)
	s.executeTokenCleanup(task)

	// Verify no cleanup jobs were called
	auth.mu.Lock()
	assert.Equal(t, 0, auth.tokenCalls)
	auth.mu.Unlock()
}

func TestExecuteTokenCleanup_Error(t *testing.T) {
	auth := &fakeAuthCleanup{
		tokenErr: errors.New("token cleanup failed"),
	}

	s := NewScheduler(nil, nil, auth)

	task := &ScheduledTask{Name: "token-cleanup"}

	// Execute token cleanup (should handle error gracefully)
	s.executeTokenCleanup(task)

	// Verify cleanup was attempted
	auth.mu.Lock()
	assert.Equal(t, 1, auth.tokenCalls)
	auth.mu.Unlock()
}

func TestExecuteSessionCleanup_Success(t *testing.T) {
	activeSvc := &mockActiveService{
		cleanupAbandonedResult: 5,
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "session-cleanup"}

	// Execute session cleanup
	s.executeSessionCleanup(task, 15, 60)

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

func TestExecuteSessionCleanup_NoAbandoned(t *testing.T) {
	activeSvc := &mockActiveService{
		cleanupAbandonedResult: 0,
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "session-cleanup"}

	// Execute session cleanup
	s.executeSessionCleanup(task, 15, 30)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.cleanupAbandonedCalls)
	assert.Equal(t, 30*time.Minute, activeSvc.cleanupAbandonedDuration)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionCleanup_Error(t *testing.T) {
	activeSvc := &mockActiveService{
		cleanupAbandonedErr: errors.New("cleanup failed"),
	}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "session-cleanup"}

	// Execute session cleanup (should handle error gracefully)
	s.executeSessionCleanup(task, 15, 60)

	// Verify service was called
	activeSvc.mu.Lock()
	assert.Equal(t, 1, activeSvc.cleanupAbandonedCalls)
	activeSvc.mu.Unlock()
}

func TestExecuteSessionCleanup_AlreadyRunning(t *testing.T) {
	activeSvc := &mockActiveService{}

	s := &Scheduler{
		activeService: activeSvc,
		done:          make(chan struct{}),
	}

	task := &ScheduledTask{Name: "session-cleanup", Running: true}

	// Execute session cleanup (should skip because already running)
	s.executeSessionCleanup(task, 15, 60)

	// Verify service was NOT called
	activeSvc.mu.Lock()
	assert.Equal(t, 0, activeSvc.cleanupAbandonedCalls)
	activeSvc.mu.Unlock()
}

// =============================================================================
// Configuration Parsing Tests
// =============================================================================

func TestScheduleSessionCleanupTask_CustomInterval(t *testing.T) {
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_CLEANUP_INTERVAL_MINUTES", "30"))
	defer func() {
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
		_ = os.Unsetenv("SESSION_CLEANUP_INTERVAL_MINUTES")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			activeService: &mockActiveService{}, // Needed for session cleanup
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_ABANDONED_THRESHOLD_MINUTES", "120"))
	defer func() {
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
		_ = os.Unsetenv("SESSION_ABANDONED_THRESHOLD_MINUTES")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			activeService: &mockActiveService{}, // Needed for session cleanup
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_CLEANUP_INTERVAL_MINUTES", "invalid"))
	defer func() {
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
		_ = os.Unsetenv("SESSION_CLEANUP_INTERVAL_MINUTES")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			activeService: &mockActiveService{}, // Needed for session cleanup
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("SESSION_CLEANUP_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_CLEANUP_INTERVAL_MINUTES", "-5"))
	defer func() {
		_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")
		_ = os.Unsetenv("SESSION_CLEANUP_INTERVAL_MINUTES")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			activeService: &mockActiveService{}, // Needed for session cleanup
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{}),
		}

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
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("CLEANUP_SCHEDULER_TIME", "03:30"))
	defer func() {
		_ = os.Unsetenv("CLEANUP_SCHEDULER_ENABLED")
		_ = os.Unsetenv("CLEANUP_SCHEDULER_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

		// Schedule cleanup task
		s.scheduleCleanupTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was created with custom schedule
		s.mu.RLock()
		task, hasTask := s.tasks["visit-cleanup"]
		s.mu.RUnlock()
		assert.True(t, hasTask)
		assert.Equal(t, "03:30", task.Schedule)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_CustomTime(t *testing.T) {
	require.NoError(t, os.Setenv("SESSION_END_SCHEDULER_ENABLED", "true"))
	require.NoError(t, os.Setenv("SESSION_END_TIME", "17:00"))
	defer func() {
		_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")
		_ = os.Unsetenv("SESSION_END_TIME")
	}()

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

		// Schedule session end task
		s.scheduleSessionEndTask()

		// Wait for goroutines to be durably blocked (fake time makes sleeps instant)
		synctest.Wait()

		// Verify task was created with custom schedule
		s.mu.RLock()
		task, hasTask := s.tasks["session-end"]
		s.mu.RUnlock()
		assert.True(t, hasTask)
		assert.Equal(t, "17:00", task.Schedule)

		// Stop scheduler
		close(s.done)
		s.wg.Wait()
	})
}

func TestScheduleSessionEndTask_DefaultEnabled(t *testing.T) {
	// Clear env var to test default behavior (enabled)
	_ = os.Unsetenv("SESSION_END_SCHEDULER_ENABLED")

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			tasks: make(map[string]*ScheduledTask),
			done:  make(chan struct{}),
		}

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
	// Clear env var to test default behavior (enabled)
	_ = os.Unsetenv("SESSION_CLEANUP_ENABLED")

	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			activeService: &mockActiveService{}, // Needed for session cleanup
			tasks:         make(map[string]*ScheduledTask),
			done:          make(chan struct{}),
		}

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

func TestMockActiveService_ImplementsInterface(_ *testing.T) {
	var _ activeService.Service = &mockActiveService{}
}

func TestMockCleanupService_ImplementsInterface(_ *testing.T) {
	var _ activeService.CleanupService = &mockCleanupService{}
}
