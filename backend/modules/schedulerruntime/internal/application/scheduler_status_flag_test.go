package application

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScheduleStatusFlagClearTask_DisabledByEnvVar — the new env kill-switch
// stops the status-flag task from registering, matching the pattern of every
// other per-tenant scheduler task.
func TestScheduleStatusFlagClearTask_DisabledByEnvVar(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
		s.getenv = testEnv("STATUS_FLAG_CLEAR_ENABLED", "false")
		s.scheduleStatusFlagClearTask()
		synctest.Wait()

		s.mu.RLock()
		_, exists := s.tasks["status-flag-clear"]
		s.mu.RUnlock()
		assert.False(t, exists, "task must not register when env var is set to false")
	})
}

// statusFlagArchives records what the status-flag run asks the Care Plan
// archive for. The status and source the root binds for each flag are
// covered in roots/owners/scheduler_presence_ports_test.go, the archive's
// SQL (status day of the flag, kept source, carried day) in
// roots/owners/status_flag_archive_test.go.
type statusFlagArchives struct {
	mu    sync.Mutex
	calls []statusFlagArchive
	err   error
}

type statusFlagArchive struct {
	tenantID      int64
	inTransaction bool
	flagColumn    string
	sinceColumn   string
	carry         bool
	before        time.Time
	date          calendar.Date
}

func (a *statusFlagArchives) record(ctx context.Context, call statusFlagArchive) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, call.inTransaction = testpkg.TransactionFromContext(ctx)
	call.tenantID = testpkg.TenantIDFromContext(ctx)
	a.calls = append(a.calls, call)
	if a.err != nil {
		return 0, a.err
	}
	return 1, nil
}

func (a *statusFlagArchives) ArchiveEndOfDayStatusFlag(ctx context.Context, flagColumn, sinceColumn string, before time.Time, date calendar.Date, _ time.Time) (int64, error) {
	return a.record(ctx, statusFlagArchive{flagColumn: flagColumn, sinceColumn: sinceColumn, before: before, date: date})
}

func (a *statusFlagArchives) CarryStatusFlag(ctx context.Context, flagColumn, sinceColumn string, date calendar.Date, _ time.Time) (int64, error) {
	return a.record(ctx, statusFlagArchive{flagColumn: flagColumn, sinceColumn: sinceColumn, carry: true, date: date})
}

func (a *statusFlagArchives) flagColumns() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	columns := make([]string, 0, len(a.calls))
	for _, call := range a.calls {
		columns = append(columns, call.flagColumn)
	}
	return columns
}

func (a *statusFlagArchives) take() []statusFlagArchive {
	a.mu.Lock()
	defer a.mu.Unlock()
	calls := a.calls
	a.calls = nil
	return calls
}

// berlinClock is a scheduler clock the test moves.
type berlinClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *berlinClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *berlinClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

func berlinTime(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, calendar.Berlin)
}

func endOfDayModes() *fakeStatusFlagSettings {
	return &fakeStatusFlagSettings{overrides: map[string]string{
		"operations.sick_clear_mode":    "end_of_day",
		"operations.excused_clear_mode": "end_of_day",
	}}
}

// fakeStatusFlagSettings scripts the scheduler's SettingsResolver for the
// status-flag task. Keys resolve in the order the task reads them: clear time,
// sick_clear_mode, excused_clear_mode.
type fakeStatusFlagSettings struct {
	overrides map[string]string
}

func (f *fakeStatusFlagSettings) ResolveString(_ context.Context, key string) (string, error) {
	return f.overrides[key], nil
}

func (f *fakeStatusFlagSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (f *fakeStatusFlagSettings) ResolveInt(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (f *fakeStatusFlagSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	_, ok := f.overrides[key]
	return ok, nil
}

// TestStatusFlagClearDue pins the two status-day boundaries on the Berlin
// calendar (#3729, #6291): the fixed end of the day at 18:00 and midnight.
// A school is due once a boundary has passed since its last run, so a missed
// boundary is caught up at the next poll; both days of the daylight saving
// change keep 18:00 and midnight on the wall clock.
func TestStatusFlagClearDue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		now      time.Time
		boundary time.Time
	}{
		{name: "17:59", now: berlinTime(2026, time.September, 30, 17, 59), boundary: berlinTime(2026, time.September, 30, 0, 0)},
		{name: "18:00", now: berlinTime(2026, time.September, 30, 18, 0), boundary: berlinTime(2026, time.September, 30, 18, 0)},
		{name: "23:59", now: berlinTime(2026, time.September, 30, 23, 59), boundary: berlinTime(2026, time.September, 30, 18, 0)},
		{name: "midnight", now: berlinTime(2026, time.October, 1, 0, 0), boundary: berlinTime(2026, time.October, 1, 0, 0)},
		{name: "spring forward 18:00 CEST", now: berlinTime(2026, time.March, 29, 18, 0), boundary: time.Date(2026, time.March, 29, 16, 0, 0, 0, time.UTC)},
		{name: "spring forward 03:30 CEST", now: berlinTime(2026, time.March, 29, 3, 30), boundary: time.Date(2026, time.March, 28, 23, 0, 0, 0, time.UTC)},
		{name: "fall back 18:00 CET", now: berlinTime(2026, time.October, 25, 18, 0), boundary: time.Date(2026, time.October, 25, 17, 0, 0, 0, time.UTC)},
		{name: "fall back 02:30 CET", now: time.Date(2026, time.October, 25, 1, 30, 0, 0, time.UTC), boundary: time.Date(2026, time.October, 24, 22, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, tc.boundary.Equal(statusFlagBoundary(tc.now)), "boundary %s, want %s", statusFlagBoundary(tc.now), tc.boundary)
			assert.True(t, statusFlagClearDue(tc.now, time.Time{}, false), "a school without a run in this process is due")
			assert.True(t, statusFlagClearDue(tc.now, tc.boundary.Add(-time.Second), true), "a run before the boundary leaves it due")
			assert.False(t, statusFlagClearDue(tc.now, tc.boundary, true), "a run at the boundary covers it")
		})
	}
}

// TestStatusFlagArchiveWindow: after 18:00 every flag reported so far
// belongs to an ended day; before 18:00 only the flags of earlier days, and a
// flag without a timestamp is put on the latest ended day.
func TestStatusFlagArchiveWindow(t *testing.T) {
	t.Parallel()
	evening := berlinTime(2026, time.October, 25, 18, 7)
	before, date := statusFlagArchiveWindow(evening)
	assert.True(t, before.Equal(evening))
	assert.Equal(t, calendar.NewDate(2026, time.October, 25), date)

	morning := berlinTime(2026, time.October, 26, 8, 0)
	before, date = statusFlagArchiveWindow(morning)
	assert.True(t, before.Equal(calendar.NewDate(2026, time.October, 26).BerlinMidnight()))
	assert.Equal(t, calendar.NewDate(2026, time.October, 25), date)
}

// TestStatusFlagClearCatchesUpAMissedEndOfDay — the Worker is down from
// 17:55 until 08:00 the next morning. The first poll catches the missed
// 18:00 run up and archives the flags of the ended day only: the archive
// window ends at midnight, so a report of the new morning stays, and the
// ended day is the date for a flag without a timestamp. The next poll in the
// same window runs nothing.
func TestStatusFlagClearCatchesUpAMissedEndOfDay(t *testing.T) {
	t.Parallel()
	clock := &berlinClock{now: berlinTime(2026, time.November, 10, 17, 54)}
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{studentStatusDayRepo: archives, settings: endOfDayModes(), now: clock.Now, logger: slog.Default()})
	task := &ScheduledTask{Name: "status-flag-clear"}

	s.checkAndRunStatusFlagClear(context.Background(), task)
	require.Len(t, archives.take(), 2, "the first poll of the process runs once")

	clock.set(berlinTime(2026, time.November, 11, 8, 0))
	s.checkAndRunStatusFlagClear(context.Background(), task)
	calls := archives.take()
	require.Len(t, calls, 2, "the missed end of the day is caught up")
	for _, call := range calls {
		assert.False(t, call.carry)
		assert.True(t, call.before.Equal(calendar.NewDate(2026, time.November, 11).BerlinMidnight()), "only flags of the ended day: %s", call.before)
		assert.Equal(t, calendar.NewDate(2026, time.November, 10), call.date, "the ended day, not the day of the run")
	}

	clock.set(berlinTime(2026, time.November, 11, 8, 1))
	s.checkAndRunStatusFlagClear(context.Background(), task)
	assert.Empty(t, archives.take(), "a boundary runs once")
}

// TestStatusFlagClearArchivesAnEveningReportAtMidnight — a report after the
// 18:00 run would otherwise stay live through the next day. The midnight run
// archives it onto its own day: every flag reported before midnight.
func TestStatusFlagClearArchivesAnEveningReportAtMidnight(t *testing.T) {
	t.Parallel()
	clock := &berlinClock{now: berlinTime(2026, time.November, 10, 18, 0)}
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{studentStatusDayRepo: archives, settings: endOfDayModes(), now: clock.Now, logger: slog.Default()})
	task := &ScheduledTask{Name: "status-flag-clear"}

	s.checkAndRunStatusFlagClear(context.Background(), task)
	calls := archives.take()
	require.Len(t, calls, 2)
	assert.True(t, calls[0].before.Equal(berlinTime(2026, time.November, 10, 18, 0)))
	assert.Equal(t, calendar.NewDate(2026, time.November, 10), calls[0].date)

	clock.set(berlinTime(2026, time.November, 10, 19, 30))
	s.checkAndRunStatusFlagClear(context.Background(), task)
	assert.Empty(t, archives.take(), "nothing runs between the boundaries")

	clock.set(berlinTime(2026, time.November, 11, 0, 0))
	s.checkAndRunStatusFlagClear(context.Background(), task)
	calls = archives.take()
	require.Len(t, calls, 2, "midnight is a boundary")
	assert.True(t, calls[0].before.Equal(calendar.NewDate(2026, time.November, 11).BerlinMidnight()))
	assert.Equal(t, calendar.NewDate(2026, time.November, 10), calls[0].date)
}

// TestStatusFlagClearCarriesFlagsThatOutlastTheirDay — with manual or
// next_checkin the flag stays set; the run lets it stand for today, so the
// views that read the day's rows show what the flag shows.
func TestStatusFlagClearCarriesFlagsThatOutlastTheirDay(t *testing.T) {
	t.Parallel()
	clock := &berlinClock{now: berlinTime(2026, time.November, 11, 0, 0)}
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{
		studentStatusDayRepo: archives, now: clock.Now, logger: slog.Default(),
		settings: &fakeStatusFlagSettings{overrides: map[string]string{
			"operations.sick_clear_mode":    "manual",
			"operations.excused_clear_mode": "next_checkin",
		}},
	})

	s.checkAndRunStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	calls := archives.take()
	require.Len(t, calls, 2)
	for _, call := range calls {
		assert.True(t, call.carry)
		assert.Equal(t, calendar.NewDate(2026, time.November, 11), call.date)
	}
	assert.Equal(t, []string{"sick", "excused"}, []string{calls[0].flagColumn, calls[1].flagColumn})
}

// TestCheckAndRunStatusFlagClear_SkipsACoveredBoundary — once every school
// has run for the latest boundary, a poll reaches no school and no archive.
func TestCheckAndRunStatusFlagClear_SkipsACoveredBoundary(t *testing.T) {
	t.Parallel()
	now := berlinTime(2026, time.November, 10, 12, 0)
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{studentStatusDayRepo: archives, settings: endOfDayModes(), now: func() time.Time { return now }})
	s.statusFlagPass.Store(statusFlagBoundary(now).Unix())

	s.checkAndRunStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Empty(t, archives.take(), "a covered boundary runs nothing")
	_, present := s.lastStatusFlagClear.Load(schedulerUnitTenantID)
	assert.False(t, present)
}

// TestRunStatusFlagClear_PassThatReachesNoSchoolStaysDue — when the
// school settings cannot be loaded, no school runs; the boundary stays due
// and the next poll runs it.
func TestRunStatusFlagClear_PassThatReachesNoSchoolStaysDue(t *testing.T) {
	t.Parallel()
	now := berlinTime(2026, time.November, 11, 0, 0)
	archives := &statusFlagArchives{}
	failing := true
	s := unitScheduler(&unitSchedulerDependencies{studentStatusDayRepo: archives, settings: endOfDayModes(), now: func() time.Time { return now }, logger: slog.Default()})
	loader := s.minuteSnapshotLoader
	s.minuteSnapshotLoader = func(ctx context.Context) (*schedulerMinuteSnapshot, error) {
		if failing {
			return nil, errors.New("settings unavailable")
		}
		return loader(ctx)
	}
	task := &ScheduledTask{Name: "status-flag-clear"}

	s.checkAndRunStatusFlagClear(context.Background(), task)
	assert.Empty(t, archives.take())
	assert.Zero(t, s.statusFlagPass.Load(), "a pass that reached no school is not done")

	failing = false
	s.minuteSnapshotLoad = nil
	s.checkAndRunStatusFlagClear(context.Background(), task)
	assert.Len(t, archives.take(), 2, "the next poll runs the boundary")
	assert.Equal(t, now.Unix(), s.statusFlagPass.Load())
}

// TestRunStatusFlagClear_MissingArchiveKeepsTheSchoolDue — without the Care
// Plan archive the run fails for the school: it is not marked and the pass
// stays open, so the next poll retries.
func TestRunStatusFlagClear_MissingArchiveKeepsTheSchoolDue(t *testing.T) {
	t.Parallel()
	now := berlinTime(2026, time.November, 10, 18, 0)
	s := unitScheduler(&unitSchedulerDependencies{settings: endOfDayModes(), now: func() time.Time { return now }})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"}, now)

	_, present := s.lastStatusFlagClear.Load(schedulerUnitTenantID)
	assert.False(t, present, "a failed school is not marked")
	assert.Zero(t, s.statusFlagPass.Load(), "a failed school keeps the pass open")
}

// TestRunStatusFlagClearTaskPolling_StopsOnDone — the polling goroutine must
// exit promptly when the scheduler's done channel is closed, otherwise Stop()
// would hang. Uses synctest to make the waitUntilNextMinute sleep instant.
func TestRunStatusFlagClearTaskPolling_StopsOnDone(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&unitSchedulerDependencies{
			done:     make(chan struct{}),
			settings: &fakeStatusFlagSettings{overrides: map[string]string{}}})

		task := &ScheduledTask{Name: "status-flag-clear"}
		s.wg.Go(func() {
			s.runStatusFlagClearTaskPolling(task)
		})

		// Let the goroutine settle past the immediate-run branch.
		synctest.Wait()
		close(s.done)
		// Ensure the goroutine exits.
		s.wg.Wait()
	})
}

// TestCheckAndRunStatusFlagClear_EndToEnd_ClearsBothFlags runs the end-of-day
// job at 18:00 with both clear modes on end_of_day. Both flags reach the
// archive inside the school's tenant transaction, and the school and the pass
// are marked once that transaction commits.
func TestCheckAndRunStatusFlagClear_EndToEnd_ClearsBothFlags(t *testing.T) {
	t.Parallel()
	now := berlinTime(2026, time.November, 10, 18, 0)
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{studentStatusDayRepo: archives, settings: endOfDayModes(), now: func() time.Time { return now }, logger: slog.Default()})

	s.checkAndRunStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Equal(t, []string{"sick", "excused"}, archives.flagColumns())
	for _, call := range archives.calls {
		assert.Equal(t, schedulerUnitTenantID, call.tenantID, "the archive runs for the school")
		assert.True(t, call.inTransaction, "the archive runs inside the school's tenant transaction")
		assert.False(t, call.carry)
	}
	last, ok := s.lastStatusFlagClear.Load(schedulerUnitTenantID)
	require.True(t, ok)
	assert.True(t, now.Equal(last.(time.Time)))
	assert.Equal(t, now.Unix(), s.statusFlagPass.Load())
}

// TestCheckAndRunStatusFlagClear_EndToEnd_RespectsModeSetting confirms that
// with sick_clear_mode = end_of_day and excused_clear_mode = next_checkin the
// scheduler archives sick and carries excused.
func TestCheckAndRunStatusFlagClear_EndToEnd_RespectsModeSetting(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.sick_clear_mode":    "end_of_day",
				"operations.excused_clear_mode": "next_checkin",
			},
		},
		logger: slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"}, berlinTime(2026, time.November, 10, 18, 0))

	require.Len(t, archives.calls, 2)
	assert.False(t, archives.calls[0].carry, "sick is archived")
	assert.True(t, archives.calls[1].carry, "excused is not cleared when excused_clear_mode != end_of_day")
}

// TestCheckAndRunStatusFlagClear_EndToEnd_ClearsUnconfiguredModes covers a
// school without its own clear-mode values (#3728). The scheduler reads those
// through its own fallback, which must match the registry default
// "end_of_day" pinned by services/config/defaults.TestStatusFlagClearMode_Defaults;
// otherwise an ended sick note would linger with nobody clearing it.
func TestCheckAndRunStatusFlagClear_EndToEnd_ClearsUnconfiguredModes(t *testing.T) {
	t.Parallel()
	require.Equal(t, "end_of_day", clearModeEndOfDay, "the fallback is the registry default")
	archives := &statusFlagArchives{}
	s := unitScheduler(&unitSchedulerDependencies{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{},
		},
		logger: slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"}, berlinTime(2026, time.November, 10, 18, 0))

	assert.Equal(t, []string{"sick", "excused"}, archives.flagColumns(),
		"both flags must be cleared at end of day when the school kept the default")
	for _, call := range archives.calls {
		assert.False(t, call.carry)
	}
}

// TestRunStatusFlagClear_ArchiveFailureKeepsTheDayOpen — a failed archive
// leaves the school unmarked so the next poll retries it.
func TestRunStatusFlagClear_ArchiveFailureKeepsTheDayOpen(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{err: errors.New("archive unavailable")}
	s := unitScheduler(&unitSchedulerDependencies{
		studentStatusDayRepo: archives,
		settings:             endOfDayModes(),
		logger:               slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"}, berlinTime(2026, time.November, 10, 18, 0))

	assert.Len(t, archives.calls, 2)
	_, present := s.lastStatusFlagClear.Load(schedulerUnitTenantID)
	assert.False(t, present)
	assert.Zero(t, s.statusFlagPass.Load())
}
