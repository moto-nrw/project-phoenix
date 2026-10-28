package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestParentGuardianChainDeadlineExcludesNestedFixtures pins the budget rule
// behind #5191: a composing fixture's deadline covers its own statements, not
// the nested fixtures that already have one each. Two table locks slow the
// student and the account fixture to about 2.6s each. Neither exceeds its own
// 5s, but together they would exhaust a deadline started before them, and
// the chain's guardian profile and link would fail with "context deadline
// exceeded" as they did under a loaded local test gate.
func TestParentGuardianChainDeadlineExcludesNestedFixtures(t *testing.T) {
	t.Parallel()

	// The locks are database-global, so the test owns its clone.
	db := SetupIsolatedTestDB(t)
	Tenant(t)

	const hold = 2600 * time.Millisecond
	students := lockTableFor(t, db, "users.student_profiles")
	// The account fixture's tenant mapping; locking auth.accounts itself
	// would also stall the person insert through its account reference.
	accounts := lockTableFor(t, db, "auth.account_tenants")
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(hold)
		students()
		time.Sleep(hold)
		accounts()
	}()

	start := time.Now()
	chain := CreateTestParentGuardianChain(t, db)
	elapsed := time.Since(start)
	<-released

	assert.GreaterOrEqual(t, elapsed, 2*hold, "both locks must have delayed the nested fixtures")
	assert.Positive(t, chain.GuardianProfileID)
}

// lockTableFor holds an EXCLUSIVE lock on table, which blocks inserts into it,
// until the returned release is called.
func lockTableFor(t *testing.T, db *bun.DB, table string) (release func()) {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), "LOCK TABLE "+table+" IN EXCLUSIVE MODE")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return func() { _ = tx.Commit() }
}
