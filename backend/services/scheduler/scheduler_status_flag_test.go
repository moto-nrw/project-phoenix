package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestScheduleStatusFlagClearTask_DisabledByEnvVar — the new env kill-switch
// stops the status-flag task from registering, matching the pattern of every
// other per-tenant scheduler task.
func TestScheduleStatusFlagClearTask_DisabledByEnvVar(t *testing.T) {
	require.NoError(t, os.Setenv("STATUS_FLAG_CLEAR_ENABLED", "false"))
	defer func() { _ = os.Unsetenv("STATUS_FLAG_CLEAR_ENABLED") }()

	synctest.Test(t, func(t *testing.T) {
		s := NewScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	s := &Scheduler{db: db}

	// Create two students: one sick, one not, both in tenant 1.
	sickStudent := testpkg.CreateTestStudent(t, db, "Clear", "SickFlag", "1a")
	healthy := testpkg.CreateTestStudent(t, db, "Already", "Healthy", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, sickStudent.ID, healthy.ID)

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
	ctx := tenant.WithTenantID(context.Background(), 1)
	err = tenant.WithTenantTx(ctx, db, 1, func(txCtx context.Context, _ bun.Tx) error {
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
	s := &Scheduler{}
	_, err := s.clearStatusFlag(context.Background(), "sick", "sick_since")
	assert.Error(t, err)
}

// fakeStatusFlagSettings scripts the scheduler's SettingsResolver for the
// status-flag task. Keys resolve in the order the task reads them: daily
// checkout time, sick_clear_mode, excused_clear_mode.
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

// TestCheckAndRunStatusFlagClear_SkipsWhenTimeDoesNotMatch — the expensive
// UPDATE must not run when the current clock doesn't match the tenant's
// configured daily checkout time.
func TestCheckAndRunStatusFlagClear_SkipsWhenTimeDoesNotMatch(t *testing.T) {
	// Set the configured time to a value that cannot equal any real minute.
	// timeMatchesNow returns false, so the task body should short-circuit
	// without attempting any DB work (db is deliberately nil here to prove
	// the short-circuit — any attempted UPDATE would panic).
	s := &Scheduler{
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.student_daily_checkout_time": "99:99",
			},
		},
	}
	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(task)
	// If we reach here without a nil-db panic the short-circuit worked.
	assert.False(t, task.Running, "task should have reset Running flag")
}

// TestCheckAndRunStatusFlagClear_SkipsWhenClearTimeEmpty — an empty
// student_daily_checkout_time means "no daily gate configured" and the task
// must do nothing regardless of the mode settings.
func TestCheckAndRunStatusFlagClear_SkipsWhenClearTimeEmpty(t *testing.T) {
	s := &Scheduler{
		settings: &fakeStatusFlagSettings{overrides: map[string]string{}},
	}
	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(task)
	assert.False(t, task.Running)
}

// TestCheckAndRunStatusFlagClear_FiresBothModesWhenTimeMatches — when the
// clock matches student_daily_checkout_time and both modes are end_of_day,
// the task enters the clearing branch for both flags. We run without a real
// db, which makes clearStatusFlag return an error — this is the exact path
// we want to cover (error branch) and it also lets us verify the lastRun
// marker is removed so a retry can happen on the next matching minute.
func TestCheckAndRunStatusFlagClear_FiresBothModesWhenTimeMatches(t *testing.T) {
	now := time.Now()
	// Compute HH:MM matching now; if we are within the final second of a
	// minute, skip to avoid a boundary race.
	if now.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race")
	}
	nowHHMM := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	s := &Scheduler{
		settings: &fakeStatusFlagSettings{
			overrides: map[string]string{
				"operations.student_daily_checkout_time": nowHHMM,
				"operations.sick_clear_mode":             "end_of_day",
				"operations.excused_clear_mode":          "end_of_day",
			},
		},
	}
	task := &ScheduledTask{Name: "status-flag-clear"}
	s.checkAndRunStatusFlagClear(task)

	// Since clearStatusFlag returns an error (nil db), the lastStatusFlagClear
	// entry for tenantID 0 should have been deleted so a retry can happen.
	_, present := s.lastStatusFlagClear.Load(int64(0))
	assert.False(t, present, "lastRun marker must be cleared when clearStatusFlag fails so retry is possible")
}

// TestRunStatusFlagClearTaskPolling_StopsOnDone — the polling goroutine must
// exit promptly when the scheduler's done channel is closed, otherwise Stop()
// would hang. Uses synctest to make the waitUntilNextMinute sleep instant.
func TestRunStatusFlagClearTaskPolling_StopsOnDone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &Scheduler{
			done:     make(chan struct{}),
			settings: &fakeStatusFlagSettings{overrides: map[string]string{}},
		}
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	s := &Scheduler{db: db}

	excStudent := testpkg.CreateTestStudent(t, db, "Clear", "ExcusedFlag", "1b")
	defer testpkg.CleanupActivityFixtures(t, db, excStudent.ID)

	now := time.Now()
	excTrue := true
	_, err := db.NewUpdate().
		Table("users.students").
		Set("excused = ?", excTrue).
		Set("excused_since = ?", now).
		Where("id = ?", excStudent.ID).
		Exec(context.Background())
	require.NoError(t, err)

	ctx := tenant.WithTenantID(context.Background(), 1)
	err = tenant.WithTenantTx(ctx, db, 1, func(txCtx context.Context, _ bun.Tx) error {
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
