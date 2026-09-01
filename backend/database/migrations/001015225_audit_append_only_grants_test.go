package migrations

import (
	"context"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allTablePrivileges is the complete PostgreSQL 17 table privilege set. Checking
// only SELECT/INSERT/UPDATE/DELETE would let a lone TRUNCATE (or REFERENCES,
// TRIGGER, MAINTAIN) grant slip through an assertion that claims "no privilege
// at all".
var allTablePrivileges = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE",
	"TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN",
}

// allSequencePrivileges is the complete PostgreSQL sequence privilege set. A
// sequence is a separate object from its table: "REVOKE ALL ON <table>" leaves
// it untouched, so a backup table locked down at the table level can still have
// its counter moved via nextval (USAGE).
var allSequencePrivileges = []string{"USAGE", "SELECT", "UPDATE"}

func auditSchemaTables(t *testing.T, db *testpkg.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'audit' AND c.relkind = 'r'
		ORDER BY c.relname
	`).Scan(context.Background(), &tables))
	return tables
}

// tenantACLEntries returns the raw aclitem strings granted to phoenix_tenant on
// an audit table, e.g. "phoenix_tenant=arD/postgres". Empty means the role holds
// no directly granted privilege of any kind.
func tenantACLEntries(t *testing.T, db *testpkg.DB, table string) []string {
	t.Helper()
	var entries []string
	require.NoError(t, db.NewRaw(`
		SELECT acl::text
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		CROSS JOIN LATERAL unnest(COALESCE(c.relacl, '{}'::aclitem[])) AS acl
		WHERE n.nspname = 'audit' AND c.relname = ?
		  AND acl::text LIKE 'phoenix_tenant=%'
	`, table).Scan(context.Background(), &entries))
	return entries
}

func tenantHasPrivilege(t *testing.T, db *testpkg.DB, relation, privilege string) bool {
	t.Helper()
	var granted bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, ?)`, relation, privilege).
		Scan(context.Background(), &granted))
	return granted
}

func tenantHasSequencePrivilege(t *testing.T, db *testpkg.DB, sequence, privilege string) bool {
	t.Helper()
	var granted bool
	require.NoError(t, db.NewRaw(`SELECT has_sequence_privilege('phoenix_tenant', ?, ?)`, sequence, privilege).
		Scan(context.Background(), &granted))
	return granted
}

// tenantSequenceACLEntries is the sequence counterpart of tenantACLEntries.
func tenantSequenceACLEntries(t *testing.T, db *testpkg.DB, sequence string) []string {
	t.Helper()
	var entries []string
	require.NoError(t, db.NewRaw(`
		SELECT acl::text
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		CROSS JOIN LATERAL unnest(COALESCE(c.relacl, '{}'::aclitem[])) AS acl
		WHERE n.nspname = 'audit' AND c.relname = ? AND c.relkind = 'S'
		  AND acl::text LIKE 'phoenix_tenant=%'
	`, sequence).Scan(context.Background(), &entries))
	return entries
}

// TestAuditSchemaAppendOnlyForTenantRole is the ratchet for issue #1924. The
// audit schema must stay append-only for phoenix_tenant: never UPDATE or DELETE.
func TestAuditSchemaAppendOnlyForTenantRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tables := auditSchemaTables(t, db)
	require.NotEmpty(t, tables, "expected tables in the audit schema")

	for _, table := range tables {
		relation := "audit." + table

		assert.Falsef(t, tenantHasPrivilege(t, db, relation, "UPDATE"),
			"%s: phoenix_tenant must not hold UPDATE — audit rows are append-only. "+
				"A new table inherits this from the schema default ACL only if migration "+
				"1.15.225's ALTER DEFAULT PRIVILEGES was undone; otherwise some migration "+
				"granted it explicitly.", relation)

		assert.Falsef(t, tenantHasPrivilege(t, db, relation, "DELETE"),
			"%s: phoenix_tenant must not hold DELETE. Retention must run through a fixed "+
				"database capability; ON DELETE CASCADE needs no table privilege.", relation)

		// TRUNCATE has no allowlist: the retention jobs delete row by row with a
		// date predicate, so nothing legitimately empties an audit table under
		// the tenant role — and an append-only table that can be truncated in
		// one statement is not append-only.
		assert.Falsef(t, tenantHasPrivilege(t, db, relation, "TRUNCATE"),
			"%s: phoenix_tenant must not hold TRUNCATE — it erases the whole audit table "+
				"in one statement, DELETE-allowlist or not.", relation)
	}

	// The one-off migration backups carry no RLS, so any tenant-role access
	// would cross tenant borders. Restores run as phoenix_admin/superuser.
	// Check the COMPLETE PostgreSQL table privilege set, not just the four
	// obvious ones: TRUNCATE alone would let the tenant role erase the
	// cross-tenant backup contents.
	for _, table := range []string{"room_color_migration_backup", "wc_alias_migration_backup"} {
		relation := "audit." + table
		for _, privilege := range allTablePrivileges {
			assert.Falsef(t, tenantHasPrivilege(t, db, relation, privilege),
				"%s: phoenix_tenant must hold no %s — the table has no RLS policy", relation, privilege)
		}
		// Belt and braces against a privilege type a future PostgreSQL adds and
		// allTablePrivileges does not know about yet: the table ACL must carry
		// no phoenix_tenant entry whatsoever.
		assert.Emptyf(t, tenantACLEntries(t, db, table), //nolint:testifylint // Empty reads better than Len(…, 0) here
			"audit.%s: table ACL still carries a phoenix_tenant grant", table)

		// Same for the BIGSERIAL sequence behind the table. It is a distinct
		// object that the table-level REVOKE does not reach, and 1.14.1's
		// sequence default ACL granted USAGE on it — enough to move the
		// counter of a table the role must not touch at all.
		sequence := table + "_id_seq"
		for _, privilege := range allSequencePrivileges {
			assert.Falsef(t, tenantHasSequencePrivilege(t, db, "audit."+sequence, privilege),
				"audit.%s: phoenix_tenant must hold no %s — the backing table has no RLS policy "+
					"and REVOKE ALL on the table does not cover its sequence", sequence, privilege)
		}
		assert.Emptyf(t, tenantSequenceACLEntries(t, db, sequence), //nolint:testifylint // Empty reads better than Len(…, 0) here
			"audit.%s: sequence ACL still carries a phoenix_tenant grant", sequence)
	}
}

// TestAuditSchemaDefaultACLIsAppendOnly guards the root cause: without this,
// the next audit table silently inherits UPDATE/DELETE again and only the
// per-table assertions above would catch it — after the fact.
func TestAuditSchemaDefaultACLIsAppendOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	var aclItems []string
	require.NoError(t, db.NewRaw(`
		SELECT unnest(d.defaclacl)::text
		FROM pg_default_acl d
		JOIN pg_namespace n ON n.oid = d.defaclnamespace
		WHERE n.nspname = 'audit' AND d.defaclobjtype = 'r'
	`).Scan(context.Background(), &aclItems))

	for _, item := range aclItems {
		// aclitem text form: "grantee=privileges/grantor", e.g. "phoenix_tenant=ar/postgres".
		grantee, rest, found := strings.Cut(item, "=")
		if !found || grantee != "phoenix_tenant" {
			continue
		}
		privileges, _, _ := strings.Cut(rest, "/")
		// aclitem privilege letters (case-sensitive): w=UPDATE, d=DELETE,
		// D=TRUNCATE. r=SELECT and a=INSERT are what an audit table needs.
		for letter, privilege := range map[string]string{"w": "UPDATE", "d": "DELETE", "D": "TRUNCATE"} {
			assert.NotContainsf(t, privileges, letter,
				"default ACL on schema audit still grants phoenix_tenant %s (%s) — "+
					"every future audit table would start with it", privilege, item)
		}
	}
}
