package scheduler

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

// statusFlagArchives records what the end-of-day clear asks the Care Plan
// archive for. The archive's SQL (flag to status day, flag and timestamp
// cleared, school-scoped rows) is covered in
// modules/careplan/internal/adapters/postgres.
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
	status        string
	date          calendar.Date
	source        string
}

func (a *statusFlagArchives) ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date calendar.Date, _ time.Time, source string) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, inTransaction := testpkg.TransactionFromContext(ctx)
	a.calls = append(a.calls, statusFlagArchive{
		tenantID:      testpkg.TenantIDFromContext(ctx),
		inTransaction: inTransaction,
		flagColumn:    flagColumn,
		sinceColumn:   sinceColumn,
		status:        status,
		date:          date,
		source:        source,
	})
	if a.err != nil {
		return 0, a.err
	}
	return 1, nil
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

// TestClearStatusFlag_ArchivesSickFlag — the sick flag goes to the archive
// with its timestamp column, the sick status, today's date and the
// end-of-day source.
func TestClearStatusFlag_ArchivesSickFlag(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{}
	s := unitScheduler(&Scheduler{studentStatusDayRepo: archives})

	affected, err := s.clearStatusFlag(context.Background(), "sick", "sick_since")

	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	require.Len(t, archives.calls, 1)
	call := archives.calls[0]
	assert.Equal(t, "sick_since", call.sinceColumn)
	assert.Equal(t, "sick", call.status)
	assert.Equal(t, calendar.TodayDate(), call.date)
	assert.Equal(t, "end_of_day", call.source)
}

// TestClearStatusFlag_NilDBReturnsError — defensive guard so a misconfigured
// scheduler fails loudly instead of silently no-oping.
func TestClearStatusFlag_NilDBReturnsError(t *testing.T) {
	t.Parallel()
	s := unitScheduler(&Scheduler{})
	_, err := s.clearStatusFlag(context.Background(), "sick", "sick_since")
	assert.Error(t, err)
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

// TestStatusFlagClearDue pins the fixed end of the day (#3729): the clear
// runs in the 18:00 minute and in no other.
func TestStatusFlagClearDue(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.September, 30).BerlinMidnight()
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{name: "17:59", at: day.Add(17*time.Hour + 59*time.Minute), want: false},
		{name: "18:00", at: day.Add(18 * time.Hour), want: true},
		{name: "18:00:59", at: day.Add(18*time.Hour + 59*time.Second), want: true},
		{name: "18:01", at: day.Add(18*time.Hour + time.Minute), want: false},
		{name: "midnight", at: day, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, statusFlagClearDue(tc.at.In(calendar.Berlin)))
		})
	}
}

// TestCheckAndRunStatusFlagClear_SkipsOutsideEndOfDay — the expensive
// UPDATE must not run outside the fixed end-of-day minute.
func TestCheckAndRunStatusFlagClear_SkipsOutsideEndOfDay(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	if statusFlagClearDue(time.Now()) {
		t.Skip("running inside the end-of-day minute")
	}
	// db is deliberately nil here to prove the short-circuit: any attempted
	// UPDATE would fail and clear nothing, but the settings would be read.
	settings := &fakeStatusFlagSettings{
		overrides: map[string]string{
			"operations.sick_clear_mode":    "end_of_day",
			"operations.excused_clear_mode": "end_of_day",
		},
	}
	s := unitScheduler(&Scheduler{settings: settings})

	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(context.Background(), task)
	assert.False(t, task.Running, "task should not be running")
	_, present := s.lastStatusFlagClear.Load(int64(0))
	assert.False(t, present, "no tenant may be marked as cleared outside the end-of-day minute")
}

// TestRunStatusFlagClear_FiresBothModes — at the end of the day, with both
// modes on end_of_day, the task enters the clearing branch for both flags. We run without a real
// db, which makes clearStatusFlag return an error — this is the exact path
// we want to cover (error branch) and it also lets us verify the lastRun
// marker is removed so a retry can happen on the next matching minute.
func TestRunStatusFlagClear_FiresBothModes(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := unitScheduler(&Scheduler{
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.sick_clear_mode":    "end_of_day",
				"operations.excused_clear_mode": "end_of_day",
			},
		}})

	task := &ScheduledTask{Name: "status-flag-clear"}
	s.runStatusFlagClear(context.Background(), task)

	// Since clearStatusFlag returns an error (nil db), the lastStatusFlagClear
	// entry for tenantID 0 should have been deleted so a retry can happen.
	_, present := s.lastStatusFlagClear.Load(int64(0))
	assert.False(t, present, "lastRun marker must be cleared when clearStatusFlag fails so retry is possible")
}

// TestRunStatusFlagClearTaskPolling_StopsOnDone — the polling goroutine must
// exit promptly when the scheduler's done channel is closed, otherwise Stop()
// would hang. Uses synctest to make the waitUntilNextMinute sleep instant.
func TestRunStatusFlagClearTaskPolling_StopsOnDone(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := unitScheduler(&Scheduler{
			done:     make(chan struct{}),
			settings: &fakeStatusFlagSettings{overrides: map[string]string{}}})

		task := &ScheduledTask{Name: "status-flag-clear"}
		s.wg.Add(1)
		go s.runStatusFlagClearTaskPolling(task)
		// Let the goroutine settle past the immediate-run branch.
		synctest.Wait()
		close(s.done)
		// Ensure the goroutine exits.
		s.wg.Wait()
	})
}

// TestClearStatusFlag_ArchivesExcusedFlag — mirror of the sick test for the
// excused flag's column plumbing.
func TestClearStatusFlag_ArchivesExcusedFlag(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{}
	s := unitScheduler(&Scheduler{studentStatusDayRepo: archives})

	affected, err := s.clearStatusFlag(context.Background(), "excused", "excused_since")

	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	require.Len(t, archives.calls, 1)
	assert.Equal(t, "excused", archives.calls[0].flagColumn)
	assert.Equal(t, "excused_since", archives.calls[0].sinceColumn)
	assert.Equal(t, "excused", archives.calls[0].status)
}

// TestCheckAndRunStatusFlagClear_EndToEnd_ClearsBothFlags runs the end-of-day
// job behind the 18:00 gate with both clear modes on end_of_day. Both flags
// reach the archive inside the school's tenant transaction, and the school is
// marked as cleared for the day once that transaction commits.
func TestCheckAndRunStatusFlagClear_EndToEnd_ClearsBothFlags(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{}
	s := unitScheduler(&Scheduler{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.sick_clear_mode":    "end_of_day",
				"operations.excused_clear_mode": "end_of_day",
			},
		},
		logger: slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Equal(t, []string{"sick", "excused"}, archives.flagColumns())
	for _, call := range archives.calls {
		assert.Equal(t, schedulerUnitTenantID, call.tenantID, "the archive runs for the school")
		assert.True(t, call.inTransaction, "the archive runs inside the school's tenant transaction")
	}
	assert.True(t, wasRunToday(&s.lastStatusFlagClear, schedulerUnitTenantID))
}

// TestCheckAndRunStatusFlagClear_EndToEnd_RespectsModeSetting confirms that
// when only sick_clear_mode = end_of_day is set and excused_clear_mode is
// next_checkin, the scheduler clears sick but leaves excused alone.
func TestCheckAndRunStatusFlagClear_EndToEnd_RespectsModeSetting(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{}
	s := unitScheduler(&Scheduler{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.sick_clear_mode":    "end_of_day",
				"operations.excused_clear_mode": "next_checkin",
			},
		},
		logger: slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Equal(t, []string{"sick"}, archives.flagColumns(),
		"excused must NOT be cleared when excused_clear_mode != end_of_day")
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
	s := unitScheduler(&Scheduler{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{},
		},
		logger: slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Equal(t, []string{"sick", "excused"}, archives.flagColumns(),
		"both flags must be cleared at end of day when the school kept the default")
}

// TestRunStatusFlagClear_ArchiveFailureKeepsTheDayOpen — a failed archive
// leaves the school unmarked so the next run of the day retries it.
func TestRunStatusFlagClear_ArchiveFailureKeepsTheDayOpen(t *testing.T) {
	t.Parallel()
	archives := &statusFlagArchives{err: errors.New("archive unavailable")}
	s := unitScheduler(&Scheduler{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.sick_clear_mode":    "end_of_day",
				"operations.excused_clear_mode": "end_of_day",
			},
		},
		logger: slog.Default()})

	s.runStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Len(t, archives.calls, 2)
	assert.False(t, wasRunToday(&s.lastStatusFlagClear, schedulerUnitTenantID))
}

// TestCheckAndRunStatusFlagClear_EndToEnd_DoesNothingWhenTimeDoesNotMatch
// is the negative case: a correctly-configured end_of_day mode must not
// reach the archive outside the fixed end-of-day minute. This proves the
// statusFlagClearDue guard short-circuits before touching rows.
func TestCheckAndRunStatusFlagClear_EndToEnd_DoesNothingWhenTimeDoesNotMatch(t *testing.T) {
	t.Parallel()
	if statusFlagClearDue(time.Now()) {
		t.Skip("running inside the end-of-day minute")
	}
	archives := &statusFlagArchives{}
	s := unitScheduler(&Scheduler{
		studentStatusDayRepo: archives,
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.sick_clear_mode":    "end_of_day",
				"operations.excused_clear_mode": "end_of_day",
			},
		},
		logger: slog.Default()})

	s.checkAndRunStatusFlagClear(context.Background(), &ScheduledTask{Name: "status-flag-clear"})

	assert.Empty(t, archives.calls, "no flag may be archived outside the end-of-day minute")
}
