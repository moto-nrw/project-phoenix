package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

// TestClearStatusFlag_ClearsAllMatchingRowsWithinTenant — a bulk UPDATE driven
// by the tenant transaction clears the flag column and wipes the timestamp
// only for rows where the flag is currently true.
func TestClearStatusFlag_ClearsSickFlag(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	s := unitScheduler(&Scheduler{db: db, studentStatusDayRepo: statusDayRepository(t, db)})

	// Create two students: one sick, one not, both in tenant 1.
	sickStudent := testpkg.CreateTestStudent(t, db, "Clear", "SickFlag", "1a")
	_ = testpkg.CreateTestStudent(t, db, "Already", "Healthy", "1a")

	// Set the sick flag + timestamp on the first student directly.
	now := time.Now()
	sickTrue := true
	_, err := db.NewUpdate().
		Table("users.students").
		Set("sick = ?", sickTrue).
		Set("sick_since = ?", now).
		Where("id = ?", sickStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Call the scheduler helper inside a tenant tx so the UPDATE lands.
	ctx := testpkg.Ctx(t)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		affected, clearErr := s.clearStatusFlag(txCtx, "sick", "sick_since")
		require.NoError(t, clearErr)
		// We don't assert exact row count — other tests may leave sick rows
		// behind, and the scheduler's real UPDATE runs inside a tenant
		// transaction. Assert >= 1 to prove our target was touched.
		assert.GreaterOrEqual(t, affected, int64(1))
		return nil
	})
	require.NoError(t, err)

	// The target student should now have sick = false and sick_since = NULL.
	reloaded := &userModels.Student{Model: base.Model{ID: sickStudent.ID}}
	err = db.NewSelect().
		Model(reloaded).
		Column("sick", "sick_since").
		Where("id = ?", sickStudent.ID).
		Scan(context.Background())
	require.NoError(t, err)
	if reloaded.Sick != nil {
		assert.False(t, *reloaded.Sick, "sick flag should be cleared")
	}
	assert.Nil(t, reloaded.SickSince, "sick_since timestamp should be NULL")
}

// TestClearStatusFlag_NilDBReturnsError — defensive guard so a misconfigured
// scheduler fails loudly instead of silently no-oping.
func TestClearStatusFlag_NilDBReturnsError(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
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

func statusDayRepository(t *testing.T, db *bun.DB) activeModels.StudentStatusDayOverviewRepository {
	t.Helper()
	factory, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	return factory.StudentStatusDay
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

// TestCheckAndRunStatusFlagClear_SkipsWhenTimeDoesNotMatch — the expensive
// UPDATE must not run when the current clock doesn't match the tenant's
// configured status flag clear time.
func TestCheckAndRunStatusFlagClear_SkipsWhenTimeDoesNotMatch(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	// Set the configured time to a value that cannot equal any real minute.
	// timeMatchesNow returns false, so the task body should short-circuit
	// without attempting any DB work (db is deliberately nil here to prove
	// the short-circuit — any attempted UPDATE would panic).
	s := unitScheduler(&Scheduler{
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.status_flag_clear_time": "99:99",
			},
		}})

	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(context.Background(), task)
	// If we reach here without a nil-db panic the short-circuit worked.
	assert.False(t, task.Running, "task should have reset Running flag")
}

// TestCheckAndRunStatusFlagClear_SkipsWhenClearTimeEmpty guards the defensive
// branch for malformed runtime config. The registered default is 18:00, so
// real tenants should not hit this path through SettingsService.
func TestCheckAndRunStatusFlagClear_SkipsWhenClearTimeEmpty(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := unitScheduler(&Scheduler{
		settings: &fakeStatusFlagSettings{overrides: map[string]string{}}})

	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(context.Background(), task)
	assert.False(t, task.Running)
}

// TestCheckAndRunStatusFlagClear_FiresBothModesWhenTimeMatches — when the
// clock matches status_flag_clear_time and both modes are end_of_day,
// the task enters the clearing branch for both flags. We run without a real
// db, which makes clearStatusFlag return an error — this is the exact path
// we want to cover (error branch) and it also lets us verify the lastRun
// marker is removed so a retry can happen on the next matching minute.
func TestCheckAndRunStatusFlagClear_FiresBothModesWhenTimeMatches(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	now := time.Now()
	// Compute HH:MM matching now; if we are within the final second of a
	// minute, skip to avoid a boundary race.
	if now.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race")
	}
	nowHHMM := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	s := unitScheduler(&Scheduler{
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.status_flag_clear_time": nowHHMM,
				"operations.sick_clear_mode":        "end_of_day",
				"operations.excused_clear_mode":     "end_of_day",
			},
		}})

	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(context.Background(), task)

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

// TestClearStatusFlag_ClearsExcusedFlag — mirror of the sick test to exercise
// the column plumbing for the new excused flag.
func TestClearStatusFlag_ClearsExcusedFlag(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	s := unitScheduler(&Scheduler{db: db, studentStatusDayRepo: statusDayRepository(t, db)})

	excStudent := testpkg.CreateTestStudent(t, db, "Clear", "ExcusedFlag", "1b")

	now := time.Now()
	excTrue := true
	_, err := db.NewUpdate().
		Table("users.students").
		Set("excused = ?", excTrue).
		Set("excused_since = ?", now).
		Where("id = ?", excStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)

	ctx := testpkg.Ctx(t)
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		affected, clearErr := s.clearStatusFlag(txCtx, "excused", "excused_since")
		require.NoError(t, clearErr)
		assert.GreaterOrEqual(t, affected, int64(1))
		return nil
	})
	require.NoError(t, err)

	reloaded := &userModels.Student{Model: base.Model{ID: excStudent.ID}}
	err = db.NewSelect().
		Model(reloaded).
		Column("excused", "excused_since").
		Where("id = ?", excStudent.ID).
		Scan(context.Background())
	require.NoError(t, err)
	if reloaded.Excused != nil {
		assert.False(t, *reloaded.Excused, "excused flag should be cleared")
	}
	assert.Nil(t, reloaded.ExcusedSince, "excused_since timestamp should be NULL")
}

// reloadStudentFlags re-reads a student's sick / excused flags from the
// database. Used by the end-to-end scheduler tests below.
func reloadStudentFlags(t *testing.T, db *bun.DB, studentID int64) (sick, excused bool) {
	t.Helper()
	var row struct {
		Sick    *bool `bun:"sick"`
		Excused *bool `bun:"excused"`
	}
	err := db.NewSelect().
		Table("users.students").
		Column("sick", "excused").
		Where("id = ?", studentID).
		Scan(context.Background(), &row)
	require.NoError(t, err)
	if row.Sick != nil {
		sick = *row.Sick
	}
	if row.Excused != nil {
		excused = *row.Excused
	}
	return sick, excused
}

// TestCheckAndRunStatusFlagClear_EndToEnd_ClearsBothFlags is the full
// end-to-end integration test for the end_of_day path. It wires a real DB
// and a real SchoolRepository into the scheduler, plants a student with
// sick = true AND another with excused = true in tenant 1, configures the
// settings resolver to return operations.status_flag_clear_time =
// "now" with both clear modes set to end_of_day, then invokes
// checkAndRunStatusFlagClear and asserts both flags get wiped by the
// bulk UPDATE that the scheduler runs inside a tenant transaction. This
// covers the glue that the unit tests split apart: time-match check →
// forEachTenantSettings iteration → tenant tx → bulk UPDATE → RLS-scoped
// row visibility.
func TestCheckAndRunStatusFlagClear_EndToEnd_ClearsBothFlags(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	now := time.Now()
	if now.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}
	nowHHMM := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	// Plant a sick student and an excused student in tenant 1.
	sickStudent := testpkg.CreateTestStudent(t, db, "E2E", "SickClear", "e1")
	excusedStudent := testpkg.CreateTestStudent(t, db, "E2E", "ExcusedClear", "e2")

	flagTrue := true
	ts := now
	_, err := db.NewUpdate().
		Table("users.students").
		Set("sick = ?", flagTrue).
		Set("sick_since = ?", ts).
		Where("id = ?", sickStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewUpdate().
		Table("users.students").
		Set("excused = ?", flagTrue).
		Set("excused_since = ?", ts).
		Where("id = ?", excusedStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Fully wired scheduler: real db, real school repo (so
	// forEachTenantSettings iterates every active tenant), fake settings
	// resolver that pins the clock + both modes to what the task expects.
	s := isolatedUnitScheduler(t, db, &Scheduler{
		db:                   db,
		schoolRepo:           platformRepo.NewSchoolRepository(db),
		studentStatusDayRepo: statusDayRepository(t, db),
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.status_flag_clear_time": nowHHMM,
				"operations.sick_clear_mode":        "end_of_day",
				"operations.excused_clear_mode":     "end_of_day",
			},
		},
		logger: slog.Default()})

	task := &ScheduledTask{Name: "status-flag-clear"}

	s.checkAndRunStatusFlagClear(context.Background(), task)

	gotSick, _ := reloadStudentFlags(t, db, sickStudent.ID)
	_, gotExcused := reloadStudentFlags(t, db, excusedStudent.ID)
	assert.False(t, gotSick, "sick flag should be cleared by the end-of-day job")
	assert.False(t, gotExcused, "excused flag should be cleared by the end-of-day job")
}

// TestCheckAndRunStatusFlagClear_EndToEnd_RespectsModeSetting confirms that
// when only sick_clear_mode = end_of_day is set and excused_clear_mode is
// next_checkin, the scheduler clears sick but leaves excused alone. This
// exercises the per-mode gate in checkAndRunStatusFlagClear that the
// unit-level test could not reach (because it ran with a nil db).
func TestCheckAndRunStatusFlagClear_EndToEnd_RespectsModeSetting(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	now := time.Now()
	if now.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}
	nowHHMM := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	sickStudent := testpkg.CreateTestStudent(t, db, "E2E", "SickOnly", "m1")
	excusedStudent := testpkg.CreateTestStudent(t, db, "E2E", "ExcusedOnly", "m2")

	flagTrue := true
	ts := now
	_, err := db.NewUpdate().
		Table("users.students").
		Set("sick = ?", flagTrue).
		Set("sick_since = ?", ts).
		Where("id = ?", sickStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewUpdate().
		Table("users.students").
		Set("excused = ?", flagTrue).
		Set("excused_since = ?", ts).
		Where("id = ?", excusedStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)

	s := isolatedUnitScheduler(t, db, &Scheduler{
		db:                   db,
		schoolRepo:           platformRepo.NewSchoolRepository(db),
		studentStatusDayRepo: statusDayRepository(t, db),
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.status_flag_clear_time": nowHHMM,
				"operations.sick_clear_mode":        "end_of_day",
				"operations.excused_clear_mode":     "next_checkin",
			},
		},
		logger: slog.Default()})

	task := &ScheduledTask{Name: "status-flag-clear"}

	s.checkAndRunStatusFlagClear(context.Background(), task)

	gotSick, _ := reloadStudentFlags(t, db, sickStudent.ID)
	_, gotExcused := reloadStudentFlags(t, db, excusedStudent.ID)
	assert.False(t, gotSick, "sick must be cleared when sick_clear_mode = end_of_day")
	assert.True(t, gotExcused, "excused must NOT be cleared when excused_clear_mode != end_of_day")
}

// TestCheckAndRunStatusFlagClear_EndToEnd_DoesNothingWhenTimeDoesNotMatch
// is the negative case: a correctly-configured end_of_day mode must not
// fire the UPDATE when the clock is not at the configured minute. This
// proves the timeMatchesNow guard short-circuits before touching rows
// and is the only thing standing between "job fires harmlessly" and
// "job clears flags at the wrong time of day."
func TestCheckAndRunStatusFlagClear_EndToEnd_DoesNothingWhenTimeDoesNotMatch(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	sickStudent := testpkg.CreateTestStudent(t, db, "E2E", "NoFire", "n1")

	flagTrue := true
	now := time.Now()
	_, err := db.NewUpdate().
		Table("users.students").
		Set("sick = ?", flagTrue).
		Set("sick_since = ?", now).
		Where("id = ?", sickStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Configure a clearly past time so timeMatchesNow always returns false.
	s := isolatedUnitScheduler(t, db, &Scheduler{
		db:                   db,
		schoolRepo:           platformRepo.NewSchoolRepository(db),
		studentStatusDayRepo: statusDayRepository(t, db),
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.status_flag_clear_time": "25:99",
				"operations.sick_clear_mode":        "end_of_day",
				"operations.excused_clear_mode":     "end_of_day",
			},
		},
		logger: slog.Default()})

	task := &ScheduledTask{Name: "status-flag-clear"}

	s.checkAndRunStatusFlagClear(context.Background(), task)

	gotSick, _ := reloadStudentFlags(t, db, sickStudent.ID)
	assert.True(t, gotSick, "sick must NOT be cleared when clock does not match configured time")
}
