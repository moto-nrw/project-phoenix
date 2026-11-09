package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The failure streak of a nightly cleanup job (#6327). Every test drives the
// production gate with a fixed clock; nothing sleeps.

// cleanupLogs collects the scheduler's JSON log lines.
type cleanupLogs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *cleanupLogs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *cleanupLogs) logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(l, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// take returns the records with message msg and drops them from the buffer.
func (l *cleanupLogs) take(t *testing.T, msg string) []map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var matched []map[string]any
	for _, line := range bytes.Split(l.buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		if record["msg"] == msg {
			matched = append(matched, record)
		}
	}
	l.buf.Reset()
	return matched
}

const (
	alertMessage     = "nightly cleanup failing repeatedly"
	recoveredMessage = "nightly cleanup recovered"
)

// failingTimetableScheduler is catchUpScheduler with a timetable cleanup that
// fails until cleared, logging into logs.
func failingTimetableScheduler(runs ports.CleanupRunLedger, clock *cleanupClock, cleanup *fakeTimetableCleanup, logs *cleanupLogs) *Scheduler {
	s := catchUpScheduler(runs, clock, cleanup)
	s.logger = logs.logger()
	return s
}

func (f *fakeTimetableCleanup) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cleanupCalls
}

func (f *fakeTimetableCleanup) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// expectRetryAt ticks one minute before at, where the job must still wait,
// and at at, where it must run once more.
func expectRetryAt(t *testing.T, s *Scheduler, clock *cleanupClock, cleanup *fakeTimetableCleanup, at time.Time) {
	t.Helper()
	before := cleanup.calls()
	clock.Set(at.Add(-time.Minute))
	runTimetableTick(s)
	require.Equal(t, before, cleanup.calls(), "no retry before %s", at.Format(time.DateTime))
	clock.Set(at)
	runTimetableTick(s)
	require.Equal(t, before+1, cleanup.calls(), "retry at %s", at.Format(time.DateTime))
}

// Invariant: the wait grows with every further failure up to the ceiling.
func TestNightlyCleanupRetryDelayGrowsToTheCeiling(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	s := failingTimetableScheduler(runs, clock, cleanup, &cleanupLogs{})

	runTimetableTick(s)
	require.Equal(t, 1, cleanup.calls(), "the configured minute runs the cleanup")

	expectRetryAt(t, s, clock, cleanup, berlin(14, 3, 0))  // 1 h after failure 1
	expectRetryAt(t, s, clock, cleanup, berlin(14, 5, 0))  // 2 h after failure 2
	expectRetryAt(t, s, clock, cleanup, berlin(14, 9, 0))  // 4 h after failure 3
	expectRetryAt(t, s, clock, cleanup, berlin(14, 17, 0)) // 8 h after failure 4

	// The configured minute of the next day does not bypass the wait.
	clock.Set(berlin(15, 2, 0))
	runTimetableTick(s)
	require.Equal(t, 5, cleanup.calls(), "a new day keeps the failure streak and its wait")

	expectRetryAt(t, s, clock, cleanup, berlin(15, 9, 0)) // 16 h after failure 5
	expectRetryAt(t, s, clock, cleanup, berlin(16, 9, 0)) // 24 h after failure 6: the ceiling
	expectRetryAt(t, s, clock, cleanup, berlin(17, 9, 0)) // still 24 h after failure 7

	last, _ := runs.last("timetable-cleanup")
	assert.Equal(t, berlin(13, 2, 0), last, "no failed run counts as succeeded")
}

func TestCleanupRetryDelay(t *testing.T) {
	t.Parallel()
	for failures, want := range map[int]time.Duration{
		1: time.Hour, 2: 2 * time.Hour, 3: 4 * time.Hour, 4: 8 * time.Hour,
		5: 16 * time.Hour, 6: 24 * time.Hour, 7: 24 * time.Hour, 1000: 24 * time.Hour,
	} {
		assert.Equal(t, want, cleanupRetryDelay(failures), "after %d failures", failures)
	}
}

// Invariant: a success ends the failure streak, its wait and its alert.
func TestNightlyCleanupSuccessResetsTheFailureStreak(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	logs := &cleanupLogs{}
	s := failingTimetableScheduler(runs, clock, cleanup, logs)

	runTimetableTick(s)
	expectRetryAt(t, s, clock, cleanup, berlin(14, 3, 0))
	expectRetryAt(t, s, clock, cleanup, berlin(14, 5, 0))
	require.Len(t, logs.take(t, alertMessage), 1, "the third failure in a row alerts")

	cleanup.setErr(nil)
	expectRetryAt(t, s, clock, cleanup, berlin(14, 9, 0))
	last, _ := runs.last("timetable-cleanup")
	require.Equal(t, berlin(14, 9, 0), last)
	recovered := logs.take(t, recoveredMessage)
	require.Len(t, recovered, 1, "the success after an alerted streak ends the alert")
	assert.InDelta(t, 0, recovered[0]["consecutive_failures"], 0)
	assert.InDelta(t, 3, recovered[0]["previous_failures"], 0)

	// The next failure starts a new streak: one hour, no alert.
	cleanup.setErr(errors.New("poisoned row"))
	clock.Set(berlin(15, 2, 0))
	runTimetableTick(s)
	require.Equal(t, 5, cleanup.calls())
	expectRetryAt(t, s, clock, cleanup, berlin(15, 3, 0))
	assert.Empty(t, logs.take(t, alertMessage), "two failures after a success stay below the threshold")
}

// Invariant: from the N-th failure in a row on, every failed run logs exactly
// one alert line with school, job, failed steps and streak.
func TestNightlyCleanupAlertsOncePerFailedRunFromTheThreshold(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["visit-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	logs := &cleanupLogs{}
	s := unitScheduler(&unitSchedulerDependencies{
		logger:          logs.logger(),
		cleanupService:  &mockCleanupService{cleanupResult: &ports.ExpiredVisitCleanup{Success: true}},
		feedbackCleaner: &fakeFeedbackCleaner{err: errors.New("feedback unavailable")},
		cleanupRuns:     runs,
		cleanupClock:    clock.Now,
		settings: &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: "02:00"},
			intValues:    map[string]int{settingDataCleanupTimeoutMinutes: 30},
		},
		retention: mustRetentionRun(t, []ports.RetentionStep{{Kind: "poisoned_retention", Run: func(context.Context) (ports.RetentionOutcome, error) {
			return ports.RetentionOutcome{}, errors.New("delete failed")
		}}}, func(context.Context, string, ports.RetentionOutcome) error { return nil }),
	})
	tick := func(at time.Time) {
		clock.Set(at)
		s.checkAndRunCleanup(context.Background(), &ScheduledTask{Name: "visit-cleanup"})
	}

	for i, at := range []time.Time{berlin(14, 2, 0), berlin(14, 3, 0)} {
		tick(at)
		assert.Empty(t, logs.take(t, alertMessage), "failure %d stays below the threshold", i+1)
	}
	tick(berlin(14, 5, 0))
	alerts := logs.take(t, alertMessage)
	require.Len(t, alerts, 1, "two failed steps still make one alert line per run")
	alert := alerts[0]
	assert.Equal(t, "ERROR", alert["level"])
	assert.Equal(t, "visit-cleanup", alert["job_id"])
	assert.InDelta(t, schedulerUnitTenantID, alert["tenant_id"], 0)
	assert.InDelta(t, 3, alert["consecutive_failures"], 0)
	assert.Equal(t, "feedback,retention", alert["failed_steps"])

	tick(berlin(14, 8, 59))
	assert.Empty(t, logs.take(t, alertMessage), "a run that waits for its retry does not alert")
	tick(berlin(14, 9, 0))
	alerts = logs.take(t, alertMessage)
	require.Len(t, alerts, 1, "every further failed run alerts once")
	assert.InDelta(t, 4, alerts[0]["consecutive_failures"], 0)
}

// The streak is stored with the last success: a new process keeps waiting.
// A failed run rolls its transaction back, so its failure is written in the
// job's next minute tick.
func TestNightlyCleanupBackoffSurvivesARestart(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	first := failingTimetableScheduler(runs, clock, cleanup, &cleanupLogs{})
	runTimetableTick(first)
	expectRetryAt(t, first, clock, cleanup, berlin(14, 3, 0))
	expectRetryAt(t, first, clock, cleanup, berlin(14, 5, 0))
	assert.Equal(t, 2, runs.failureCount("timetable-cleanup"), "the failure of the run that rolled back is not written yet")
	clock.Set(berlin(14, 5, 1))
	runTimetableTick(first)
	require.Equal(t, 3, runs.failureCount("timetable-cleanup"), "the next tick writes it")

	logs := &cleanupLogs{}
	restarted := failingTimetableScheduler(runs, clock, cleanup, logs)
	expectRetryAt(t, restarted, clock, cleanup, berlin(14, 9, 0))
	alerts := logs.take(t, alertMessage)
	require.Len(t, alerts, 1)
	assert.InDelta(t, 4, alerts[0]["consecutive_failures"], 0, "the new process continues the stored streak")
}

// A failing job does not delay another job of the same school; schools are
// covered in compose.TestNightlyCleanupFailureStreakIsPerSchool.
func TestNightlyCleanupFailureStreakIsPerJob(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	runs.runs["visit-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	s := failingTimetableScheduler(runs, clock, cleanup, &cleanupLogs{})
	s.cleanupService = &mockCleanupService{cleanupResult: &ports.ExpiredVisitCleanup{Success: true}}

	runTimetableTick(s)
	expectRetryAt(t, s, clock, cleanup, berlin(14, 3, 0))
	s.checkAndRunCleanup(context.Background(), &ScheduledTask{Name: "visit-cleanup"})

	last, _ := runs.last("visit-cleanup")
	assert.Equal(t, berlin(14, 3, 0), last, "the other job runs although the timetable cleanup keeps failing")
}

// A school whose cleanup switch cannot be read fails every due run without
// running anything. Its streak is stored all the same, so a restart keeps
// the wait and the alert count.
func TestNightlyCleanupStoresTheStreakOfAnUnreadableSwitch(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{}
	settings := &stubSettingsResolver{boolErr: errors.New("settings unavailable"), stringVal: "02:00", intVal: 30}
	newWorker := func() *Scheduler {
		return unitScheduler(&unitSchedulerDependencies{
			logger: slog.Default(), timetableCleanup: cleanup, cleanupRuns: runs,
			cleanupClock: clock.Now, settings: settings,
		})
	}
	s := newWorker()
	for _, at := range []time.Time{berlin(14, 2, 0), berlin(14, 3, 0), berlin(14, 5, 0)} {
		clock.Set(at)
		runTimetableTick(s)
	}
	require.Equal(t, 3, runs.failureCount("timetable-cleanup"), "every failed run is stored in its own tick")

	settings.boolErr = nil
	settings.boolVal = true
	restarted := newWorker()
	expectRetryAt(t, restarted, clock, cleanup, berlin(14, 9, 0))
}

// A streak the process kept is dropped once another Worker stored a later
// success, for example after the lease moved and came back.
func TestNightlyCleanupIgnoresAStreakSupersededByASuccess(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	s := failingTimetableScheduler(runs, clock, cleanup, &cleanupLogs{})
	runTimetableTick(s)
	expectRetryAt(t, s, clock, cleanup, berlin(14, 3, 0))

	// Another Worker succeeds while this one is not leading.
	require.NoError(t, runs.RecordCleanupSucceeded(context.Background(), "timetable-cleanup", berlin(14, 6, 0)))

	clock.Set(berlin(15, 2, 0))
	runTimetableTick(s)
	require.Equal(t, 3, cleanup.calls(), "the configured minute of the next day runs")
	expectRetryAt(t, s, clock, cleanup, berlin(15, 3, 0))
}

// The success after a failure always writes the recovery line, also when
// the failure that raised the alert was lost before it was stored.
func TestNightlyCleanupRecoversAnAlertWhoseFailureWasNotStored(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	logs := &cleanupLogs{}
	first := failingTimetableScheduler(runs, clock, cleanup, logs)
	runTimetableTick(first)
	expectRetryAt(t, first, clock, cleanup, berlin(14, 3, 0))
	expectRetryAt(t, first, clock, cleanup, berlin(14, 5, 0))
	require.Len(t, logs.take(t, alertMessage), 1)
	require.Equal(t, 2, runs.failureCount("timetable-cleanup"), "the third failure is not stored before the restart")

	cleanup.setErr(nil)
	restarted := failingTimetableScheduler(runs, clock, cleanup, logs)
	clock.Set(berlin(14, 5, 1))
	runTimetableTick(restarted)
	require.Equal(t, 4, cleanup.calls(), "the restarted Worker retries after the stored two-hour wait")
	recovered := logs.take(t, recoveredMessage)
	require.Len(t, recovered, 1, "the success ends the alert")
	assert.InDelta(t, 0, recovered[0]["consecutive_failures"], 0)
}

// A school whose cleanup time cannot be read cannot tell when its run is
// due. Each attempt after the wait counts as a failed run, so the streak
// backs off and raises the alert like any other failure.
func TestNightlyCleanupCountsAnUnreadableCleanupTimeAsFailure(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{}
	logs := &cleanupLogs{}
	s := unitScheduler(&unitSchedulerDependencies{
		logger: logs.logger(), timetableCleanup: cleanup, cleanupRuns: runs, cleanupClock: clock.Now,
		settings: &stubSettingsResolver{boolVal: true, stringErr: errors.New("cleanup time unreadable"), intVal: 30},
	})
	tick := func(at time.Time) {
		clock.Set(at)
		runTimetableTick(s)
	}

	tick(berlin(14, 2, 0))
	tick(berlin(14, 2, 30))
	tick(berlin(14, 3, 0))
	tick(berlin(14, 4, 59))
	tick(berlin(14, 5, 0))
	tick(berlin(14, 5, 1))

	assert.Zero(t, cleanup.calls(), "nothing runs without a cleanup time")
	assert.Equal(t, 3, runs.failureCount("timetable-cleanup"), "failures at 02:00, 03:00 and 05:00, none within the wait")
	alerts := logs.take(t, alertMessage)
	require.Len(t, alerts, 1, "the third failure alerts once")
	assert.Equal(t, "run", alerts[0]["failed_steps"])
}

// A failure write that keeps failing must not stop the cleanup: the process
// keeps the streak, retries after the wait and still raises the alert.
func TestNightlyCleanupKeepsRunningWhenTheFailureWriteFails(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	runs.failureErr = errors.New("ledger refuses the failure")
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("poisoned row")}
	logs := &cleanupLogs{}
	s := failingTimetableScheduler(runs, clock, cleanup, logs)

	runTimetableTick(s)
	expectRetryAt(t, s, clock, cleanup, berlin(14, 3, 0))
	expectRetryAt(t, s, clock, cleanup, berlin(14, 5, 0))
	require.Len(t, logs.take(t, alertMessage), 1, "the third failure alerts from the streak the process keeps")
	assert.Zero(t, runs.failureCount("timetable-cleanup"))
}
