package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type erasurePlanEntry struct {
	Name      string   `json:"name"`
	Predicate string   `json:"predicate"`
	After     []string `json:"after,omitempty"`
	KeyColumn string   `json:"keyColumn,omitempty"`
}

type erasureResult struct {
	Status    string           `json:"status"`
	RowCounts map[string]int64 `json:"row_counts"`
	ErrorCode *string          `json:"error_code"`
}

func readErasurePlan(t *testing.T, db *bun.DB) []erasurePlanEntry {
	t.Helper()
	var data string
	require.NoError(t, db.NewRaw(`SELECT platform.school_erasure_plan()::text`).Scan(t.Context(), &data))
	var plan []erasurePlanEntry
	require.NoError(t, json.Unmarshal([]byte(data), &plan))
	return plan
}

func erasureCatalogPredicate(table lifecycleTable, catalog map[string]lifecycleTable, alias string, depth int) string {
	switch table.Scope {
	case "tenant":
		return alias + ".tenant_id = $1"
	case "school", "tenantReference":
		return alias + "." + table.ScopeColumn + " = $1"
	default:
		p := table.Parent
		parentAlias := fmt.Sprintf("p%d", depth)
		return fmt.Sprintf("%s.%s IN (SELECT %s.%s FROM %s AS %s WHERE %s)", alias, p.Column, parentAlias, p.ParentColumn, p.Table, parentAlias,
			erasureCatalogPredicate(catalog[p.Table], catalog, parentAlias, depth+1))
	}
}

func TestSchoolErasurePlanCoversCatalogAndDeleteTriggers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	catalog := readLifecycleCatalog(t)
	byName := make(map[string]lifecycleTable)
	for _, table := range catalog.Tables {
		byName[table.Name] = table
	}
	expected := make(map[string]string)
	for _, table := range catalog.Tables {
		switch table.Scope {
		case "tenantReference":
			if table.Name == "platform.billing_key_date_counts" {
				require.Equal(t, "excludeInternal", table.Export)
				require.Equal(t, ownRetention, tableRetentions[table.Name].class)
				continue // Explicit retained invoice exception, ADR 0092.
			}
			expected[table.Name] = erasureCatalogPredicate(table, byName, "r", 0)
		case "tenant", "tenantChild", "school":
			expected[table.Name] = erasureCatalogPredicate(table, byName, "r", 0)
		}
	}
	actual := make(map[string]string)
	for _, entry := range readErasurePlan(t, db) {
		actual[entry.Name] = entry.Predicate
	}
	require.Equal(t, expected, actual, "a new scoped table needs a migration updating the installed erasure plan")
	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_school(bigint,bigint)'::regprocedure) || pg_get_functiondef('platform.school_erasure_delete(bigint)'::regprocedure) || pg_get_functiondef('platform.school_erasure_assert_empty(bigint,jsonb)'::regprocedure)`).Scan(t.Context(), &definition))
	positions := make(map[string]int)
	for _, entry := range readErasurePlan(t, db) {
		predicate := strings.ReplaceAll(entry.Predicate, "$1", "p_school_id")
		statement := "DELETE FROM " + entry.Name + " AS r WHERE " + predicate + ";"
		require.Contains(t, definition, statement, "installed static deletion must match the catalog selector")
		require.Equal(t, 1, strings.Count(definition, "DELETE FROM "+entry.Name+" AS r WHERE "))
		positions[entry.Name] = strings.Index(definition, statement)
		if byName[entry.Name].Scope == "tenantChild" {
			require.Equal(t, byName[entry.Name].Parent.Column, entry.KeyColumn, "freeze indirect keys for the post-parent residual check")
			require.Contains(t, definition, "to_jsonb(r.\""+entry.KeyColumn+"\") IN (SELECT value FROM jsonb_array_elements(v_child_keys->'"+entry.Name+"'))")
		}
	}
	var edges []struct{ Child, Parent, Action string }
	require.NoError(t, db.NewRaw(`SELECT cn.nspname||'.'||cc.relname AS child,pn.nspname||'.'||pc.relname AS parent,f.confdeltype::text AS action
  FROM pg_constraint f JOIN pg_class cc ON cc.oid=f.conrelid JOIN pg_namespace cn ON cn.oid=cc.relnamespace
  JOIN pg_class pc ON pc.oid=f.confrelid JOIN pg_namespace pn ON pn.oid=pc.relnamespace WHERE f.contype='f'`).Scan(t.Context(), &edges))
	for _, edge := range edges {
		child, childOK := positions[edge.Child]
		parent, parentOK := positions[edge.Parent]
		if childOK && parentOK && edge.Child != edge.Parent && edge.Action != "n" && edge.Action != "d" {
			require.Less(t, child, parent, "restricting/cascading FK child %s precedes %s", edge.Child, edge.Parent)
		}
	}
	for _, entry := range readErasurePlan(t, db) {
		for _, source := range entry.After {
			require.Less(t, positions[source], positions[entry.Name], "trigger sink follows its source")
		}
	}
	var triggers []string
	require.NoError(t, db.NewRaw(`SELECT n.nspname||'.'||c.relname||':'||pn.nspname||'.'||p.proname
  FROM pg_trigger tr JOIN pg_class c ON c.oid=tr.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
  JOIN pg_proc p ON p.oid=tr.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
  WHERE NOT tr.tgisinternal AND (tr.tgtype::int & 8) <> 0 ORDER BY 1`).Scan(t.Context(), &triggers))
	assert.Equal(t, []string{
		"active.student_status_days:schedule.record_meal_sickness_status",
		"schedule.activity_instances:calendar.capture_activity_instance_feed_tombstones",
		"schedule.instance_staff:calendar.capture_instance_staff_feed_tombstone",
		"schedule.staff_shifts:calendar.capture_staff_shift_feed_tombstone",
	}, triggers, "review new DELETE side effects and add their sinks to the plan's dependency edges")
	var foreignKeys int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid='platform.school_erasures'::regclass`).Scan(t.Context(), &foreignKeys))
	assert.Zero(t, foreignKeys, "proofs must survive erasure of their referenced roots")
}

func erasureExec(t *testing.T, db *bun.DB, query string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(t.Context(), query, args...)
	require.NoError(t, err)
}

func beginErasure(t *testing.T, db *bun.DB, schoolID int64) int64 {
	t.Helper()
	operator := testpkg.CreateTestOperator(t, db)
	var id int64
	require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_school_erasure`); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT platform.begin_school_erasure(?, ?, ?)`, schoolID, operator.ID, fmt.Sprintf("fixture-order-%d", schoolID)).Scan(ctx, &id)
	}))
	return id
}

func eraseSchoolAsCLI(t *testing.T, db *bun.DB, schoolID, orderID int64) erasureResult {
	t.Helper()
	var data string
	require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_school_erasure`); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT platform.erase_school(?, ?)::text`, schoolID, orderID).Scan(ctx, &data)
	}))
	var result erasureResult
	require.NoError(t, json.Unmarshal([]byte(data), &result))
	return result
}

type erasureFixture struct{ AccountID, RoleID int64 }

// Both schools have real children, staff and guardian links, timetable, audit,
// outbox, settings, enrollment, billing and tenantChild rows. No API or worker
// is running: all data is synthetic and stays in the disposable local clone.
func seedErasureSchool(t *testing.T, db *bun.DB, schoolID int64) erasureFixture {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, db, schoolID, "Fixture", "Child", "1a")
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, schoolID, "Fixture", "Staff")
	testpkg.MapAccountToTenant(t, db, account.ID, schoolID)
	role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("erasure-%d", schoolID), schoolID)
	erasureExec(t, db, `INSERT INTO auth.account_roles(account_id,role_id,tenant_id) VALUES (?,?,?)`, account.ID, role.ID, schoolID)
	erasureExec(t, db, `INSERT INTO auth.role_permissions(role_id,permission_id) SELECT ?,id FROM auth.permissions ORDER BY id LIMIT 1`, role.ID)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, schoolID, "Fixture", "Guardian", "erasure-guardian")
	erasureExec(t, db, `UPDATE users.guardian_profiles SET account_id=?, has_account=TRUE WHERE id=?`, account.ID, guardian.ID)
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, schoolID, student.ID, guardian.ID, "custom")
	erasureExec(t, db, `INSERT INTO meta.meal_participation_permission_grants(student_guardian_id) VALUES (?)`, link.ID)
	erasureExec(t, db, `INSERT INTO meta.parent_student_consent_permission_grants(student_guardian_id) VALUES (?)`, link.ID)
	room := testpkg.CreateTestRoomForTenant(t, db, schoolID, "Erasure room")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, schoolID, "Erasure activity")
	instance := testpkg.CreateTestActivityInstanceForTenant(t, db, schoolID, calendar.NewDate(2026, time.October, 20), room.ID,
		testpkg.ActivityInstanceOpts{Title: "Erasure block"})
	testpkg.CreateTestInstanceStaffForTenant(t, db, schoolID, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	erasureExec(t, db, `INSERT INTO schedule.instance_students(tenant_id,instance_id,student_id) VALUES (?,?,?)`, schoolID, instance.ID, student.ID)
	erasureExec(t, db, `INSERT INTO activities.student_enrollments(tenant_id,activity_group_id,student_id) VALUES (?,?,?)`, schoolID, group.ID, student.ID)
	erasureExec(t, db, `INSERT INTO active.student_status_days(tenant_id,student_id,date,status,reported_at) VALUES (?,?,'2026-10-20','sick',NOW())`, schoolID, student.ID)
	erasureExec(t, db, `INSERT INTO schedule.staff_shifts(tenant_id,staff_id,created_by,date,start_time,end_time)
  VALUES (?,?,?,'2026-10-20','08:00','16:00')`, schoolID, staff.ID, staff.ID)
	erasureExec(t, db, `INSERT INTO config.setting_values(tenant_id,setting_key,value,updated_by)
  VALUES (?,'school.name','"synthetic-setting"'::jsonb,?)`, schoolID, account.ID)
	erasureExec(t, db, `INSERT INTO config.setting_audit(tenant_id,setting_key,action,changed_by) VALUES (?,'school.name','set',?)`, schoolID, account.ID)
	erasureExec(t, db, `INSERT INTO platform.email_outbox(tenant_id,kind,recipient,payload)
  VALUES (?,'fixture','{"email":"fixture@example.invalid"}'::jsonb,'{}'::jsonb)`, schoolID)
	erasureExec(t, db, `INSERT INTO audit.auth_events(tenant_id,account_id,event_type,ip_address) VALUES (?,?,'login','127.0.0.1')`, schoolID, account.ID)
	erasureExec(t, db, `INSERT INTO audit.data_access_log(tenant_id,actor_account_id,actor_role,resource_type,range_start,range_end)
  VALUES (?,?,'admin','students',NOW(),NOW())`, schoolID, account.ID)
	testpkg.CreateTestDataDeletionForTenant(t, db, schoolID, student.ID)
	erasureExec(t, db, `WITH model AS (INSERT INTO config.work_time_models(tenant_id,name,rotation_anchor_date)
  VALUES (?,'Fixture work model','2026-10-19') RETURNING id)
  INSERT INTO config.work_time_model_entries(model_id,week_index,day_of_week,target_minutes) SELECT id,0,1,480 FROM model`, schoolID)
	erasureExec(t, db, `WITH phase AS (INSERT INTO enrollment.phases(tenant_id,name,service_start_date,service_end_date)
  VALUES (?,'Fixture phase','2026-08-01','2027-07-31') RETURNING id),
  request AS (INSERT INTO enrollment.requests(tenant_id,phase_id,status_token,guardian_first_name,guardian_last_name,guardian_email)
  SELECT ?,id,?,'Fixture','Guardian','fixture@example.invalid' FROM phase RETURNING id)
  INSERT INTO enrollment.request_children(tenant_id,request_id,first_name,last_name,date_of_birth)
  SELECT ?,id,'Fixture','Child','2020-01-01' FROM request`, schoolID, schoolID, fmt.Sprintf("fixture-status-%d", schoolID), schoolID)
	erasureExec(t, db, `INSERT INTO platform.billing_key_date_counts(school_id,period,key_date,school_name,organization_name,active_students,active_terminals)
  VALUES (?,'2026-10-01','2026-10-15','Fixture school','Fixture organization',1,1)`, schoolID)
	return erasureFixture{AccountID: account.ID, RoleID: role.ID}
}

func erasureSnapshot(t *testing.T, db *bun.DB, schoolID int64) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	for _, entry := range readErasurePlan(t, db) {
		var data string
		query := fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb)::text FROM %s AS r WHERE %s`, entry.Name,
			strings.ReplaceAll(entry.Predicate, "$1", "?"))
		require.NoError(t, db.NewRaw(query, schoolID).Scan(t.Context(), &data))
		snapshot[entry.Name] = data
	}
	return snapshot
}

func TestSchoolErasureSeparatesSchoolsAndKeepsSharedAccounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, db)
	fixtureA := seedErasureSchool(t, db, schoolA)
	fixtureB := seedErasureSchool(t, db, schoolB)
	// This account belongs to A and B. A's exclusive account remains unmapped
	// afterwards, ready for auth.erase_unused_accounts, never directly deleted.
	testpkg.MapAccountToTenant(t, db, fixtureB.AccountID, schoolA)
	beforeA := erasureSnapshot(t, db, schoolA)
	beforeB := erasureSnapshot(t, db, schoolB)
	populated := 0
	for _, rows := range beforeA {
		if rows != "[]" {
			populated++
		}
	}
	require.GreaterOrEqual(t, populated, 30, "realistic fixture must exercise many catalog tables")
	t.Logf("two-school fixture populates %d scoped tables per school", populated)
	orderID := beginErasure(t, db, schoolA)
	result := eraseSchoolAsCLI(t, db, schoolA, orderID)
	require.Equal(t, "db_done", result.Status, result.ErrorCode)
	require.Nil(t, result.ErrorCode)
	afterA := erasureSnapshot(t, db, schoolA)
	for table, rows := range afterA {
		assert.Equal(t, "[]", rows, "school A: %s", table)
		assert.Contains(t, result.RowCounts, table)
		var originalRows []json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(beforeA[table]), &originalRows))
		assert.GreaterOrEqual(t, result.RowCounts[table], int64(len(originalRows)), "counts include every original row plus trigger-generated rows in %s", table)
	}
	assert.Equal(t, beforeB, erasureSnapshot(t, db, schoolB), "every row, not just counts, of B remains intact")
	var mappings int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_tenants WHERE account_id=?`, fixtureA.AccountID).Scan(t.Context(), &mappings))
	assert.Zero(t, mappings)
	var alive int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.accounts WHERE id IN (?,?) AND personal_data_erased_at IS NULL`, fixtureA.AccountID, fixtureB.AccountID).Scan(t.Context(), &alive))
	assert.Equal(t, 2, alive, "neither exclusive nor shared global account is directly erased")
	assert.Equal(t, result.RowCounts, eraseSchoolAsCLI(t, db, schoolA, orderID).RowCounts, "retry is an idempotent database no-op")
	var filesDone, completed *time.Time
	require.NoError(t, db.NewRaw(`SELECT files_done_at,completed_at FROM platform.school_erasures WHERE id=?`, orderID).Scan(t.Context(), &filesDone, &completed))
	assert.Nil(t, filesDone)
	assert.Nil(t, completed, "database erasure must not claim overall success")
}

func TestSchoolErasureRollsBackAndRetriesAfterMidDeleteFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	seedErasureSchool(t, db, schoolID)
	schoolB, _ := testpkg.CreateTestTenant(t, db)
	seedErasureSchool(t, db, schoolB)
	beforeB := erasureSnapshot(t, db, schoolB)
	before := erasureSnapshot(t, db, schoolID)
	orderID := beginErasure(t, db, schoolID)
	erasureExec(t, db, `CREATE FUNCTION platform.fail_erasure_fixture() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'synthetic detail must not be stored'; END $$;
 CREATE TRIGGER fail_erasure_fixture BEFORE DELETE ON users.persons FOR EACH ROW EXECUTE FUNCTION platform.fail_erasure_fixture()`)
	failed := eraseSchoolAsCLI(t, db, schoolID, orderID)
	require.Equal(t, "failed", failed.Status)
	require.NotNil(t, failed.ErrorCode)
	assert.Equal(t, "P0001", *failed.ErrorCode)
	assert.Empty(t, failed.RowCounts)
	assert.Equal(t, before, erasureSnapshot(t, db, schoolID), "all earlier deletes and trigger writes must roll back")
	assert.Equal(t, beforeB, erasureSnapshot(t, db, schoolB), "injected failure also leaves B intact")
	var proof string
	require.NoError(t, db.NewRaw(`SELECT platform.read_school_erasure(?)::text`, orderID).Scan(t.Context(), &proof))
	assert.NotContains(t, proof, "synthetic detail")
	assert.Contains(t, proof, `"db_done_at": null`)
	erasureExec(t, db, `DROP TRIGGER fail_erasure_fixture ON users.persons; DROP FUNCTION platform.fail_erasure_fixture()`)
	assert.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
}

func TestSchoolErasureRejectsCrossSchoolCascade(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, db)
	a := seedErasureSchool(t, db, schoolA)
	b := seedErasureSchool(t, db, schoolB)
	// A legacy single-column FK permits this polluted row. CASCADE would delete
	// B's role assignment; the erasure preflight must reject it instead.
	erasureExec(t, db, `INSERT INTO auth.account_roles(account_id,role_id,tenant_id) VALUES (?,?,?)`, b.AccountID, a.RoleID, schoolB)
	beforeA := erasureSnapshot(t, db, schoolA)
	beforeB := erasureSnapshot(t, db, schoolB)
	failed := eraseSchoolAsCLI(t, db, schoolA, beginErasure(t, db, schoolA))
	require.Equal(t, "failed", failed.Status)
	require.NotNil(t, failed.ErrorCode)
	assert.Equal(t, "23503", *failed.ErrorCode)
	assert.Equal(t, beforeA, erasureSnapshot(t, db, schoolA))
	assert.Equal(t, beforeB, erasureSnapshot(t, db, schoolB))
}

func TestSchoolErasurePrivilegesAndMatchingOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, db)
	orderID := beginErasure(t, db, schoolA)
	for _, role := range []string{"phoenix_auth", "phoenix_tenant", "phoenix_admin", "phoenix_demo"} {
		err := db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
			if _, e := tx.ExecContext(ctx, `SET LOCAL ROLE `+role); e != nil {
				return e
			}
			_, e := tx.ExecContext(ctx, `SELECT platform.erase_school(?,?)`, schoolA, orderID)
			return e
		})
		require.Error(t, err, "%s must not execute erasure", role)
		var retained bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, 'auth.mfa_overrides','SELECT')
   AND has_table_privilege(?, 'auth.mfa_overrides','INSERT') AND has_table_privilege(?, 'auth.mfa_overrides','UPDATE')
   AND has_table_privilege(?, 'auth.mfa_overrides','DELETE') AND has_sequence_privilege(?, 'auth.mfa_overrides_id_seq','USAGE')`, role, role, role, role, role).Scan(t.Context(), &retained))
		assert.True(t, retained, "narrowing PUBLIC must preserve the existing runtime role's MFA privileges")
	}
	var allowed bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_school_erasure','platform.school_erasures','SELECT,INSERT,UPDATE,DELETE')`).Scan(t.Context(), &allowed))
	assert.False(t, allowed, "CLI gets functions only, no general table access")
	var accessibleTables []string
	require.NoError(t, db.NewRaw(`SELECT n.nspname||'.'||c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE c.relkind IN ('r','p') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_'
  AND has_table_privilege('phoenix_school_erasure',c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE')`).Scan(t.Context(), &accessibleTables))
	assert.Empty(t, accessibleTables, "isolated role must not have general access to any application table")
	var unsafeFunctions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname='platform' AND p.proname IN ('begin_school_erasure','erase_school','read_school_erasure','delete_expired_school_erasures')
  AND (NOT p.prosecdef OR p.proconfig IS DISTINCT FROM ARRAY['search_path=pg_catalog']::text[]
   OR has_function_privilege('phoenix_auth',p.oid,'EXECUTE') OR has_function_privilege('phoenix_tenant',p.oid,'EXECUTE')
   OR has_function_privilege('phoenix_admin',p.oid,'EXECUTE') OR has_function_privilege('phoenix_demo',p.oid,'EXECUTE'))`).Scan(t.Context(), &unsafeFunctions))
	assert.Zero(t, unsafeFunctions, "all four capabilities have fixed search paths and reject web roles")
	var exposedHelpers int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname='platform' AND p.proname IN ('school_erasure_preflight','school_erasure_delete','school_erasure_assert_empty')
  AND (p.prosecdef OR p.proconfig IS DISTINCT FROM ARRAY['search_path=pg_catalog']::text[]
   OR has_function_privilege('phoenix_school_erasure',p.oid,'EXECUTE') OR has_function_privilege('phoenix_auth',p.oid,'EXECUTE')
   OR has_function_privilege('phoenix_tenant',p.oid,'EXECUTE') OR has_function_privilege('phoenix_admin',p.oid,'EXECUTE')
   OR has_function_privilege('phoenix_demo',p.oid,'EXECUTE'))`).Scan(t.Context(), &exposedHelpers))
	assert.Zero(t, exposedHelpers, "internal phases are invoker-only and not a bypass for order validation")
	err := db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, e := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_school_erasure`); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `SELECT platform.erase_school(?,?)`, schoolB, orderID)
		return e
	})
	require.Error(t, err, "order of A never authorizes deleting B")
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM platform.school_erasures WHERE id=?`, orderID).Scan(t.Context(), &status))
	assert.Equal(t, "pending", status)
}

func TestSchoolErasureFailsClosedForNewTenantTablesAndSuppressedDelete(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	orderID := beginErasure(t, db, schoolID)
	erasureExec(t, db, `CREATE TABLE platform.erasure_unknown_fixture(tenant_id bigint)`)
	failed := eraseSchoolAsCLI(t, db, schoolID, orderID)
	require.Equal(t, "failed", failed.Status)
	assert.Equal(t, "55000", *failed.ErrorCode)
	erasureExec(t, db, `DROP TABLE platform.erasure_unknown_fixture;
 CREATE TABLE platform.erasure_fk_fixture(school_id bigint REFERENCES platform.schools(id))`)
	failed = eraseSchoolAsCLI(t, db, schoolID, orderID)
	require.Equal(t, "failed", failed.Status)
	assert.Equal(t, "55000", *failed.ErrorCode, "unknown FK changes require regenerating scope checks and order")
	erasureExec(t, db, `DROP TABLE platform.erasure_fk_fixture;
 CREATE FUNCTION platform.suppress_erasure_fixture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$;
 CREATE TRIGGER suppress_erasure_fixture BEFORE DELETE ON platform.schools FOR EACH ROW EXECUTE FUNCTION platform.suppress_erasure_fixture()`)
	failed = eraseSchoolAsCLI(t, db, schoolID, orderID)
	require.Equal(t, "failed", failed.Status)
	assert.Equal(t, "55000", *failed.ErrorCode, "residual check catches a DELETE suppressed by a trigger")
	erasureExec(t, db, `DROP TRIGGER suppress_erasure_fixture ON platform.schools; DROP FUNCTION platform.suppress_erasure_fixture()`)
	assert.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
}

func TestSchoolErasureProofRetentionAndCompletedIdempotence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	orderID := beginErasure(t, db, schoolID)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
	// Later phases belong to 3837-c. Arrange completed proof as the DB owner
	// solely to exercise its retention and no-repeat contracts.
	erasureExec(t, db, `UPDATE platform.school_erasures SET status='completed', files_done_at=NOW(),completed_at=NOW() WHERE id=?`, orderID)
	assert.Equal(t, "completed", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	pendingID := beginErasure(t, db, otherSchool)
	erasureExec(t, db, `UPDATE platform.school_erasures SET created_at=NOW()-interval '4 years' WHERE id=?`, pendingID)
	var deleted int64
	require.NoError(t, db.NewRaw(`SELECT platform.delete_expired_school_erasures()`).Scan(t.Context(), &deleted))
	assert.Zero(t, deleted, "recent evidence and unresolved orders stay")
	erasureExec(t, db, `UPDATE platform.school_erasures SET completed_at=NOW()-interval '3 years 1 day' WHERE id=?`, orderID)
	require.NoError(t, db.NewRaw(`SELECT platform.delete_expired_school_erasures()`).Scan(t.Context(), &deleted))
	assert.EqualValues(t, 1, deleted)
	var ids []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM platform.school_erasures ORDER BY id`).Scan(t.Context(), &ids))
	assert.Equal(t, []int64{pendingID}, ids)
}

func TestSchoolErasureDetectsSuppressedIndirectChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	role := testpkg.CreateTestRoleForTenant(t, db, "erasure-suppressed-child", schoolID)
	erasureExec(t, db, `INSERT INTO auth.role_permissions(role_id,permission_id) SELECT ?,id FROM auth.permissions ORDER BY id LIMIT 1`, role.ID)
	orderID := beginErasure(t, db, schoolID)
	erasureExec(t, db, `CREATE FUNCTION platform.suppress_child_fixture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$;
 CREATE TRIGGER suppress_child_fixture BEFORE DELETE ON auth.role_permissions FOR EACH ROW EXECUTE FUNCTION platform.suppress_child_fixture()`)
	failed := eraseSchoolAsCLI(t, db, schoolID, orderID)
	require.Equal(t, "failed", failed.Status)
	assert.Equal(t, "55000", *failed.ErrorCode, "frozen keys detect the suppressed child even after its parent disappeared")
	var roots int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.schools WHERE id=?`, schoolID).Scan(t.Context(), &roots))
	assert.Equal(t, 1, roots, "residual failure rolls back deletion of the school root")
	erasureExec(t, db, `DROP TRIGGER suppress_child_fixture ON auth.role_permissions; DROP FUNCTION platform.suppress_child_fixture()`)
	assert.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
}

func TestSchoolErasureRejectsPostCommitTenantWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "erasure-late-writer")
	orderID := beginErasure(t, db, schoolID)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
	_, err := db.ExecContext(t.Context(), `INSERT INTO auth.mfa_email_challenges(account_id,tenant_id,scope,code_hash,expires_at)
  VALUES (?,?,'','test-placeholder',NOW()+interval '1 hour')`, account.ID, schoolID)
	require.Error(t, err, "a writer queued before erasure must not recreate personal tenant rows after commit")
}

func TestEveryTenantTableHasASchoolErasureWriteFence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var missing []string
	require.NoError(t, db.NewRaw(`SELECT n.nspname||'.'||c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  JOIN pg_attribute a ON a.attrelid=c.oid AND a.attname='tenant_id' AND NOT a.attisdropped
  WHERE c.relkind IN ('r','p') AND NOT c.relispartition AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_'
  AND NOT EXISTS (SELECT FROM pg_constraint f JOIN pg_attribute parent_id ON parent_id.attrelid=f.confrelid AND parent_id.attname='id'
   WHERE f.contype='f' AND f.conrelid=c.oid AND f.confrelid='platform.schools'::regclass
    AND f.conkey=ARRAY[a.attnum] AND f.confkey=ARRAY[parent_id.attnum] AND f.convalidated)
  ORDER BY 1`).Scan(t.Context(), &missing))
	require.Empty(t, missing, "every tenant table needs a validated school FK so queued writers cannot recreate erased rows")
}

func TestSchoolErasureMigrationRoundTripPreservesEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	require.NoError(t, schoolErasureDown(t.Context(), db))
	require.NoError(t, schoolErasureUp(t.Context(), db))
	orderID := beginErasure(t, db, schoolID)
	require.Error(t, schoolErasureDown(t.Context(), db), "rollback must not discard any pending or completed proof")
	assert.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status, "failed rollback preserves evidence and executable procedure")
}
