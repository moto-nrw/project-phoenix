package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCleanupRuns is the ledger of last successful runs, shared between
// scheduler instances to stand in for a process restart.
type fakeCleanupRuns struct {
	mu       sync.Mutex
	runs     map[string]time.Time
	recorded int
}

func newFakeCleanupRuns() *fakeCleanupRuns {
	return &fakeCleanupRuns{runs: map[string]time.Time{}}
}

func (f *fakeCleanupRuns) LastCleanupSucceeded(_ context.Context, jobID string) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.runs[jobID]
	return at, ok, nil
}

func (f *fakeCleanupRuns) RecordCleanupSucceeded(_ context.Context, jobID string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[jobID] = at
	f.recorded++
	return nil
}

func (f *fakeCleanupRuns) last(jobID string) (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.runs[jobID]
	return at, ok
}

// berlin returns a fixed instant on 14 October 2026 in Berlin.
func berlin(day, hour, minute int) time.Time {
	return time.Date(2026, time.October, day, hour, minute, 0, 0, calendar.Berlin)
}

// cleanupClock is a settable clock for the nightly cleanup gate.
type cleanupClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *cleanupClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *cleanupClock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

// catchUpScheduler is one Worker process whose timetable cleanup runs at
// 02:00 and whose last successful runs live in runs.
func catchUpScheduler(runs CleanupRunLedger, clock *cleanupClock, cleanup *fakeTimetableCleanup) *Scheduler {
	return unitScheduler(&Scheduler{
		logger:           slog.Default(),
		timetableCleanup: cleanup,
		cleanupRuns:      runs,
		cleanupClock:     clock.Now,
		settings: &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: "02:00"},
		},
	})
}

func runTimetableTick(s *Scheduler) {
	s.checkAndRunTimetableCleanup(context.Background(), &ScheduledTask{Name: "timetable-cleanup"})
}

func TestNightlyCleanupCatchesUpARunMissedForMoreThanADay(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0).Add(-time.Minute)}
	cleanup := &fakeTimetableCleanup{}
	s := catchUpScheduler(runs, clock, cleanup)

	runTimetableTick(s)
	assert.Equal(t, 0, cleanup.cleanupCalls, "23:59 hours after the last run the cleanup is not yet due")

	// The process was down at 02:00 and comes back at 10:15.
	clock.Set(berlin(14, 10, 15))
	runTimetableTick(s)

	assert.Equal(t, 1, cleanup.cleanupCalls, "a run missed for more than a day is caught up at the next tick")
	last, ok := runs.last("timetable-cleanup")
	require.True(t, ok)
	assert.Equal(t, berlin(14, 10, 15), last)

	clock.Set(berlin(14, 10, 16))
	runTimetableTick(s)
	assert.Equal(t, 1, cleanup.cleanupCalls, "the caught-up run counts as today's run")
}

func TestNightlyCleanupRunsOnceADayAcrossARestart(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	first := &fakeTimetableCleanup{}
	runTimetableTick(catchUpScheduler(runs, clock, first))
	require.Equal(t, 1, first.cleanupCalls, "the configured minute runs the cleanup")

	// A new process starts in the same minute and the next one: its memory is
	// empty, the stored run still holds.
	restarted := &fakeTimetableCleanup{}
	s := catchUpScheduler(runs, clock, restarted)
	runTimetableTick(s)
	clock.Set(berlin(14, 2, 1))
	runTimetableTick(s)

	assert.Equal(t, 0, restarted.cleanupCalls, "a restart must not run the cleanup a second time on the same day")
}

func TestFailedNightlyCleanupKeepsTheLastRunAndIsCaughtUp(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(14, 2, 0)}
	cleanup := &fakeTimetableCleanup{err: errors.New("database unavailable")}
	s := catchUpScheduler(runs, clock, cleanup)

	runTimetableTick(s)
	require.Equal(t, 1, cleanup.cleanupCalls)
	last, _ := runs.last("timetable-cleanup")
	assert.Equal(t, berlin(13, 2, 0), last, "a failed run must not count as succeeded")

	// The next minutes are past the day: the run is due again, but a failure
	// is retried only after the retry interval.
	clock.Set(berlin(14, 2, 1))
	runTimetableTick(s)
	assert.Equal(t, 1, cleanup.cleanupCalls, "a failed run is not retried every minute")

	cleanup.mu.Lock()
	cleanup.err = nil
	cleanup.mu.Unlock()
	clock.Set(berlin(14, 3, 0))
	runTimetableTick(s)

	assert.Equal(t, 2, cleanup.cleanupCalls, "the failed run is caught up after the retry interval")
	last, _ = runs.last("timetable-cleanup")
	assert.Equal(t, berlin(14, 3, 0), last)
}

func TestNightlyCleanupWithoutAStoredRunWaitsForItsMinute(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	clock := &cleanupClock{now: berlin(14, 10, 0)}
	cleanup := &fakeTimetableCleanup{}
	s := catchUpScheduler(runs, clock, cleanup)

	runTimetableTick(s)
	assert.Equal(t, 0, cleanup.cleanupCalls, "a school without a stored run waits for its configured minute")

	clock.Set(berlin(15, 2, 0))
	runTimetableTick(s)
	assert.Equal(t, 1, cleanup.cleanupCalls)
	_, ok := runs.last("timetable-cleanup")
	assert.True(t, ok)
}

type recordedRetention struct {
	kind    string
	outcome RetentionOutcome
}

func TestNightlyCleanupRunsTheRetentionStepsAndLogsWhatTheyDeleted(t *testing.T) {
	t.Parallel()

	var logged []recordedRetention
	runs := newFakeCleanupRuns()
	s := unitScheduler(&Scheduler{
		logger:         slog.Default(),
		cleanupService: &mockCleanupService{cleanupResult: &studentpresence.CleanupResult{Success: true}},
		cleanupRuns:    runs,
		cleanupClock:   func() time.Time { return berlin(14, 2, 0) },
		settings: &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: "02:00"},
			intValues:    map[string]int{settingDataCleanupTimeoutMinutes: 30},
		},
		retention: mustRetentionRun(t, []RetentionStep{
			{Kind: "first_retention", Run: func(context.Context) (RetentionOutcome, error) {
				return RetentionOutcome{Deleted: 3, RetentionDays: 90}, nil
			}},
			{Kind: "second_retention", Run: func(context.Context) (RetentionOutcome, error) {
				return RetentionOutcome{RetentionDays: 365}, nil
			}},
		}, func(_ context.Context, kind string, outcome RetentionOutcome) error {
			logged = append(logged, recordedRetention{kind: kind, outcome: outcome})
			return nil
		}),
	})

	s.checkAndRunCleanup(context.Background(), &ScheduledTask{Name: "visit-cleanup"})

	assert.Equal(t, []recordedRetention{{kind: "first_retention", outcome: RetentionOutcome{Deleted: 3, RetentionDays: 90}}}, logged,
		"only a step that deleted something writes a deletion-log entry")
	_, ok := runs.last("visit-cleanup")
	assert.True(t, ok)
}

func TestFailingRetentionStepFailsTheNightlyRunOfTheSchool(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	laterStepRan := false
	s := unitScheduler(&Scheduler{
		logger:         slog.Default(),
		cleanupService: &mockCleanupService{cleanupResult: &studentpresence.CleanupResult{Success: true}},
		cleanupRuns:    runs,
		cleanupClock:   func() time.Time { return berlin(14, 2, 0) },
		settings: &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: "02:00"},
			intValues:    map[string]int{settingDataCleanupTimeoutMinutes: 30},
		},
		retention: mustRetentionRun(t, []RetentionStep{
			{Kind: "failing_retention", Run: func(context.Context) (RetentionOutcome, error) {
				return RetentionOutcome{}, errors.New("delete failed")
			}},
			{Kind: "later_retention", Run: func(context.Context) (RetentionOutcome, error) {
				laterStepRan = true
				return RetentionOutcome{}, nil
			}},
		}, func(context.Context, string, RetentionOutcome) error { return nil }),
	})

	s.checkAndRunCleanup(context.Background(), &ScheduledTask{Name: "visit-cleanup"})

	assert.False(t, laterStepRan, "a failing step stops the run of the school")
	_, ok := runs.last("visit-cleanup")
	assert.False(t, ok, "a run with a failing step must stay due")
}

func mustRetentionRun(t *testing.T, steps []RetentionStep, log RetentionLog) RetentionRun {
	t.Helper()
	run, err := NewRetentionRun(steps, log, slog.Default())
	require.NoError(t, err)
	return run
}

func TestRetentionRunRefusesStepsWithoutADeletionLog(t *testing.T) {
	t.Parallel()

	step := RetentionStep{Kind: "some_retention", Run: func(context.Context) (RetentionOutcome, error) {
		return RetentionOutcome{}, nil
	}}

	_, err := NewRetentionRun([]RetentionStep{step}, nil, nil)
	require.ErrorContains(t, err, "deletion log")
	_, err = NewRetentionRun([]RetentionStep{{Kind: "no_run"}}, func(context.Context, string, RetentionOutcome) error { return nil }, nil)
	require.ErrorContains(t, err, "kind and a run")
}

func TestWorkerRefusesToRunWithoutTheCleanupRunLedger(t *testing.T) {
	t.Parallel()

	deps := minimalWorkerDependencies(t)
	deps.CleanupRuns = nil

	require.ErrorContains(t, validateWorkerDependencies(deps), "cleanup run ledger")
}

func TestNightlyCleanupFollowsATimeMovedLaterInTheDay(t *testing.T) {
	t.Parallel()

	runs := newFakeCleanupRuns()
	runs.runs["timetable-cleanup"] = berlin(13, 2, 0)
	clock := &cleanupClock{now: berlin(13, 22, 0)}
	cleanup := &fakeTimetableCleanup{}
	s := unitScheduler(&Scheduler{
		logger:           slog.Default(),
		timetableCleanup: cleanup,
		cleanupRuns:      runs,
		cleanupClock:     clock.Now,
		settings: &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: "22:00"},
		},
	})

	runTimetableTick(s)
	assert.Equal(t, 0, cleanup.cleanupCalls, "the run of the day already happened at the old time")

	clock.Set(berlin(14, 2, 1))
	runTimetableTick(s)
	assert.Equal(t, 0, cleanup.cleanupCalls, "no scheduled run was missed, so nothing is caught up at the old time")

	clock.Set(berlin(14, 22, 0))
	runTimetableTick(s)
	assert.Equal(t, 1, cleanup.cleanupCalls, "the school's new time runs the cleanup")
}
