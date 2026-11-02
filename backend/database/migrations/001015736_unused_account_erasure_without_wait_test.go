package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type erasureFunctionState struct {
	Definition string
	Config     []string
	ACL        string
}

func readUnusedAccountErasure(t *testing.T, db *bun.DB) erasureFunctionState {
	t.Helper()
	var state erasureFunctionState
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef(oid), COALESCE(proconfig, '{}'), COALESCE(proacl::text, '')
		FROM pg_proc WHERE oid = 'auth.erase_unused_accounts()'::regprocedure`).
		Scan(t.Context(), &state.Definition, pgdialect.Array(&state.Config), &state.ACL))
	return state
}

// Writers lock an account's rows and the account in both orders (a session
// renewal the session row, then the account; a login the membership, then
// the account). The erasure therefore never waits long for a lock (#5546):
// it runs with a lock_timeout far below deadlock_timeout, erases one account
// per subtransaction and defers an account whose lock is in use. The
// behaviour, in both orders against each writer, is tested in
// modules/identityaccess/compose/account_erasure_lock_test.go.
func TestUnusedAccountErasureNeverWaitsLongForALock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	state := readUnusedAccountErasure(t, db)
	assert.Contains(t, state.Config, "lock_timeout=100ms", "the erasure gives up on a lock before a deadlock is detected")
	var deadlockTimeout, lockTimeout float64
	require.NoError(t, db.NewRaw(`SELECT
		EXTRACT(EPOCH FROM current_setting('deadlock_timeout')::interval),
		EXTRACT(EPOCH FROM '100ms'::interval)`).Scan(t.Context(), &deadlockTimeout, &lockTimeout))
	assert.Less(t, lockTimeout*5, deadlockTimeout, "the lock timeout stays far below deadlock_timeout")

	definition := state.Definition
	assert.Regexp(t, `IF tried >= 200 OR clock_timestamp\(\) - started > INTERVAL '500 milliseconds' THEN\s+EXIT;`, definition,
		"the erasure has a budget")
	assert.Regexp(t, `BEGIN\s+--[^\n]*\n[^\n]*\n\s+PERFORM 1 FROM auth\.accounts AS account\s+WHERE account\.id = candidate AND account\.personal_data_erased_at IS NULL\s+FOR UPDATE SKIP LOCKED;`, definition,
		"the account is locked inside its subtransaction and passed over when held")
	assert.Regexp(t, `EXCEPTION WHEN lock_not_available OR deadlock_detected THEN\s+RAISE LOG 'unused account erasure deferred account %`, definition,
		"an account whose lock is in use is deferred, not failed")
	assert.Regexp(t, `EXCEPTION WHEN lock_not_available OR deadlock_detected THEN\s+RAISE LOG 'unused account erasure skipped this run`, definition,
		"a locked table the scan reads skips the run")
	assert.Regexp(t, `erased := erased \+ 1;\s+(--[^\n]*\n\s+)*RETURN QUERY`, definition,
		"an account is returned only once it is erased")
	assert.NotContains(t, definition, "= ANY (erased_ids)", "no statement works on several accounts at once")
}

// Down reinstalls the erasure of 1.15.646 exactly, grants included, and Up
// installs this one again.
func TestUnusedAccountErasureWithoutWaitRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	current := readUnusedAccountErasure(t, db)

	require.NoError(t, unusedAccountErasureWithoutWaitDown(ctx, db))
	restored := readUnusedAccountErasure(t, db)
	assert.Equal(t, []string{"search_path=pg_catalog"}, restored.Config, "the lock_timeout goes with the rollback")
	assert.Equal(t, current.ACL, restored.ACL, "the grants stay")
	assert.Contains(t, restored.Definition, "FOR UPDATE SKIP LOCKED\n\t\t\t) AS locked;", "the erasure of 1.15.646 is back")

	require.NoError(t, unusedAccountErasureWithoutWaitUp(ctx, db))
	assert.Equal(t, current, readUnusedAccountErasure(t, db), "Up installs this erasure again")
}
