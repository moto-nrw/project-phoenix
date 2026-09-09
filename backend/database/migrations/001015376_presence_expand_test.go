package migrations

// Migration tests stay internal: the active architecture policy forbids an
// external module-behavior-test from importing a migration adapter. Exercise
// the registered deployment entry points below without widening that policy.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Exercise the same registered entry points the deployment runner uses.
func runPresenceMigration(ctx context.Context, db *testpkg.DB, name string, up bool) error {
	runner := testpkg.NewMigrator(db, Migrations)
	for _, migration := range Migrations.Sorted() {
		if migration.Name == name {
			if up {
				return migration.Up(ctx, runner, &migration)
			}
			return migration.Down(ctx, runner, &migration)
		}
	}
	return fmt.Errorf("migration %s is not registered", name)
}

func TestPresenceExpandColumnMapping(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, tc := range []struct {
		target, source string
		columns        []string
	}{
		{"activity_sessions", "activity_instances", []string{
			"id", "tenant_id", "schedule_instance_id", "status", "active_group_id", "started_by", "started_at",
			"completed_at", "completed_by", "reopen_until", "completion_snapshot", "created_at", "updated_at",
		}},
		{"activity_session_attendance", "instance_students", []string{
			"id", "tenant_id", "instance_student_id", "status", "substatus", "note", "checked_in_at", "checked_out_at",
			"is_unplanned", "not_scheduled", "manual_status_at", "student_status_day_id", "pickup_exception_id", "created_at", "updated_at",
		}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			var columns []string
			require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
				WHERE table_schema = 'active' AND table_name = ? ORDER BY ordinal_position`, tc.target).Scan(t.Context(), &columns))
			require.ElementsMatch(t, tc.columns, columns)
			// Every moved field keeps its SQL type and nullability. Lifecycle status
			// alone changes default/domain; old planned/cancelled state stays put.
			for _, column := range tc.columns {
				if column == "schedule_instance_id" || column == "instance_student_id" {
					continue
				}
				var same bool
				require.NoError(t, db.NewRaw(`SELECT old.data_type = target.data_type
					AND old.is_nullable = target.is_nullable
					AND (old.column_name = 'id' OR (old.column_name = 'status' AND target.table_name = 'activity_sessions')
						OR old.column_default IS NOT DISTINCT FROM target.column_default)
					FROM information_schema.columns old JOIN information_schema.columns target USING (column_name)
					WHERE old.table_schema = 'schedule' AND old.table_name = ?
					AND target.table_schema = 'active' AND target.table_name = ? AND old.column_name = ?`,
					tc.source, tc.target, column).Scan(t.Context(), &same))
				require.True(t, same, "mapping of %s", column)
			}
		})
	}
}

type presenceExpandFixture struct {
	tenant, instance, participant, staff, group, statusDay, pickup int64
}

func createPresenceExpandFixture(t *testing.T, db *testpkg.DB) presenceExpandFixture {
	t.Helper()
	f := presenceExpandFixture{tenant: testpkg.UniqueTestTenantID(t)}
	testpkg.EnsureTestTenant(t, db, f.tenant)
	room := testpkg.CreateTestRoomForTenant(t, db, f.tenant, "Presence Expand")
	student := testpkg.CreateTestStudentForTenant(t, db, f.tenant, "Presence", "Expand", "2a")
	f.staff = testpkg.CreateTestStaffForTenant(t, db, f.tenant, "Presence", "Staff").ID
	f.group = testpkg.CreateTestActiveGroupForTenant(t, db, f.tenant).ID
	f.instance = testpkg.CreateTestActivityInstanceForTenant(t, db, f.tenant, testpkg.Date(2026, 9, 9), room.ID,
		testpkg.ActivityInstanceOpts{Status: "active", ActiveGroupID: &f.group}).ID
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.instance_students (tenant_id, instance_id, student_id, status)
		VALUES (?, ?, ?, 'present') RETURNING id`, f.tenant, f.instance, student.ID).Scan(t.Context(), &f.participant))
	require.NoError(t, db.NewRaw(`INSERT INTO active.student_status_days (tenant_id, student_id, date, status, source, reported_at)
		VALUES (?, ?, '2026-09-09', 'sick', 'manual', NOW()) RETURNING id`, f.tenant, student.ID).Scan(t.Context(), &f.statusDay))
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.student_pickup_exceptions (tenant_id, student_id, exception_date, created_by)
		VALUES (?, ?, '2026-09-09', ?) RETURNING id`, f.tenant, student.ID, f.staff).Scan(t.Context(), &f.pickup))
	return f
}

func requirePresenceSQLState(t *testing.T, err error, state string) {
	t.Helper()
	var pgErr interface {
		error
		Field(byte) string
	}
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, state, pgErr.Field('C'), "%v", err)
}

func TestPresenceExpandTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a, b := createPresenceExpandFixture(t, db), createPresenceExpandFixture(t, db)
	for _, table := range []string{"activity_sessions", "activity_session_attendance"} {
		t.Run(table, func(t *testing.T) {
			column := "schedule_instance_id"
			parents := []int64{a.instance, b.instance}
			if table == "activity_session_attendance" {
				column, parents = "instance_student_id", []int64{a.participant, b.participant}
			}
			insert := fmt.Sprintf("INSERT INTO active.%s (tenant_id, %s) VALUES (?, ?)", table, column)
			for i, own := range []presenceExpandFixture{a, b} {
				require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, insert, own.tenant, parents[i])
					return err
				}))
			}
			for i, own := range []presenceExpandFixture{a, b} {
				other := b
				if i == 1 {
					other = a
				}
				var tenants []int64
				require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					return tx.NewRaw("SELECT tenant_id FROM active."+table).Scan(ctx, &tenants)
				}))
				require.Equal(t, []int64{own.tenant}, tenants, "unfiltered reads must remain tenant scoped")
				for _, query := range []string{
					"UPDATE active." + table + " SET updated_at = NOW() WHERE tenant_id = ?",
					"DELETE FROM active." + table + " WHERE tenant_id = ?",
				} {
					var affected int64
					require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
						result, err := tx.ExecContext(ctx, query, other.tenant)
						if err != nil {
							return err
						}
						affected, err = result.RowsAffected()
						return err
					}))
					require.Zero(t, affected)
				}
				for _, query := range []string{insert, "UPDATE active." + table + " SET tenant_id = ? WHERE " + column + " = ?"} {
					err := testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
						_, err := tx.ExecContext(ctx, query, other.tenant, parents[i])
						return err
					})
					requirePresenceSQLState(t, err, "42501")
				}
			}
			// An absent school context is deny-by-default, not a global read.
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_tenant; SELECT set_config('app.current_tenant_id', '', true)`)
			require.NoError(t, err)
			var count int
			require.NoError(t, tx.NewRaw("SELECT count(*) FROM active."+table).Scan(t.Context(), &count))
			require.Zero(t, count)
			_, err = tx.ExecContext(t.Context(), insert, a.tenant, parents[0])
			requirePresenceSQLState(t, err, "42501")
		})
	}
}

func TestPresenceExpandConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a, b := createPresenceExpandFixture(t, db), createPresenceExpandFixture(t, db)
	_, err := db.ExecContext(t.Context(), `INSERT INTO active.activity_sessions
		(tenant_id, schedule_instance_id, active_group_id, started_by) VALUES (?, ?, ?, ?)`, a.tenant, a.instance, a.group, a.staff)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO active.activity_session_attendance
		(tenant_id, instance_student_id, student_status_day_id, pickup_exception_id) VALUES (?, ?, ?, ?)`, a.tenant, a.participant, a.statusDay, a.pickup)
	require.NoError(t, err)
	for _, tc := range []struct {
		table, column string
		foreignID     int64
	}{
		{"activity_sessions", "schedule_instance_id", b.instance},
		{"activity_sessions", "active_group_id", b.group},
		{"activity_sessions", "started_by", b.staff},
		{"activity_session_attendance", "instance_student_id", b.participant},
		{"activity_session_attendance", "student_status_day_id", b.statusDay},
		{"activity_session_attendance", "pickup_exception_id", b.pickup},
	} {
		t.Run(tc.column, func(t *testing.T) {
			_, err := db.ExecContext(t.Context(), fmt.Sprintf("UPDATE active.%s SET %s = ? WHERE tenant_id = ?", tc.table, tc.column), tc.foreignID, a.tenant)
			requirePresenceSQLState(t, err, "23503")
		})
	}
	for _, status := range []string{"planned", "cancelled", "unknown"} {
		_, err := db.ExecContext(t.Context(), `UPDATE active.activity_sessions SET status = ?`, status)
		requirePresenceSQLState(t, err, "23514")
	}
	for _, status := range []string{"active", "completed"} {
		_, err := db.ExecContext(t.Context(), `UPDATE active.activity_sessions SET status = ?`, status)
		require.NoError(t, err)
	}
	for _, status := range []string{"expected", "present", "absent"} {
		_, err := db.ExecContext(t.Context(), `UPDATE active.activity_session_attendance SET status = ?`, status)
		require.NoError(t, err)
	}
	for _, substatus := range []string{"late", "excused", "sick", "field_trip", "other"} {
		_, err := db.ExecContext(t.Context(), `UPDATE active.activity_session_attendance SET substatus = ?`, substatus)
		require.NoError(t, err)
	}
	for _, query := range []string{
		`UPDATE active.activity_session_attendance SET status = 'unknown'`,
		`UPDATE active.activity_session_attendance SET substatus = 'unknown'`,
		`UPDATE active.activity_session_attendance SET note = repeat('ü', 501)`,
	} {
		_, err := db.ExecContext(t.Context(), query)
		requirePresenceSQLState(t, err, "23514")
	}
	_, err = db.ExecContext(t.Context(), `UPDATE active.activity_session_attendance SET substatus = NULL, note = ?`, strings.Repeat("ü", 500))
	require.NoError(t, err)
	for _, table := range []string{"activity_sessions", "activity_session_attendance"} {
		column, parent := "schedule_instance_id", a.instance
		if table == "activity_session_attendance" {
			column, parent = "instance_student_id", a.participant
		}
		_, err := db.ExecContext(t.Context(), fmt.Sprintf("INSERT INTO active.%s (tenant_id, %s) VALUES (?, ?)", table, column), a.tenant, parent)
		requirePresenceSQLState(t, err, "23505")
	}
}

func presenceOldStorageSnapshot(t *testing.T, db *testpkg.DB) []string {
	t.Helper()
	var result []string
	for _, table := range []string{"activity_instances", "instance_students"} {
		for _, query := range []string{
			"SELECT COALESCE(jsonb_agg(to_jsonb(row) ORDER BY id), '[]')::text FROM schedule." + table + " row",
			`SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY ordinal_position), '[]')::text FROM information_schema.columns c WHERE table_schema = 'schedule' AND table_name = ?`,
			`SELECT COALESCE(jsonb_agg(pg_get_constraintdef(oid) ORDER BY conname), '[]')::text FROM pg_constraint WHERE conrelid = ?::regclass`,
			`SELECT COALESCE(jsonb_agg(indexdef ORDER BY indexname), '[]')::text FROM pg_indexes WHERE schemaname = 'schedule' AND tablename = ?`,
			`SELECT COALESCE(jsonb_agg(pg_get_triggerdef(oid) ORDER BY tgname), '[]')::text FROM pg_trigger WHERE tgrelid = ?::regclass AND NOT tgisinternal`,
		} {
			var snapshot string
			var args []any
			if strings.Contains(query, "?::regclass") {
				args = []any{"schedule." + table}
			} else if strings.Contains(query, "?") {
				args = []any{table}
			}
			require.NoError(t, db.NewRaw(query, args...).Scan(t.Context(), &snapshot))
			result = append(result, snapshot)
		}
	}
	return result
}

func assertPresenceTargetsEmpty(t *testing.T, db *testpkg.DB) {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM active.activity_sessions)
		+ (SELECT count(*) FROM active.activity_session_attendance)`).Scan(t.Context(), &count))
	require.Zero(t, count, "Expand must never copy or dual-write old rows")
}

func TestPresenceExpandUpDownPreservesOldAuthority(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	assertPresenceTargetsEmpty(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015376", false))
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015375", false))
	beforePrerequisite := presenceOldStorageSnapshot(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015375", true))
	afterPrerequisite := presenceOldStorageSnapshot(t, db)
	// The approved prerequisite changes exactly one index inventory, not data,
	// columns, constraints, or application triggers on either old table.
	for i := range beforePrerequisite {
		if i != 8 {
			require.Equal(t, beforePrerequisite[i], afterPrerequisite[i])
		}
	}
	require.NotEqual(t, beforePrerequisite[8], afterPrerequisite[8])
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015376", true))
	require.Equal(t, afterPrerequisite, presenceOldStorageSnapshot(t, db))
	assertPresenceTargetsEmpty(t, db)
	// The previous application's SQL shape can still complete, reopen, plan,
	// cancel and record attendance. No target row is required or produced.
	for _, status := range []string{"completed", "active", "planned", "cancelled"} {
		require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, f.tenant, func(ctx context.Context, tx testpkg.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = ?, started_at = NOW(),
				completed_at = NOW(), reopen_until = NOW(), completion_snapshot = '{"students":[]}' WHERE id = ?`, status, f.instance)
			return err
		}))
	}
	require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, f.tenant, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE schedule.instance_students SET status = 'absent', substatus = 'excused',
			note = 'retained', checked_in_at = NOW(), checked_out_at = NOW(), is_unplanned = TRUE,
			not_scheduled = TRUE, manual_status_at = NOW(), student_status_day_id = ?, pickup_exception_id = ? WHERE id = ?`,
			f.statusDay, f.pickup, f.participant)
		return err
	}))
	assertPresenceTargetsEmpty(t, db)
	beforeDown := presenceOldStorageSnapshot(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015376", false))
	require.Equal(t, beforeDown, presenceOldStorageSnapshot(t, db))
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'active' AND (c.relname LIKE '%activity_sessions%' OR c.relname LIKE '%activity_session_attendance%')`).Scan(t.Context(), &remaining))
	require.Zero(t, remaining, "tables, indexes and owned sequences must all be gone")
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015375", false))
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015375", true))
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015376", true))
	assertPresenceTargetsEmpty(t, db)
}

func TestPresenceExpandRollbackRefusesPopulatedTargets(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	for _, tc := range []struct {
		table, column string
		parent        int64
	}{
		{"activity_sessions", "schedule_instance_id", f.instance},
		{"activity_session_attendance", "instance_student_id", f.participant},
	} {
		_, err := db.ExecContext(t.Context(), fmt.Sprintf("INSERT INTO active.%s (tenant_id, %s) VALUES (?, ?)", tc.table, tc.column), f.tenant, tc.parent)
		require.NoError(t, err)
		require.ErrorContains(t, runPresenceMigration(t.Context(), db, "001015376", false), "requires empty target tables")
		var count int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM active."+tc.table).Scan(t.Context(), &count))
		require.Equal(t, 1, count)
		_, err = db.ExecContext(t.Context(), "DELETE FROM active."+tc.table)
		require.NoError(t, err)
	}
	// The prerequisite also refuses to cascade through the target FK.
	require.Error(t, runPresenceMigration(t.Context(), db, "001015375", false))
	assertPresenceTargetsEmpty(t, db)
}

func TestPresenceExpandReferenceDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	account := testpkg.CreateTestAccount(t, db, "presence-expand@example.test")
	_, err := db.ExecContext(t.Context(), `INSERT INTO active.activity_sessions
		(tenant_id, schedule_instance_id, active_group_id, started_by, completed_by)
		VALUES (?, ?, ?, ?, ?)`, f.tenant, f.instance, f.group, f.staff, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO active.activity_session_attendance
		(tenant_id, instance_student_id, student_status_day_id, pickup_exception_id)
		VALUES (?, ?, ?, ?)`, f.tenant, f.participant, f.statusDay, f.pickup)
	require.NoError(t, err)
	for _, tc := range []struct {
		table string
		id    int64
	}{
		{"active.student_status_days", f.statusDay},
		{"schedule.student_pickup_exceptions", f.pickup},
		{"active.groups", f.group},
		{"users.staff", f.staff},
		{"auth.accounts", account.ID},
	} {
		_, err := db.ExecContext(t.Context(), "DELETE FROM "+tc.table+" WHERE id = ?", tc.id)
		require.NoError(t, err)
	}
	var preserved bool
	require.NoError(t, db.NewRaw(`SELECT tenant_id = ? AND active_group_id IS NULL AND started_by IS NULL AND completed_by IS NULL
		FROM active.activity_sessions WHERE schedule_instance_id = ?`, f.tenant, f.instance).Scan(t.Context(), &preserved))
	require.True(t, preserved, "clear optional actor/live links without deleting the session or its tenant")
	require.NoError(t, db.NewRaw(`SELECT tenant_id = ? AND student_status_day_id IS NULL AND pickup_exception_id IS NULL
		FROM active.activity_session_attendance WHERE instance_student_id = ?`, f.tenant, f.participant).Scan(t.Context(), &preserved))
	require.True(t, preserved, "clear provenance without deleting attendance or its tenant")
	_, err = db.ExecContext(t.Context(), `DELETE FROM schedule.activity_instances WHERE id = ?`, f.instance)
	require.NoError(t, err)
	assertPresenceTargetsEmpty(t, db)
}

func TestPresenceExpandCatalogAndDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, table := range []string{"activity_sessions", "activity_session_attendance"} {
		var forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid = ?::regclass`, "active."+table).Scan(t.Context(), &forced))
		require.True(t, forced)
		var policies []string
		require.NoError(t, db.NewRaw(`SELECT policyname FROM pg_policies WHERE schemaname = 'active' AND tablename = ?`, table).Scan(t.Context(), &policies))
		require.Equal(t, []string{"tenant_isolation_active_" + table}, policies)
		var invalid int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_index WHERE indrelid = ?::regclass AND NOT indisvalid`, "active."+table).Scan(t.Context(), &invalid))
		require.Zero(t, invalid)
		var customTriggers int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_trigger WHERE tgrelid = ?::regclass AND NOT tgisinternal`, "active."+table).Scan(t.Context(), &customTriggers))
		require.Zero(t, customTriggers)
	}
	for index, columns := range map[string]string{
		"uq_activity_sessions_instance":                    "tenant_id, schedule_instance_id",
		"uq_activity_sessions_active_group":                "tenant_id, active_group_id",
		"idx_activity_sessions_status":                     "tenant_id, status",
		"idx_activity_sessions_started_by":                 "tenant_id, started_by",
		"idx_activity_sessions_completed_by":               "completed_by",
		"idx_activity_sessions_reopenable":                 "tenant_id, reopen_until",
		"uq_activity_session_attendance_participant":       "tenant_id, instance_student_id",
		"idx_activity_session_attendance_status":           "tenant_id, status",
		"idx_activity_session_attendance_status_day":       "tenant_id, student_status_day_id",
		"idx_activity_session_attendance_pickup_exception": "tenant_id, pickup_exception_id",
	} {
		var actual string
		require.NoError(t, db.NewRaw(`SELECT string_agg(pg_get_indexdef(indexrelid, n, true), ', ' ORDER BY n)
			FROM pg_index CROSS JOIN LATERAL generate_series(1, indnkeyatts) n
			WHERE indexrelid = ?::regclass`, "active."+index).Scan(t.Context(), &actual))
		require.Equal(t, columns, actual, index)
	}
	f := createPresenceExpandFixture(t, db)
	var status string
	require.NoError(t, db.NewRaw(`INSERT INTO active.activity_sessions (tenant_id, schedule_instance_id) VALUES (?, ?) RETURNING status`, f.tenant, f.instance).Scan(t.Context(), &status))
	require.Equal(t, "active", status)
	var defaults bool
	require.NoError(t, db.NewRaw(`INSERT INTO active.activity_session_attendance (tenant_id, instance_student_id) VALUES (?, ?)
		RETURNING status = 'expected' AND NOT is_unplanned AND NOT not_scheduled
		AND substatus IS NULL AND note IS NULL AND checked_in_at IS NULL AND checked_out_at IS NULL
		AND manual_status_at IS NULL AND student_status_day_id IS NULL AND pickup_exception_id IS NULL
		AND created_at IS NOT NULL AND updated_at IS NOT NULL`, f.tenant, f.participant).Scan(t.Context(), &defaults))
	require.True(t, defaults)
	// A second occurrence cannot claim the same live group.
	room := testpkg.CreateTestRoomForTenant(t, db, f.tenant, "Second occurrence")
	second := testpkg.CreateTestActivityInstanceForTenant(t, db, f.tenant, testpkg.Date(2026, 9, 10), room.ID, testpkg.ActivityInstanceOpts{})
	_, err := db.ExecContext(t.Context(), `UPDATE active.activity_sessions SET active_group_id = ?`, f.group)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO active.activity_sessions (tenant_id, schedule_instance_id, active_group_id) VALUES (?, ?, ?)`, f.tenant, second.ID, f.group)
	requirePresenceSQLState(t, err, "23505")
}

func TestPresenceExpandFailureIsAtomic(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015376", false))
	// Fail after the first table has been created, before RLS provisioning.
	_, err := db.ExecContext(t.Context(), `CREATE TABLE active.activity_session_attendance (probe BOOLEAN)`)
	require.NoError(t, err)
	require.Error(t, runPresenceMigration(t.Context(), db, "001015376", true))
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('active.activity_sessions') IS NULL`).Scan(t.Context(), &absent))
	require.True(t, absent, "a failed Expand must leave no partially provisioned session table")
}
