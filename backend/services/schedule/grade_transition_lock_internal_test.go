package schedule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	db := testpkg.SetupTestDB(t)

	tenantCtx := tenant.WithTenantID(context.Background(), 1)

	t.Run("refuses without a database", func(t *testing.T) {
		require.ErrorContains(t, lockTenantGradeTransitions(tenantCtx, nil),
			"database is not configured")
	})

	t.Run("refuses without a tenant", func(t *testing.T) {
		// The lock key is derived from the tenant id; without one there is no
		// school to serialize against.
		require.ErrorContains(t, lockTenantGradeTransitions(context.Background(), db),
			"tenant id is required")
	})

	t.Run("refuses outside a transaction", func(t *testing.T) {
		// An implicit one-statement transaction would release the advisory lock
		// before the protected operation even starts.
		require.ErrorContains(t, lockTenantGradeTransitions(tenantCtx, db),
			"requires a transaction")
	})

	t.Run("acquires the gate inside a transaction", func(t *testing.T) {
		tx, err := db.BeginTx(tenantCtx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.NoError(t, lockTenantGradeTransitions(modelBase.ContextWithTx(tenantCtx, &tx), db))
	})
}

// TestLockTenantRecurrenceWrites covers the same guards on the recurrence gate,
// which the documented lock order always takes FIRST.
func TestLockTenantRecurrenceWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantCtx := tenant.WithTenantID(context.Background(), 1)

	require.ErrorContains(t, lockTenantRecurrenceWrites(tenantCtx, nil),
		"database is not configured")
	require.ErrorContains(t, lockTenantRecurrenceWrites(context.Background(), db),
		"tenant id is required")
	require.ErrorContains(t, lockTenantRecurrenceWrites(tenantCtx, db),
		"requires a transaction")
}
