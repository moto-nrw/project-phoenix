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

// Writers of a row that references an account lock in two orders: an
// UPDATE locks the row and then, in auth.lock_live_account_reference, the
// account; an upsert or insert locks the account in that trigger first. The
// demo erasure avoids both cycles (#5540): it writes the memberships and
// roles of the visitor schools before it holds an account, locks only the
// demo accesses (updated row first) before the accounts, and locks no row of
// a table an upsert writes (memberships, roles, profiles) before them.
func TestDemoVisitorErasureLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(t.Context(), &definition))
	accountLock := strings.Index(definition, "PERFORM 1 FROM auth.accounts")
	require.Positive(t, accountLock, "the erasure locks the visitor accounts")
	before, after := definition[:accountLock], definition[accountLock:]

	for _, table := range []string{"auth.account_tenants", "auth.account_roles"} {
		visitorSchools := regexp.MustCompile(`DELETE FROM ` + regexp.QuoteMeta(table) + `\s[^;]*tenant_id = ANY \(visitor\.visitor_school_ids\);`)
		assert.True(t, visitorSchools.MatchString(before), "%s of the visitor schools goes before any account is held", table)
	}
	for _, table := range []string{"auth.account_tenants", "auth.account_roles", "users.profiles"} {
		assert.NotRegexp(t, `PERFORM 1 FROM `+regexp.QuoteMeta(table)+`\s`, before, "no row of %s is locked before the account", table)
	}
	assert.Regexp(t, `PERFORM 1 FROM auth\.demo_accesses\s[^;]*FOR UPDATE;`, before, "the demo accesses are locked before the accounts")
	assert.Regexp(t, `DELETE FROM auth\.demo_accesses\s`, after)
	assert.Regexp(t, `auth\.account_roles AS role\s+WHERE role\.account_id = account\.id\s+AND role\.tenant_id <> ALL \(visitor\.visitor_school_ids\)`, after,
		"a role outside the visitor schools keeps the account")
}
