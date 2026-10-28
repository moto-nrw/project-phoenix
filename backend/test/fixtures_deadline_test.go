package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestParentGuardianChainDeadlineExcludesNestedFixtures pins the budget rule
// behind #5191: a composing fixture's deadline covers its own statements, not
// the nested fixtures that already have one each. Table locks keep the
// student and then the account fixture blocked for 2.6s each, counted from
// the moment PostgreSQL reports the fixture waiting on the lock. Neither
// exceeds its own 5s, but together they would exhaust a deadline started
// before them, and the chain's guardian profile would fail with "context
// deadline exceeded" as it did under a loaded local test gate.
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

	blocked := make(chan []bool, 1)
	go func() {
		var seen []bool
		for _, lock := range []heldLock{students, accounts} {
			waited := awaitLockWaiter(db, lock.table)
			if waited {
				time.Sleep(hold)
			}
			lock.release()
			seen = append(seen, waited)
		}
		blocked <- seen
	}()

	chain := CreateTestParentGuardianChain(t, db)

	assert.Equal(t, []bool{true, true}, <-blocked, "both nested fixtures must have waited on their lock")
	assert.Positive(t, chain.GuardianProfileID)
}

type heldLock struct {
	table   string
	release func()
}

// lockTableFor holds an EXCLUSIVE lock on table, which blocks inserts into it,
// until release is called.
func lockTableFor(t *testing.T, db *bun.DB, table string) heldLock {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), "LOCK TABLE "+table+" IN EXCLUSIVE MODE")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return heldLock{table: table, release: func() { _ = tx.Commit() }}
}

// awaitLockWaiter reports whether a session started waiting for a lock on
// table within a generous bound. False means the fixture never reached the
// table, and the caller releases the lock anyway so the test cannot hang.
func awaitLockWaiter(db *bun.DB, table string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT FROM pg_locks
			WHERE relation = ?::regclass AND NOT granted)`, table).Scan(ctx, &waiting)
		if err == nil && waiting {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
