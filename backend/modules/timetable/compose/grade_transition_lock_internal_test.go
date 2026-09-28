package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestLockTenantGradeTransitions pins the gate that keeps a materialization pass
// and a grade transition apply from running concurrently for one school, plus
// the three refusals that make it meaningful.
//
// Each refusal exists because acquiring nothing looks exactly like acquiring the
// lock at the call site: a PostgreSQL advisory lock is transaction-scoped, so a
// caller outside a transaction would take it and lose it again before the
// protected work runs, and a call without a tenant would serialize on a key that
// belongs to no school. Failing loudly beats an overlap nobody notices (#405).
func TestLockTenantGradeTransitions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantCtx := testpkg.Ctx(t)

	t.Run("refuses without a database", func(t *testing.T) {
		_, err := NewRecurrenceWriteLock(RecurrenceLockDependencies{GradeTransitionLockKey: testGradeTransitionLockKey})
		require.ErrorContains(t, err, "required dependency is nil")
	})

	lock := newTestRecurrenceWriteLock(t, db)

	t.Run("refuses without a tenant", func(t *testing.T) {
		// The lock key is derived from the tenant id; without one there is no
		// school to serialize against.
		require.ErrorContains(t, lock.LockRecurrenceWritesThenGradeTransitions(context.Background()),
			"tenant id is required")
	})

	t.Run("refuses outside a transaction", func(t *testing.T) {
		// An implicit one-statement transaction would release the advisory lock
		// before the protected operation even starts.
		require.ErrorContains(t, lock.LockRecurrenceWritesThenGradeTransitions(tenantCtx),
			"requires a transaction")
	})

	t.Run("acquires the gate inside a transaction", func(t *testing.T) {
		tx, err := db.BeginTx(tenantCtx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.NoError(t, lock.LockRecurrenceWritesThenGradeTransitions(tenant.WithTransactionForTest(tenantCtx, &tx)))
	})
}

// TestLockTenantRecurrenceWrites covers the same guards on the recurrence gate,
// which the documented lock order always takes FIRST.
func TestLockTenantRecurrenceWrites(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantCtx := testpkg.Ctx(t)

	_, err := NewRecurrenceWriteLock(RecurrenceLockDependencies{GradeTransitionLockKey: testGradeTransitionLockKey})
	require.ErrorContains(t, err, "required dependency is nil")
	lock := newTestRecurrenceWriteLock(t, db)
	require.ErrorContains(t, lock.LockRecurrenceWrites(context.Background()),
		"tenant id is required")
	require.ErrorContains(t, lock.LockRecurrenceWrites(tenantCtx),
		"requires a transaction")
}

// testGradeTransitionLockKey stands in for School Structure's grade-transition
// key, which the composition root binds in production.
func testGradeTransitionLockKey(tenantID int64) string {
	return fmt.Sprintf("test_grade_transitions_%d", tenantID)
}

func newTestRecurrenceWriteLock(t *testing.T, db *bun.DB) timetable.RecurrenceWriteLock {
	t.Helper()
	lock, err := NewRecurrenceWriteLock(RecurrenceLockDependencies{DB: db, GradeTransitionLockKey: testGradeTransitionLockKey})
	require.NoError(t, err)
	return lock
}
