package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type demoSchoolDatabaseErasure struct {
	Status    string           `json:"status"`
	RowCounts map[string]int64 `json:"row_counts"`
	ErrorCode string           `json:"error_code"`
}

// asDemoProcess runs one statement on the demo process's restricted role.
func asDemoProcess(t *testing.T, db *bun.DB, run func(context.Context, bun.Tx) error) {
	t.Helper()
	require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_demo`); err != nil {
			return err
		}
		return run(ctx, tx)
	}))
}

func dueRetiredDemoSchoolDatabases(t *testing.T, db *bun.DB) []int64 {
	t.Helper()
	ids := []int64{}
	asDemoProcess(t, db, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`SELECT id FROM platform.retired_demo_schools_due_for_database_erasure(50) AS id`).Scan(ctx, &ids)
	})
	return ids
}

func eraseRetiredDemoSchool(t *testing.T, db *bun.DB, schoolID int64) demoSchoolDatabaseErasure {
	t.Helper()
	var data string
	asDemoProcess(t, db, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`SELECT platform.erase_retired_demo_school(?)::text`, schoolID).Scan(ctx, &data)
	})
	var result demoSchoolDatabaseErasure
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	return result
}

// recordDemoOrigin lets an order name the school and erases the order, as
// the visitor erasure does (#4048): the trigger keeps the origin (ADR 0148).
func recordDemoOrigin(t *testing.T, db *bun.DB, schoolID int64) {
	t.Helper()
	name := fmt.Sprintf("ogs-db-erasure-%d", schoolID)
	erasureExec(t, db, `INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status) VALUES (?, ?, '{}'::jsonb, 'ready')`, name, schoolID)
	erasureExec(t, db, `DELETE FROM platform.demo_school_states WHERE name = ?`, name)
}

// retireSchool soft-deletes the school as the demo retirement does, the
// given number of days ago, and optionally writes the #5544 file marker.
func retireSchool(t *testing.T, db *bun.DB, schoolID int64, daysAgo int, filesErased bool) {
	t.Helper()
	erasureExec(t, db, `UPDATE platform.schools SET deleted_at = clock_timestamp() - make_interval(days => ?) WHERE id = ?`, daysAgo, schoolID)
	if filesErased {
		erasureExec(t, db, `INSERT INTO platform.demo_school_file_erasures (school_id, file_counts) VALUES (?, '{"files": 1}'::jsonb)`, schoolID)
	}
}

// typeVisitorContent writes what a visitor types into a demo school: a
// folder name, a stored file name, a setting text and an enrollment, each
// carrying token, plus a queued mail.
func typeVisitorContent(t *testing.T, db *bun.DB, schoolID, accountID int64, token string) {
	t.Helper()
	var folderID int64
	require.NoError(t, db.NewRaw(`INSERT INTO documents.folders (tenant_id, name, visibility, created_by) VALUES (?, ?, 'all_staff', ?) RETURNING id`,
		schoolID, "Ordner "+token, accountID).Scan(t.Context(), &folderID))
	erasureExec(t, db, `INSERT INTO documents.files (tenant_id, category, filename_display, filename_stored, size_bytes, content_type, uploaded_by, folder_id)
		VALUES (?, 'file', ?, ?, 12, 'application/pdf', ?, ?)`, schoolID, token+" Elternabend.pdf", fmt.Sprintf("demo-%d.pdf", schoolID), accountID, folderID)
	erasureExec(t, db, `UPDATE config.setting_values SET value = to_jsonb(?::text) WHERE tenant_id = ? AND setting_key = 'school.name'`, token, schoolID)
	erasureExec(t, db, `UPDATE enrollment.requests SET guardian_last_name = ? WHERE tenant_id = ?`, token, schoolID)
	erasureExec(t, db, `INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload)
		VALUES (?, 'fixture', jsonb_build_object('email', 'besuch@example.invalid'), jsonb_build_object('text', ?::text))`, schoolID, token)
}

// textOccurrences counts every row in every table whose text, character or
// JSON column holds token: a scan of the whole schema, not of the plan.
func textOccurrences(t *testing.T, db *bun.DB, token string) map[string]int {
	t.Helper()
	var columns []struct {
		Table  string `bun:"table_name"`
		Column string `bun:"column_name"`
	}
	require.NoError(t, db.NewRaw(`SELECT quote_ident(c.table_schema) || '.' || quote_ident(c.table_name) AS table_name, quote_ident(c.column_name) AS column_name
		FROM information_schema.columns AS c JOIN information_schema.tables AS tb USING (table_schema, table_name)
		WHERE tb.table_type = 'BASE TABLE' AND c.table_schema NOT IN ('pg_catalog', 'information_schema')
			AND c.data_type IN ('text', 'character varying', 'character', 'jsonb', 'json')`).Scan(t.Context(), &columns))
	found := map[string]int{}
	for _, column := range columns {
		var n int
		query := fmt.Sprintf(`SELECT count(*) FROM %s WHERE strpos(%s::text, ?) > 0`, column.Table, column.Column)
		require.NoError(t, db.NewRaw(query, token).Scan(t.Context(), &n))
		if n > 0 {
			found[column.Table+"."+column.Column] = n
		}
	}
	return found
}

// demoErasureSchoolTablesOutsideThePlan classifies every table that names a
// school by tenant_id or school_id but that the school erasure plan does not
// delete. Most belong to the Träger, which outlives its schools (ADR 0092).
// visitorErased tables hold personal rows that the visitor erasure
// (platform.erase_closed_demo_visitors) removes long before; the guard
// requires them empty for the erased school. retained tables keep rows on
// purpose and without content the visitor typed. A new table with such a
// column fails the guard until it is planned or classified here.
var demoErasureSchoolTablesOutsideThePlan = map[string]struct {
	retained bool
	reason   string
}{
	"care_contract.contracts":                  {false, "care contracts of a retired demo school go with its visitor (#4684)"},
	"care_contract.contract_events":            {false, "the trail goes with its contract (#4684)"},
	"parent_billing.care_invoices":             {false, "monthly invoices of a retired demo school go with its visitor (#4684)"},
	"parent_billing.care_meal_positions":       {false, "meal positions go with their invoice (#4686)"},
	"parent_billing.care_notices":              {false, "annual notices of a retired demo school go with its visitor (#4685)"},
	"schedule.meal_billing_months":             {false, "meal billing months of a retired demo school go with its visitor (#4686)"},
	"care_contract.contract_templates":         {true, "Träger contract terms of an offering, no person; stay with the Träger like after any school erasure"},
	"parent_billing.care_meal_tariffs":         {true, "Träger meal price per school, no person"},
	"parent_billing.care_price_lists":          {true, "Träger price list head per school, no person"},
	"enrollment.traeger_phase_schools":         {true, "Träger phase selection and outcome per school (ADR 0195), no person; stays with the Träger like after any school erasure"},
	"enrollment.traeger_care_offering_schools": {true, "Träger offering outcome per school (ADR 0195), no person; stays with the Träger like after any school erasure"},
	"parent_billing.care_run_schools":          {true, "school, month and run state of a Träger billing run, no person"},
	"parent_billing.care_tariffs":              {true, "Träger price per care module and weekday, no person"},
	"platform.billing_key_date_counts":         {true, "school and Träger name with sums for billing, no person; its own 10-year deadline (ADR 0092)"},
	"platform.school_erasures":                 {true, "evidence of an operator's school erasure; IDs and counts only"},
	"platform.traeger_document_reads":          {true, "Träger document reads keep their own three-year deadline; the visitor account is anonymized (#4048)"},
	"platform.traeger_document_releases":       {true, "a Träger release goes with its document or the Träger (ADR 0157)"},
	"workforce.staff_record_links":             {true, "an Einsatzort of a Träger staff record names the school by plain id and goes with its record or the Träger (ADR 0232)"},
	"workforce.staff_record_changes":           {true, "the trail of a Träger staff record names the school of a value and goes with its record or the Träger (ADR 0232)"},
	"platform.demo_school_file_erasures":       {true, "the #5544 file marker carries the database erasure evidence: time and row counts per table (ADR 0148, ADR 0189)"},
	"platform.demo_school_origins":             {true, "the demo origin: school ID and time only; it keeps a reused ID from ever looking like a real school (ADR 0148)"},
}

// TestRetiredDemoSchoolIsErasedFromTheDatabaseAfterNinetyDaysOnly is the
// acceptance test of #5559. One demo school is due; next to it are a demo
// school retired 89 days ago, an active demo school, a demo school an order
// still names, a retired demo school whose files are not erased yet, and two
// real schools, one retired long ago with a demo-like name and even a file
// marker but no demo origin. Only the due school goes, to the last row of
// what its visitor typed; every other school and the shared account stay.
func TestRetiredDemoSchoolIsErasedFromTheDatabaseAfterNinetyDaysOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	due := testpkg.Tenant(t)
	early, _ := testpkg.CreateTestTenant(t, db)
	active, _ := testpkg.CreateTestTenant(t, db)
	ordered, _ := testpkg.CreateTestTenant(t, db)
	filesPending, _ := testpkg.CreateTestTenant(t, db)
	realRetired, _ := testpkg.CreateTestTenant(t, db)
	realActive, _ := testpkg.CreateTestTenant(t, db)

	dueFixture := seedErasureSchool(t, db, due)
	token := fmt.Sprintf("Besuchertext-%d", due)
	typeVisitorContent(t, db, due, dueFixture.AccountID, token)
	realFixture := seedErasureSchool(t, db, realRetired)
	activeFixture := seedErasureSchool(t, db, realActive)
	for _, school := range []int64{early, active, ordered, filesPending} {
		testpkg.CreateTestStudentForTenant(t, db, school, "Demo", "Kind", "1a")
	}
	// A real account, also used in the due demo school, keeps its real school.
	testpkg.MapAccountToTenant(t, db, activeFixture.AccountID, due)

	for _, school := range []int64{due, early, active, ordered, filesPending} {
		recordDemoOrigin(t, db, school)
	}
	retireSchool(t, db, due, 91, true)
	retireSchool(t, db, early, 89, true)
	retireSchool(t, db, ordered, 120, true)
	retireSchool(t, db, filesPending, 120, false)
	retireSchool(t, db, realRetired, 400, true)
	erasureExec(t, db, `UPDATE platform.schools SET name = 'Demo-OGS Sonnenschein' WHERE id = ?`, realRetired)
	erasureExec(t, db, `INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status, visitor_school_ids)
		VALUES (?, ?, '{}'::jsonb, 'ready', ARRAY[?::bigint])`, fmt.Sprintf("ogs-still-named-%d", ordered), active, ordered)

	kept := []int64{early, active, ordered, filesPending, realRetired, realActive}
	before := map[int64]map[string]string{}
	for _, school := range kept {
		before[school] = erasureSnapshot(t, db, school)
	}
	typed := textOccurrences(t, db, token)
	require.NotEmpty(t, typed, "the fixture must hold the typed text before the erasure")
	for _, place := range []string{"documents.files.filename_display", "documents.folders.name", "enrollment.requests.guardian_last_name", "platform.email_outbox.payload"} {
		require.Contains(t, typed, place)
	}

	require.Equal(t, []int64{due}, dueRetiredDemoSchoolDatabases(t, db), "only the due demo school is selected")
	for _, school := range kept {
		assert.Equal(t, "not_due", eraseRetiredDemoSchool(t, db, school).Status, "school %d must not be erased", school)
	}

	result := eraseRetiredDemoSchool(t, db, due)
	require.Equal(t, "erased", result.Status, result.ErrorCode)
	assert.EqualValues(t, 1, result.RowCounts["platform.schools"], "exactly the school root")

	// Catalog: every table of the erasure plan holds no row of the school.
	for table, rows := range erasureSnapshot(t, db, due) {
		assert.Equal(t, "[]", rows, "due demo school: %s", table)
	}
	var roots int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.schools WHERE id = ?`, due).Scan(t.Context(), &roots))
	assert.Zero(t, roots, "the school root is gone")
	// Schema: no text anywhere still holds what the visitor typed.
	assert.Empty(t, textOccurrences(t, db, token))
	// Schema: outside the plan only the classified tables still name it.
	assertOnlyClassifiedTablesNameSchool(t, db, due)

	var evidence struct {
		ErasedAt  *string `bun:"db_erased_at"`
		RowCounts string  `bun:"db_row_counts"`
	}
	require.NoError(t, db.NewRaw(`SELECT db_erased_at::text, db_row_counts::text FROM platform.demo_school_file_erasures WHERE school_id = ?`, due).
		Scan(t.Context(), &evidence.ErasedAt, &evidence.RowCounts))
	require.NotNil(t, evidence.ErasedAt)
	assert.NotContains(t, evidence.RowCounts, token, "evidence holds counts only")

	for _, school := range kept {
		assert.Equal(t, before[school], erasureSnapshot(t, db, school), "school %d keeps every row", school)
	}
	var alive int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.accounts WHERE id IN (?, ?, ?) AND personal_data_erased_at IS NULL`,
		activeFixture.AccountID, realFixture.AccountID, dueFixture.AccountID).Scan(t.Context(), &alive))
	assert.Equal(t, 3, alive, "the erasure removes memberships, never an account")
	var realMemberships int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, activeFixture.AccountID, realActive).
		Scan(t.Context(), &realMemberships))
	assert.Equal(t, 1, realMemberships, "the shared account keeps its real school")

	// Repeating is a no-op.
	assert.Empty(t, dueRetiredDemoSchoolDatabases(t, db))
	assert.Equal(t, "not_due", eraseRetiredDemoSchool(t, db, due).Status)
}

func assertOnlyClassifiedTablesNameSchool(t *testing.T, db *bun.DB, schoolID int64) {
	t.Helper()
	planned := map[string]bool{}
	for _, entry := range readErasurePlan(t, db) {
		planned[entry.Name] = true
	}
	var columns []struct {
		Table  string `bun:"table_name"`
		Column string `bun:"column_name"`
	}
	require.NoError(t, db.NewRaw(`SELECT n.nspname || '.' || c.relname AS table_name, a.attname AS column_name
		FROM pg_class AS c JOIN pg_namespace AS n ON n.oid = c.relnamespace
		JOIN pg_attribute AS a ON a.attrelid = c.oid AND NOT a.attisdropped AND a.attnum > 0
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
			AND n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname !~ '^pg_'
			AND a.attname IN ('tenant_id', 'school_id')
		ORDER BY 1, 2`).Scan(t.Context(), &columns))
	var unclassified []string
	for _, column := range columns {
		if planned[column.Table] {
			continue
		}
		class, ok := demoErasureSchoolTablesOutsideThePlan[column.Table]
		if !ok {
			unclassified = append(unclassified, column.Table+"."+column.Column)
			continue
		}
		if class.retained {
			continue
		}
		var n int
		require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s = ?`, column.Table, column.Column), schoolID).Scan(t.Context(), &n))
		assert.Zero(t, n, "%s.%s: %s", column.Table, column.Column, class.reason)
	}
	assert.Empty(t, unclassified, "every table naming a school is in the erasure plan or classified in demoErasureSchoolTablesOutsideThePlan")
}

// A failure in the middle of the deletion leaves every row, writes no
// evidence and reports failed with the SQLSTATE only; the next run finishes.
func TestRetiredDemoSchoolDatabaseErasureRollsBackAndResumes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	due := testpkg.Tenant(t)
	seedErasureSchool(t, db, due)
	recordDemoOrigin(t, db, due)
	retireSchool(t, db, due, 91, true)
	before := erasureSnapshot(t, db, due)
	erasureExec(t, db, `CREATE FUNCTION platform.fail_demo_erasure_fixture() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'synthetic detail must not escape'; END $$;
		CREATE TRIGGER fail_demo_erasure_fixture BEFORE DELETE ON users.persons FOR EACH ROW EXECUTE FUNCTION platform.fail_demo_erasure_fixture()`)

	failed := eraseRetiredDemoSchool(t, db, due)
	require.Equal(t, "failed", failed.Status)
	assert.Equal(t, "P0001", failed.ErrorCode)
	assert.Empty(t, failed.RowCounts)
	assert.Equal(t, before, erasureSnapshot(t, db, due), "every deletion rolls back")
	var erasedAt *string
	require.NoError(t, db.NewRaw(`SELECT db_erased_at::text FROM platform.demo_school_file_erasures WHERE school_id = ?`, due).Scan(t.Context(), &erasedAt))
	assert.Nil(t, erasedAt, "a failure writes no evidence")
	assert.Equal(t, []int64{due}, dueRetiredDemoSchoolDatabases(t, db), "the school stays due")

	erasureExec(t, db, `DROP TRIGGER fail_demo_erasure_fixture ON users.persons; DROP FUNCTION platform.fail_demo_erasure_fixture()`)
	assert.Equal(t, "erased", eraseRetiredDemoSchool(t, db, due).Status)
	assert.Empty(t, dueRetiredDemoSchoolDatabases(t, db))
}

// A school table another transaction holds longer than the 100 ms lock
// timeout makes the school busy: nothing changes and the school stays due.
func TestRetiredDemoSchoolDatabaseErasureWaitsForALockedTable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	due := testpkg.Tenant(t)
	seedErasureSchool(t, db, due)
	recordDemoOrigin(t, db, due)
	retireSchool(t, db, due, 91, true)
	before := erasureSnapshot(t, db, due)

	holder, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback() })
	_, err = holder.ExecContext(t.Context(), `LOCK TABLE users.persons IN ROW EXCLUSIVE MODE`)
	require.NoError(t, err)

	assert.Equal(t, "busy", eraseRetiredDemoSchool(t, db, due).Status)
	assert.Equal(t, before, erasureSnapshot(t, db, due))
	require.NoError(t, holder.Rollback())
	assert.Equal(t, []int64{due}, dueRetiredDemoSchoolDatabases(t, db), "the school stays due")
	assert.Equal(t, "erased", eraseRetiredDemoSchool(t, db, due).Status)
}

// Only the demo process may erase, and it cannot forge the evidence. No
// serving role and not the operator's erasure role gets the new functions.
func TestRetiredDemoSchoolDatabaseErasureBelongsToTheDemoRoleOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	functions := []string{
		"platform.erase_retired_demo_school(bigint)",
		"platform.retired_demo_schools_due_for_database_erasure(integer)",
		"platform.retired_demo_school_database_due(bigint)",
	}
	for _, role := range []string{"phoenix_demo", "phoenix_auth", "phoenix_tenant", "phoenix_admin", "phoenix_school_erasure"} {
		for _, function := range functions {
			var granted bool
			require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, ?, 'EXECUTE')`, role, function).Scan(ctx, &granted))
			want := role == "phoenix_demo" && function != "platform.retired_demo_school_database_due(bigint)"
			assert.Equal(t, want, granted, "%s EXECUTE %s", role, function)
		}
		var update bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, 'platform.demo_school_file_erasures', 'UPDATE')`, role).Scan(ctx, &update))
		assert.False(t, update, "%s cannot change a marker", role)
	}

	schoolID := testpkg.Tenant(t)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_demo`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.demo_school_file_erasures (school_id, file_counts, db_erased_at, db_row_counts)
		VALUES (?, '{}'::jsonb, clock_timestamp(), '{}'::jsonb)`, schoolID)
	require.Error(t, err, "a file marker cannot claim a database erasure")
}

func TestDemoSchoolDatabaseErasureMigrationRoundTripPreservesEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	require.NoError(t, demoSchoolDatabaseErasureDown(ctx, db))
	require.NoError(t, demoSchoolDatabaseErasureUp(ctx, db))

	due := testpkg.Tenant(t)
	recordDemoOrigin(t, db, due)
	retireSchool(t, db, due, 91, true)
	require.Equal(t, "erased", eraseRetiredDemoSchool(t, db, due).Status)
	require.Error(t, demoSchoolDatabaseErasureDown(ctx, db), "rollback must not discard evidence")
}
