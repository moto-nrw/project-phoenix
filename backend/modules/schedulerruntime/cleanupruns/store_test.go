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
		at, ok, err = NewStore().LastCleanupSucceeded(ctx, jobID)
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

	_, _, err := NewStore().LastCleanupSucceeded(context.Background(), jobID)
	require.Error(t, err)
	require.Error(t, NewStore().RecordCleanupSucceeded(context.Background(), jobID, time.Now()))
}
