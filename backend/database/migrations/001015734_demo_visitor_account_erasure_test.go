package migrations

import (
	"regexp"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/dialect/pgdialect"
)

var deletedTable = regexp.MustCompile(`DELETE FROM ([a-z_]+\.[a-z_]+)`)

func deletedTables(definition string) map[string]bool {
	tables := map[string]bool{}
	for _, match := range deletedTable.FindAllStringSubmatch(definition, -1) {
		tables[match[1]] = true
	}
	return tables
}

// The demo erasure anonymizes an account that hangs only in demo schools
// instead of deleting it, so nothing that would go with the account by
// cascade goes by itself (#5540). It deletes what the general account
// erasure deletes for the same reason. A Träger member is never a
// demo-only account, so the Träger tables have nothing to delete.
func TestDemoVisitorErasureDeletesWhatTheAccountErasureDeletes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	var accountErasure, demoErasure string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('auth.erase_unused_accounts()'::regprocedure)`).Scan(ctx, &accountErasure))
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &demoErasure))

	neverDemoOnly := map[string]bool{"auth.organization_account_roles": true, "platform.organization_accounts": true}
	demo := deletedTables(demoErasure)
	for table := range deletedTables(accountErasure) {
		if neverDemoOnly[table] {
			continue
		}
		assert.True(t, demo[table], "the demo erasure deletes %s of an anonymized visitor account", table)
	}
	assert.Contains(t, demoErasure, "INSERT INTO auth.account_avatar_file_cleanup",
		"the avatar files of an anonymized visitor account are queued for removal")
	assert.Contains(t, demoErasure, "personal_data_erased_at", "an anonymized visitor account counts as erased")
}

// The demo role reaches the new deletions only through the function.
func TestDemoRoleCannotWriteTheVisitorAccountTablesDirectly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, table := range []string{"iot.push_subscriptions", "iot.pwa_standalone_usage", "users.profiles", "auth.account_avatar_file_cleanup", "auth.accounts"} {
		var canWrite bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_demo', ?, 'INSERT') OR has_table_privilege('phoenix_demo', ?, 'UPDATE')
			OR has_table_privilege('phoenix_demo', ?, 'DELETE')`, table, table, table).Scan(t.Context(), &canWrite))
		assert.False(t, canWrite, "phoenix_demo gets no write grant on %s", table)
	}
}

// Writers lock rows and accounts in every order: an UPDATE locks the row
// and then, in auth.lock_live_account_reference, the account; an upsert
// locks the account in that trigger first; role removal locks the role and
// then deletes every session of the account. Instead of an order of its own,
// the demo erasure never waits for a lock (#5540): it runs with a
// lock_timeout far below deadlock_timeout and defers an order whose lock is
// in use, or that a deadlock hits anyway, in a subtransaction, so that
// neither the run nor the other transaction fails.
func TestDemoVisitorErasureNeverWaitsForALock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	var config []string
	require.NoError(t, db.NewRaw(`SELECT COALESCE(proconfig, '{}') FROM pg_proc
		WHERE oid = 'platform.erase_closed_demo_visitors()'::regprocedure`).Scan(ctx, pgdialect.Array(&config)))
	assert.Contains(t, config, "lock_timeout=100ms", "the erasure gives up on a lock before a deadlock is detected")
	var deadlockTimeout, lockTimeout float64
	require.NoError(t, db.NewRaw(`SELECT
		EXTRACT(EPOCH FROM current_setting('deadlock_timeout')::interval),
		EXTRACT(EPOCH FROM '100ms'::interval)`).Scan(ctx, &deadlockTimeout, &lockTimeout))
	assert.Less(t, lockTimeout*5, deadlockTimeout, "the lock timeout stays far below deadlock_timeout")

	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &definition))
	assert.Regexp(t, `EXCEPTION WHEN lock_not_available OR deadlock_detected THEN\s+PERFORM platform\.defer_demo_order\(target_name, 'erasure', SQLSTATE\);`, definition,
		"an order whose lock is in use is deferred, not failed")
	assert.Contains(t, definition, "WHERE (state.status = 'failed' OR school.deleted_at IS NOT NULL)",
		"every hidden or failed order is open, whatever its status")
	assert.Regexp(t, `IF tried >= 50 OR clock_timestamp\(\) - statement_timestamp\(\) > INTERVAL '500 milliseconds' THEN`, definition,
		"a run has a budget")
	assert.Regexp(t, `BEGIN\s+PERFORM pg_advisory_xact_lock\(hashtextextended\('demo school seed:'`, definition,
		"the subtransaction covers the order's own locks too")
	assert.Regexp(t, `PERFORM 1 FROM auth\.accounts[^;]*FOR UPDATE;`, definition, "the check holds the visitor accounts")
	assert.Regexp(t, `auth\.account_roles AS role\s+WHERE role\.account_id = account\.id\s+AND role\.tenant_id <> ALL \(visitor\.visitor_school_ids\)`, definition,
		"a role outside the visitor schools keeps the account")
	var note string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.defer_demo_order(text, text, text)'::regprocedure)`).Scan(ctx, &note))
	assert.Regexp(t, `deferrals >= 24 THEN\s+RAISE WARNING`, note, "a long run of deferrals is reported")

	// The run starts in retire_expired_demo_schools: the same holds there.
	require.NoError(t, db.NewRaw(`SELECT COALESCE(proconfig, '{}') FROM pg_proc
		WHERE oid = 'platform.retire_expired_demo_schools(text[])'::regprocedure`).Scan(ctx, pgdialect.Array(&config)))
	assert.Contains(t, config, "lock_timeout=100ms", "the run gives up on a lock before a deadlock is detected")
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.retire_expired_demo_schools(text[])'::regprocedure)`).Scan(ctx, &definition))
	assert.Regexp(t, `IF NOT pg_try_advisory_xact_lock\(hashtextextended\('platform\.demo_school_states:visitor-erasure', 0\)\) THEN`, definition,
		"a run lock in use skips the run")
	assert.NotRegexp(t, `PERFORM pg_advisory_xact_lock\(hashtextextended\('platform\.demo_school_states:visitor-erasure'`, definition,
		"the run lock is never waited for")
	assert.Regexp(t, `BEGIN[\s\S]*?PERFORM pg_advisory_xact_lock\(hashtextextended\('demo school seed:'[\s\S]*EXCEPTION WHEN lock_not_available OR deadlock_detected THEN\s+PERFORM platform\.defer_demo_order\(target_name, 'retirement', SQLSTATE\);`, definition,
		"each order is retired in a subtransaction that a lock in use defers")
	assert.Regexp(t, `AND \(school\.id IS NULL OR school\.deleted_at IS NULL\);\s+IF NOT FOUND THEN\s+CONTINUE;`, definition,
		"a hidden or failed order is left to the erasure: one deferral per order and run")
	assert.Regexp(t, `IF tried >= 50 OR clock_timestamp\(\) - statement_timestamp\(\) > INTERVAL '500 milliseconds' THEN`, definition,
		"the retirement has the run's budget too")
	assert.Regexp(t, `state\.status = 'ready'\s+AND state\.first_name <> ''\s+AND school\.deleted_at IS NULL\s+AND NOT EXISTS \(SELECT 1 FROM auth\.demo_accesses`, definition,
		"every run retires the open visitor orders no access names, so deferred work is found again")
	assert.Regexp(t, `EXCEPTION WHEN lock_not_available OR deadlock_detected THEN\s+RAISE LOG 'demo school retirement skipped this run`, definition,
		"a locked table the scan reads skips the run")
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &definition))
	assert.Regexp(t, `EXCEPTION WHEN lock_not_available OR deadlock_detected THEN\s+RAISE LOG 'demo visitor erasure skipped this run`, definition,
		"a locked table the erasure scan reads skips the erasure")
}
