package migrations

import (
	"regexp"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// auth.lock_live_account_reference locks a row that references an account
// first and the account second. The demo erasure keeps that order (#5540):
// every table with that trigger which it still writes after locking the
// accounts has its rows locked before the accounts, so the erasure never
// holds an account while it waits for such a row.
func TestDemoVisitorErasureLocksReferencingRowsBeforeTheAccounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &definition))
	var guarded []string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT trigger.tgrelid::regclass::text FROM pg_trigger AS trigger
		WHERE trigger.tgfoid = 'auth.lock_live_account_reference'::regproc ORDER BY 1`).Scan(ctx, &guarded))
	require.NotEmpty(t, guarded)

	accountLock := strings.Index(definition, "PERFORM 1 FROM auth.accounts")
	require.Positive(t, accountLock, "the erasure locks the visitor accounts")
	before, after := definition[:accountLock], definition[accountLock:]
	for _, table := range guarded {
		written := regexp.MustCompile(`(DELETE FROM|UPDATE) ` + regexp.QuoteMeta(table) + `\s`)
		locked := regexp.MustCompile(`PERFORM 1 FROM ` + regexp.QuoteMeta(table) + `\s[^;]*FOR UPDATE;`)
		if written.MatchString(after) {
			assert.True(t, locked.MatchString(before), "%s is written after the account lock, so its rows are locked before it", table)
		}
	}
	assert.Contains(t, after, "DELETE FROM auth.account_tenants ", "the check guards at least the memberships")
	// Writers of another school may hold the account first (a profile change
	// locks the user name, then the profile), so a row locked before the
	// account belongs to a visitor school.
	for _, table := range []string{"auth.account_tenants", "auth.account_roles", "users.profiles"} {
		restricted := regexp.MustCompile(`PERFORM 1 FROM ` + regexp.QuoteMeta(table) + `\s[^;]*tenant_id = ANY \(visitor\.visitor_school_ids\)[^;]*FOR UPDATE;`)
		assert.True(t, restricted.MatchString(before), "%s is locked before the account only in the visitor schools", table)
	}
}
