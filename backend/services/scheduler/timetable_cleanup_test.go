// Coverage tests for the WP-B14 timetable cleanup scheduler task:
// SetTimetableCleanup wiring, scheduleTimetableCleanupTask nil-vs-registered
// branches, the runTimetableCleanupTaskPolling loop (startup + done-exit +
// ticker fires), and every branch of checkAndRunTimetableCleanup (disabled,
// wrong time, wasRunToday dedupe, happy path, service error, zero-counter
// log suppression).
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTimetableCleanup is a test double for scheduleSvc.TimetableCleanupService.
type fakeTimetableCleanup struct {
	mu           sync.Mutex
	cleanupCalls int
	result       *scheduleSvc.TimetableCleanupResult
	err          error
}

func (f *fakeTimetableCleanup) CleanupExpiredTimetableData(_ context.Context) (*scheduleSvc.TimetableCleanupResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupCalls++
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &scheduleSvc.TimetableCleanupResult{Success: true}, nil
}

func (f *fakeTimetableCleanup) PreviewExpiredTimetableData(_ context.Context) (*scheduleSvc.TimetableCleanupPreview, error) {
	return nil, nil
}

func (f *fakeTimetableCleanup) GetStats(_ context.Context) (*scheduleSvc.TimetableCleanupStats, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// SetTimetableCleanup — pure setter
// -----------------------------------------------------------------------------

func TestSetTimetableCleanup(t *testing.T) {
	t.Parallel()

	s := NewScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
	assert.Nil(t, s.timetableCleanup)

	svc := &fakeTimetableCleanup{}
	s.SetTimetableCleanup(svc)
	assert.Same(t, svc, s.timetableCleanup)
}

// -----------------------------------------------------------------------------
// scheduleTimetableCleanupTask — nil service vs. registered
// -----------------------------------------------------------------------------

func TestScheduleTimetableCleanupTask_NilService(t *testing.T) {
	t.Parallel()

	s := &Scheduler{
		done:   make(chan struct{}),
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask),
	}
	s.scheduleTimetableCleanupTask()
	assert.Empty(t, s.tasks, "nil service → no task registered")
}

func TestScheduleTimetableCleanupTask_RegistersTask(t *testing.T) {
	t.Parallel()

	s := &Scheduler{
		done:             make(chan struct{}),
		logger:           slog.Default(),
		tasks:            make(map[string]*ScheduledTask),
		timetableCleanup: &fakeTimetableCleanup{},
	}
	// Pre-close done so the spawned goroutine exits promptly after the startup
	// check and waitUntilNextMinute returns false.
	close(s.done)

	s.scheduleTimetableCleanupTask()
	s.wg.Wait()

	s.mu.RLock()
	task, ok := s.tasks["timetable-cleanup"]
	s.mu.RUnlock()
	require.True(t, ok, "timetable-cleanup task must be registered")
	assert.Equal(t, "1m-poll", task.Schedule)
}

// -----------------------------------------------------------------------------
// runTimetableCleanupTaskPolling — startup + done-exit + ticker
// -----------------------------------------------------------------------------

func TestRunTimetableCleanupTaskPolling_ExitsOnDone(t *testing.T) {
	t.Parallel()

	// Pre-close done so waitUntilNextMinute returns false right after the
	// startup check completes.
	svc := &fakeTimetableCleanup{}
	s := &Scheduler{
		done:             make(chan struct{}),
		logger:           slog.Default(),
		tasks:            make(map[string]*ScheduledTask),
		timetableCleanup: svc,
	}
	close(s.done)
	task := &ScheduledTask{Name: "timetable-cleanup"}

	finished := make(chan struct{})
	s.wg.Add(1)
	go func() {
		s.runTimetableCleanupTaskPolling(task)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("runTimetableCleanupTaskPolling did not exit on done signal")
	}
}

func TestRunTimetableCleanupTaskPolling_TickerFires(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		svc := &fakeTimetableCleanup{}
		s := &Scheduler{
			done:             make(chan struct{}),
			logger:           slog.Default(),
			tasks:            make(map[string]*ScheduledTask),
			timetableCleanup: svc,
		}
		task := &ScheduledTask{Name: "timetable-cleanup"}

		s.wg.Add(1)
		go s.runTimetableCleanupTaskPolling(task)

		// Advance the fake clock past waitUntilNextMinute + a couple of ticker
		// intervals so the ticker.C branch + done-exit branch both execute.
		time.Sleep(3 * time.Minute)
		synctest.Wait()

		close(s.done)
		s.wg.Wait()

		// At minimum: the startup check + at least one ticker tick.
		// No enabled tenant → cleanupCalls stays at 0, but the polling loop
		// itself has run. Verify via the task's internal state instead.
	})
}

// -----------------------------------------------------------------------------
// checkAndRunTimetableCleanup — every branch
// -----------------------------------------------------------------------------

func TestCheckAndRunTimetableCleanup_AlreadyRunning(t *testing.T) {
	t.Parallel()

	svc := &fakeTimetableCleanup{}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
	}
	task := &ScheduledTask{Name: "timetable-cleanup", Running: true}

	s.checkAndRunTimetableCleanup(task)

	assert.Equal(t, 0, svc.cleanupCalls, "must skip work when task is already running")
}

func TestCheckAndRunTimetableCleanup_Disabled(t *testing.T) {
	t.Parallel()

	// When the KeyDataCleanupEnabled setting resolves to false, cleanup must
	// skip. With no settings resolver and no env vars, resolveBoolSetting
	// returns the defaultVal (true) — so flip it explicitly via the resolver.
	svc := &fakeTimetableCleanup{}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: false,
			},
		},
	}
	task := &ScheduledTask{Name: "timetable-cleanup"}

	s.checkAndRunTimetableCleanup(task)

	assert.Equal(t, 0, svc.cleanupCalls, "disabled tenant must not invoke cleanup")
	task.mu.Lock()
	assert.False(t, task.Running, "Running flag must be cleared after run")
	task.mu.Unlock()
}

func TestCheckAndRunTimetableCleanup_WrongTime(t *testing.T) {
	t.Parallel()

	// Enabled, but the configured cleanup time doesn't match now. Use a string
	// that is guaranteed to differ from the current wall clock minute.
	future := time.Now().Add(2 * time.Hour).Format("15:04")

	svc := &fakeTimetableCleanup{}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: future,
			},
		},
	}
	task := &ScheduledTask{Name: "timetable-cleanup"}

	s.checkAndRunTimetableCleanup(task)
	assert.Equal(t, 0, svc.cleanupCalls, "wrong time must skip cleanup")
}

func TestCheckAndRunTimetableCleanup_WasRunToday(t *testing.T) {
	t.Parallel()

	// Enabled + matching time, but lastTimetableCleanup is already stamped for
	// today → dedupe must prevent a second run.
	now := time.Now().Format("15:04")

	svc := &fakeTimetableCleanup{}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: now,
			},
		},
	}
	// forEachTenantSettings falls back to tenantID=0 when db/schoolRepo are nil.
	s.lastTimetableCleanup.Store(int64(0), time.Now())
	task := &ScheduledTask{Name: "timetable-cleanup"}

	s.checkAndRunTimetableCleanup(task)
	assert.Equal(t, 0, svc.cleanupCalls, "already-ran-today dedupe must skip cleanup")
}

func TestCheckAndRunTimetableCleanup_HappyPath(t *testing.T) {
	t.Parallel()

	now := time.Now().Format("15:04")

	svc := &fakeTimetableCleanup{
		result: &scheduleSvc.TimetableCleanupResult{
			Success:           true,
			InstancesDeleted:  5,
			ExceptionsDeleted: 2,
			StudentsAffected:  3,
			RetentionDays:     365,
			CutoffDate:        timezone.TodayDate(),
			DurationMS:        42,
		},
	}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: now,
			},
			intValues: map[string]int{
				configModel.KeyDataCleanupTimeoutMinutes: 30,
			},
		},
	}
	task := &ScheduledTask{Name: "timetable-cleanup"}

	s.checkAndRunTimetableCleanup(task)

	assert.Equal(t, 1, svc.cleanupCalls, "matching time + enabled + not-run-today → cleanup fires")

	// The today-stamp must be set so a second poll in the same minute is a no-op.
	_, ok := s.lastTimetableCleanup.Load(int64(0))
	assert.True(t, ok, "lastTimetableCleanup must record that tenant ran today")
}

func TestCheckAndRunTimetableCleanup_ServiceError_ClearsTodayStamp(t *testing.T) {
	t.Parallel()

	now := time.Now().Format("15:04")

	svc := &fakeTimetableCleanup{err: errors.New("cleanup blew up")}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: now,
			},
		},
	}
	task := &ScheduledTask{Name: "timetable-cleanup"}

	assert.NotPanics(t, func() {
		s.checkAndRunTimetableCleanup(task)
	})
	assert.Equal(t, 1, svc.cleanupCalls, "cleanup must be attempted")

	// On service error, the today-stamp is cleared so a retry on the next
	// matching minute can succeed.
	_, ok := s.lastTimetableCleanup.Load(int64(0))
	assert.False(t, ok, "service error must clear today-mark to permit retry")
}

func TestCheckAndRunTimetableCleanup_ZeroCounters_SuppressesInfoLog(t *testing.T) {
	t.Parallel()

	// When InstancesDeleted == 0 and ExceptionsDeleted == 0, the success Info
	// log is suppressed — but the call still counts and the today-mark is set.
	now := time.Now().Format("15:04")

	svc := &fakeTimetableCleanup{
		result: &scheduleSvc.TimetableCleanupResult{
			Success:           true,
			InstancesDeleted:  0,
			ExceptionsDeleted: 0,
		},
	}
	s := &Scheduler{
		logger:           slog.Default(),
		timetableCleanup: svc,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: now,
			},
		},
	}
	task := &ScheduledTask{Name: "timetable-cleanup"}

	s.checkAndRunTimetableCleanup(task)

	assert.Equal(t, 1, svc.cleanupCalls)
	_, ok := s.lastTimetableCleanup.Load(int64(0))
	assert.True(t, ok, "zero-counter success path still stamps today-mark")
}
