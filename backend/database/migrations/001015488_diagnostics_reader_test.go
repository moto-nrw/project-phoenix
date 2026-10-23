package migrations

import (
	"fmt"
	"os"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestDiagnosticsReaderScopesMetadataToSelectedSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	ours := testpkg.Tenant(t)
	theirs := testpkg.NewTenantScope(t, db).TenantID
	for _, school := range []int64{ours, theirs} {
		testpkg.CreateTestRoomForTenant(t, db, school, "diagnostics")
		testpkg.CreateTestEducationGroupForTenant(t, db, school, "diagnostics")
		testpkg.CreateTestActivityGroupForTenant(t, db, school, "diagnostics")
	}
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_diagnostics; SET LOCAL app.current_tenant_id = ''`)
	require.NoError(t, err)
	for _, table := range []string{"facilities.rooms", "education.groups", "activities.groups"} {
		var count int
		require.NoError(t, tx.NewRaw(`SELECT count(id) FROM `+table).Scan(ctx, &count))
		require.Zero(t, count, "no school scope means no rows")
	}
	_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?::text, true)`, ours)
	require.NoError(t, err)
	for _, table := range []string{"facilities.rooms", "education.groups", "activities.groups"} {
		var own, foreign int
		require.NoError(t, tx.NewRaw(`SELECT count(id) FROM `+table+` WHERE tenant_id = ?`, ours).Scan(ctx, &own))
		require.Positive(t, own, "selected school metadata is readable")
		require.NoError(t, tx.NewRaw(`SELECT count(id) FROM `+table+` WHERE tenant_id = ?`, theirs).Scan(ctx, &foreign))
		require.Zero(t, foreign, "no cross-school rows, even without an application filter")
		_, err = tx.ExecContext(ctx, `SELECT id, tenant_id, created_at, updated_at FROM `+table+` LIMIT 0`)
		require.NoError(t, err)
	}
}

func TestDiagnosticsReaderDeniesSensitiveColumnsAndWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	// LIMIT 0 checks authorization without reading or printing any values.
	queries := []string{
		`SELECT iban FROM users.staff_financial_data LIMIT 0`,
		`SELECT tax_id FROM users.staff_financial_data LIMIT 0`,
		`SELECT social_security_number FROM users.staff_financial_data LIMIT 0`,
		`SELECT iban FROM users.guardian_financial_data LIMIT 0`,
		`SELECT account_holder FROM users.guardian_financial_data LIMIT 0`,
		`SELECT health_info FROM users.student_care_profiles LIMIT 0`,
		`SELECT old_value FROM audit.staff_master_data_changes LIMIT 0`,
		`SELECT new_value FROM audit.staff_master_data_changes LIMIT 0`,
		`SELECT old_value FROM audit.guardian_financial_changes LIMIT 0`,
		`SELECT new_value FROM audit.guardian_financial_changes LIMIT 0`,
		`SELECT old_value FROM audit.student_field_edits LIMIT 0`,
		`SELECT new_value FROM audit.student_field_edits LIMIT 0`,
		`SELECT old_value FROM users.student_data_change_requests LIMIT 0`,
		`SELECT new_value FROM users.student_data_change_requests LIMIT 0`,
		`SELECT * FROM audit.data_access_log LIMIT 0`,
		`SELECT * FROM config.setting_audit LIMIT 0`,
		`SELECT * FROM active.student_status_days LIMIT 0`,
		`SELECT * FROM active.staff_absences LIMIT 0`,
		`SELECT * FROM active.staff_absence_audit LIMIT 0`,
		`SELECT * FROM enrollment.request_children LIMIT 0`,
		`SELECT * FROM users.staff_documents LIMIT 0`,
		`SELECT * FROM documents.files LIMIT 0`,
		`SELECT * FROM auth.accounts LIMIT 0`,
		`SELECT * FROM facilities.rooms LIMIT 0`,
		`SELECT name FROM education.groups LIMIT 0`,
		`SELECT name FROM activities.groups LIMIT 0`,
		`UPDATE facilities.rooms SET updated_at = updated_at WHERE false`,
		`DELETE FROM facilities.rooms WHERE false`,
		`INSERT INTO facilities.rooms (id) SELECT NULL::bigint WHERE false`,
		`TRUNCATE facilities.rooms`,
	}
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { require.NoError(t, tx.Rollback()) }()
			_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_diagnostics`)
			require.NoError(t, err)
			_, err = tx.ExecContext(t.Context(), query)
			require.ErrorContains(t, err, "permission denied")
			require.ErrorContains(t, err, "SQLSTATE=42501")
		})
	}
}

func TestDiagnosticsReaderEffectivePrivilegesStayOnExplicitAllowlist(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	var columns []string
	require.NoError(t, db.NewRaw(`
SELECT n.nspname || '.' || c.relname || '.' || a.attname
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND NOT EXISTS (SELECT FROM pg_depend d JOIN pg_extension e ON e.oid = d.refobjid
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid
                    AND d.deptype = 'e' AND e.extname = 'pg_stat_statements')
  AND has_column_privilege('phoenix_diagnostics', c.oid, a.attnum, 'SELECT')
`).Scan(ctx, &columns))
	require.ElementsMatch(t, []string{
		"facilities.rooms.id", "facilities.rooms.tenant_id", "facilities.rooms.created_at", "facilities.rooms.updated_at",
		"education.groups.id", "education.groups.tenant_id", "education.groups.created_at", "education.groups.updated_at",
		"activities.groups.id", "activities.groups.tenant_id", "activities.groups.created_at", "activities.groups.updated_at",
	}, columns, "effective grants include PUBLIC and inherited rights, views and future columns")
	var unexpected []string
	require.NoError(t, db.NewRaw(`
SELECT n.nspname || '.' || c.relname
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND NOT EXISTS (SELECT FROM pg_depend d JOIN pg_extension e ON e.oid = d.refobjid
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid
                    AND d.deptype = 'e' AND e.extname = 'pg_stat_statements')
  AND (has_table_privilege('phoenix_diagnostics', c.oid,
       'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
       OR has_any_column_privilege('phoenix_diagnostics', c.oid, 'INSERT,UPDATE,REFERENCES'))
`).Scan(ctx, &unexpected))
	require.Empty(t, unexpected, "no whole-table or write privileges")
	require.NoError(t, db.NewRaw(`
SELECT n.nspname || '.' || c.relname FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'S' AND n.nspname NOT LIKE 'pg_%'
  AND CASE WHEN c.relkind = 'S'
      THEN has_sequence_privilege('phoenix_diagnostics', c.oid, 'USAGE,SELECT,UPDATE')
      ELSE false END
`).Scan(ctx, &unexpected))
	require.Empty(t, unexpected, "no sequence access")
	require.NoError(t, db.NewRaw(`
SELECT p.oid::regprocedure::text FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE p.prosecdef AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
  AND has_schema_privilege('phoenix_diagnostics', n.oid, 'USAGE')
  AND has_function_privilege('phoenix_diagnostics', p.oid, 'EXECUTE')
`).Scan(ctx, &unexpected))
	require.Empty(t, unexpected, "no callable application SECURITY DEFINER bypass")
	require.NoError(t, db.NewRaw(`SELECT nspname FROM pg_namespace
WHERE nspname NOT LIKE 'pg_%' AND nspname <> 'information_schema'
  AND has_schema_privilege('phoenix_diagnostics', oid, 'CREATE')`).Scan(ctx, &unexpected))
	require.Empty(t, unexpected, "no application DDL, including replacement functions or views")
	var unsafe bool
	require.NoError(t, db.NewRaw(`SELECT
    rolcanlogin OR rolinherit OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls
    OR EXISTS (SELECT FROM pg_auth_members WHERE member = r.oid)
FROM pg_roles r WHERE rolname = 'phoenix_diagnostics'`).Scan(ctx, &unsafe))
	require.False(t, unsafe, "no login, elevated attributes or SET ROLE path to another role")
}

func TestDiagnosticsReaderPreservesMFARuntimeAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, role := range []string{"phoenix_auth", "phoenix_tenant", "phoenix_admin"} {
		var tableAccess, sequenceAccess bool
		require.NoError(t, db.NewRaw(`SELECT
    has_table_privilege(?, 'auth.mfa_overrides', 'SELECT')
    AND has_table_privilege(?, 'auth.mfa_overrides', 'INSERT')
    AND has_table_privilege(?, 'auth.mfa_overrides', 'UPDATE')
    AND has_table_privilege(?, 'auth.mfa_overrides', 'DELETE')`, role, role, role, role).Scan(t.Context(), &tableAccess))
		require.True(t, tableAccess, role)
		require.NoError(t, db.NewRaw(`SELECT has_sequence_privilege(?, 'auth.mfa_overrides_id_seq', 'USAGE')`, role).Scan(t.Context(), &sequenceAccess))
		require.True(t, sequenceAccess, role)
	}
}

func TestDiagnosticsReaderDoesNotInheritNewTablesOrColumns(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	_, err = tx.ExecContext(t.Context(), `
CREATE TABLE facilities.diagnostics_future_table (id bigint, health_info text);
ALTER TABLE facilities.rooms ADD COLUMN diagnostics_future_health_info text;
`)
	require.NoError(t, err)
	var granted bool
	require.NoError(t, tx.NewRaw(`SELECT
    has_any_column_privilege('phoenix_diagnostics', 'facilities.diagnostics_future_table', 'SELECT,INSERT,UPDATE,REFERENCES')
    OR has_column_privilege('phoenix_diagnostics', 'facilities.rooms', 'diagnostics_future_health_info', 'SELECT')
`).Scan(t.Context(), &granted))
	require.False(t, granted)
	_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_diagnostics; SELECT id FROM facilities.diagnostics_future_table LIMIT 0`)
	require.ErrorContains(t, err, "SQLSTATE=42501")
}

func TestDiagnosticsReaderPersonalLoginCannotSwitchToRuntimeRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, role := range []string{"phoenix_auth", "phoenix_tenant", "phoenix_admin", "pg_read_all_data", "pg_read_server_files"} {
		t.Run(role, func(t *testing.T) {
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { require.NoError(t, tx.Rollback()) }()
			login := bun.Ident(fmt.Sprintf("diagnostics_test_%d", testpkg.Tenant(t)))
			_, err = tx.ExecContext(t.Context(), `CREATE ROLE ? LOGIN NOINHERIT; GRANT phoenix_diagnostics TO ?; SET LOCAL SESSION AUTHORIZATION ?; SET LOCAL ROLE phoenix_diagnostics`, login, login, login)
			require.NoError(t, err)
			_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE ?`, bun.Ident(role))
			require.ErrorContains(t, err, "SQLSTATE=42501", "exercise the personal session, not a superuser's SET ROLE")
		})
	}
}

func TestDiagnosticsReaderMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	// The production group is cluster-wide and referenced by template DBs.
	// Exercise the exact migration SQL with a private name, rolled back at exit.
	role := fmt.Sprintf("diagnostics_roundtrip_%d", testpkg.Tenant(t))
	up := strings.ReplaceAll(diagnosticsReaderUpSQL, "phoenix_diagnostics", role)
	down := strings.ReplaceAll(diagnosticsReaderDownSQL, "phoenix_diagnostics", role)
	for range 2 {
		_, err = tx.ExecContext(t.Context(), up)
		require.NoError(t, err, "up also supports an existing safe group")
	}
	_, err = tx.ExecContext(t.Context(), down)
	require.NoError(t, err)
	var exists bool
	require.NoError(t, tx.NewRaw(`SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = ?)`, role).Scan(t.Context(), &exists))
	require.False(t, exists, "down removes column ACLs and the group without dropping data")
}

func TestDiagnosticsReaderRejectsPreexistingElevatedRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	role := fmt.Sprintf("diagnostics_unsafe_%d", testpkg.Tenant(t))
	_, err = tx.ExecContext(t.Context(), `CREATE ROLE ? LOGIN`, bun.Ident(role))
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), strings.ReplaceAll(diagnosticsReaderUpSQL, "phoenix_diagnostics", role))
	require.ErrorContains(t, err, "must be an unprivileged NOLOGIN group")
}

func TestDiagnosticsReaderRunbookEvidenceQueries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	doc, err := os.ReadFile("../../../docs/operations/diagnose-lesezugang-3840.md")
	require.NoError(t, err)
	_, evidence, found := strings.Cut(string(doc), "## Inhaltsfreier produktiver Rechtenachweis")
	require.True(t, found)
	_, sql, found := strings.Cut(evidence, "```sql\n")
	require.True(t, found)
	sql, _, found = strings.Cut(sql, "```")
	require.True(t, found)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	login := bun.Ident(fmt.Sprintf("diagnostics_runbook_%d", testpkg.Tenant(t)))
	_, err = tx.ExecContext(t.Context(), `CREATE ROLE ? LOGIN NOINHERIT; GRANT phoenix_diagnostics TO ?; SET LOCAL SESSION AUTHORIZATION ?`, login, login, login)
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), sql)
	require.NoError(t, err, "run the published evidence SQL as a personal non-superuser session")
}
