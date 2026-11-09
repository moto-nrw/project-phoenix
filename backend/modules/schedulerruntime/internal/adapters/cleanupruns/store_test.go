package cleanupruns

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const jobID = "visit-cleanup"

// serveDB is the least-privilege pool of the Serve root: each transaction
// runs as phoenix_tenant under the school's row-level security.
func serveDB(t *testing.T) *bun.DB {
	t.Helper()
	db := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func lastRun(t *testing.T, db *bun.DB, tenantID int64) (time.Time, bool) {
	t.Helper()
	var (
		at time.Time
		ok bool
	)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		var err error
		at, _, _, err = NewStore().CleanupRun(ctx, jobID)
		ok = !at.IsZero()
		return err
	}))
	return at, ok
}

func record(t *testing.T, db *bun.DB, tenantID int64, at time.Time) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		return NewStore().RecordCleanupSucceeded(ctx, jobID, at)
	}))
}

func TestCleanupRunIsStoredPerSchool(t *testing.T) {
	t.Parallel()
	db := serveDB(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, testpkg.SetupTestDB(t))

	_, ok := lastRun(t, db, school)
	require.False(t, ok, "a school without a run has no stored success")

	succeeded := time.Date(2026, time.October, 14, 2, 0, 0, 0, time.UTC)
	record(t, db, school, succeeded)

	at, ok := lastRun(t, db, school)
	require.True(t, ok)
	assert.True(t, at.Equal(succeeded))
	_, ok = lastRun(t, db, otherSchool)
	assert.False(t, ok, "a run of one school is invisible to another")
}

func TestCleanupRunNeverMovesBack(t *testing.T) {
	t.Parallel()
	db := serveDB(t)
	school := testpkg.Tenant(t)
	later := time.Date(2026, time.October, 14, 2, 0, 0, 0, time.UTC)

	record(t, db, school, later)
	record(t, db, school, later.Add(-24*time.Hour))

	at, _ := lastRun(t, db, school)
	assert.True(t, at.Equal(later), "an older success never replaces a newer one")
}

func TestCleanupRunRollsBackWithItsTransaction(t *testing.T) {
	t.Parallel()
	db := serveDB(t)
	school := testpkg.Tenant(t)
	failed := errors.New("deletion failed")

	err := testpkg.WithinTenantContext(t, context.Background(), db, school, func(ctx context.Context) error {
		if err := NewStore().RecordCleanupSucceeded(ctx, jobID, time.Now()); err != nil {
			return err
		}
		return failed
	})
	require.ErrorIs(t, err, failed)

	_, ok := lastRun(t, db, school)
	assert.False(t, ok, "a run whose transaction rolled back is not stored")
}

func TestCleanupRunRequiresATransaction(t *testing.T) {
	t.Parallel()

	_, _, _, err := NewStore().CleanupRun(context.Background(), jobID)
	require.Error(t, err)
	require.Error(t, NewStore().RecordCleanupSucceeded(context.Background(), jobID, time.Now()))
	require.Error(t, NewStore().RecordCleanupFailed(context.Background(), jobID, 1, time.Now()))
}

type storedRun struct {
	SucceededAt  time.Time
	Failures     int
	LastFailedAt time.Time
}

func runState(t *testing.T, db *bun.DB, tenantID int64) storedRun {
	t.Helper()
	var state storedRun
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		var err error
		state.SucceededAt, state.Failures, state.LastFailedAt, err = NewStore().CleanupRun(ctx, jobID)
		return err
	}))
	return state
}

func recordFailure(t *testing.T, db *bun.DB, tenantID int64, failures int, at time.Time) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		return NewStore().RecordCleanupFailed(ctx, jobID, failures, at)
	}))
}

// TestCleanupRunKeepsTheFailureStreakUntilASuccess covers #6327: the streak
// is stored per school, also before the first success; an older failure
// never replaces a later failure or success; a success ends the streak
// without losing its own time.
func TestCleanupRunKeepsTheFailureStreakUntilASuccess(t *testing.T) {
	t.Parallel()
	db := serveDB(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, testpkg.SetupTestDB(t))
	first := time.Date(2026, time.October, 14, 2, 0, 0, 0, time.UTC)

	recordFailure(t, db, school, 1, first)
	assert.Equal(t, 1, runState(t, db, school).Failures, "a first run that fails starts the streak")
	recordFailure(t, db, school, 3, first.Add(time.Hour))
	recordFailure(t, db, school, 2, first.Add(30*time.Minute))
	state := runState(t, db, school)
	assert.Equal(t, 3, state.Failures, "a lost write is made good, an older failure changes nothing")
	recordFailure(t, db, school, 2, first.Add(time.Hour))
	state = runState(t, db, school)
	assert.Equal(t, 3, state.Failures, "the same failure written again changes nothing")
	assert.True(t, state.LastFailedAt.Equal(first.Add(time.Hour)))
	assert.True(t, state.SucceededAt.IsZero(), "a failure is no success")
	assert.Zero(t, runState(t, db, otherSchool).Failures, "the streak of one school is invisible to another")

	succeeded := first.Add(3 * time.Hour)
	record(t, db, school, succeeded)
	state = runState(t, db, school)
	assert.True(t, state.SucceededAt.Equal(succeeded))
	assert.Zero(t, state.Failures, "a success ends the streak")
	assert.True(t, state.LastFailedAt.IsZero())

	recordFailure(t, db, school, 4, succeeded.Add(-time.Minute))
	assert.Zero(t, runState(t, db, school).Failures, "a failure before the last success changes nothing")
	recordFailure(t, db, school, 1, succeeded.Add(24*time.Hour))
	assert.Equal(t, 1, runState(t, db, school).Failures, "the next failure starts a new streak")
	assert.True(t, runState(t, db, school).SucceededAt.Equal(succeeded), "a failure keeps the last success")
}
