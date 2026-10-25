package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassHandoverRollsBack pins 1.15.502 (#4794): the tables exist only
// after the migration, with forced RLS, one handover per class and day and
// tenant-bound keys; the write scope turns into a list that keeps a stored
// value and back; the lehrkraft role holds the write permission; and the
// erasure plan knows both tables.
func TestClassHandoverRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Übergabe", "Kind", "2b")
	foreign := testpkg.CreateTestStudentForTenant(t, db, other, "Fremdes", "Kind", "2b")

	tablesExist := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('active.class_handovers') IS NOT NULL
			AND to_regclass('active.class_handover_children') IS NOT NULL`).Scan(ctx, &exists))
		return exists
	}
	lehrkraftHolds := func() bool {
		var holds bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE r.name = 'lehrkraft' AND p.name = 'class_day:handover_write')`).Scan(ctx, &holds))
		return holds
	}
	inErasurePlan := func() int {
		found := 0
		for _, entry := range readErasurePlan(t, db) {
			if entry.Name == "active.class_handovers" || entry.Name == "active.class_handover_children" {
				found++
			}
		}
		return found
	}
	setScope := func(tenantID int64, value string) {
		_, err := db.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value)
			VALUES (?, 'operations.school_portal_write_scope', ?::jsonb)
			ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`, tenantID, value)
		require.NoError(t, err)
	}
	scope := func(tenantID int64) string {
		var value string
		require.NoError(t, db.NewRaw(`SELECT value::text FROM config.setting_values
			WHERE tenant_id = ? AND setting_key = 'operations.school_portal_write_scope'`, tenantID).Scan(ctx, &value))
		return value
	}

	require.NoError(t, classHandoverDown(ctx, db))
	assert.False(t, tablesExist())
	assert.False(t, lehrkraftHolds())

	// Single values as stored before this migration.
	setScope(school, `"class_arrival_exceptions"`)
	setScope(other, `"none"`)
	require.NoError(t, classHandoverUp(ctx, db))
	assert.True(t, tablesExist())
	assert.True(t, lehrkraftHolds())
	assert.Equal(t, 2, inErasurePlan())
	assert.JSONEq(t, `["class_arrival_exceptions"]`, scope(school), "a stored write stays ticked")
	assert.JSONEq(t, `[]`, scope(other), `"none" ticks nothing`)

	for _, table := range []string{"active.class_handovers", "active.class_handover_children"} {
		var enabled, forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class
			WHERE oid = ?::regclass`, table).Scan(ctx, &enabled, &forced))
		assert.True(t, enabled, "row-level security is enabled on %s", table)
		assert.True(t, forced, "row-level security is forced on %s", table)
	}

	handOver := func(class string) (int64, error) {
		var id int64
		err := db.NewRaw(`INSERT INTO active.class_handovers (tenant_id, school_class, date)
			VALUES (?, ?, '2026-10-21') RETURNING id`, school, class).Scan(ctx, &id)
		return id, err
	}
	handover, err := handOver("2b")
	require.NoError(t, err)
	_, err = handOver(" 2B ")
	assert.ErrorContains(t, err, "uniq_class_handovers_class_date", "one handover per class and day, matched trimmed and lowercased")
	_, err = handOver("  ")
	assert.ErrorContains(t, err, "chk_class_handovers_school_class")

	child := func(tenantID, studentID int64) error {
		_, err := db.ExecContext(ctx, `INSERT INTO active.class_handover_children (tenant_id, handover_id, student_id, absent)
			VALUES (?, ?, ?, true)`, tenantID, handover, studentID)
		return err
	}
	require.NoError(t, child(school, student.ID))
	assert.ErrorContains(t, child(school, student.ID), "uniq_class_handover_children_student", "a child is handed over once per handover")
	assert.ErrorContains(t, child(school, foreign.ID), "fk_class_handover_children_student", "no child of another school")
	assert.ErrorContains(t, child(other, foreign.ID), "fk_class_handover_children_handover", "no handover of another school")
	_, err = db.ExecContext(ctx, `INSERT INTO active.class_handover_children (tenant_id, handover_id, student_id)
		VALUES (?, ?, ?)`, school, handover, student.ID)
	assert.ErrorContains(t, err, "absent", "every child is marked da or nicht da")

	_, err = db.ExecContext(ctx, `DELETE FROM active.class_handovers WHERE id = ?`, handover)
	require.NoError(t, err)
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.class_handover_children WHERE handover_id = ?`, handover).Scan(ctx, &left))
	assert.Zero(t, left, "a withdrawn handover takes its absences along")

	setScope(other, `["class_handover"]`)
	require.NoError(t, classHandoverDown(ctx, db))
	assert.JSONEq(t, `"class_arrival_exceptions"`, scope(school))
	assert.JSONEq(t, `"none"`, scope(other), "the handover alone has no single-value form")
	require.NoError(t, classHandoverUp(ctx, db))
}

// TestSchoolErasureDeletesClassHandovers proves the erasure phases cover both
// tables after 1.15.502 extended the plan with them.
func TestSchoolErasureDeletesClassHandovers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Übergabe", "Kind", "2b")
	var handover int64
	require.NoError(t, db.NewRaw(`INSERT INTO active.class_handovers (tenant_id, school_class, date)
		VALUES (?, '2b', '2026-10-21') RETURNING id`, school).Scan(t.Context(), &handover))
	erasureExec(t, db, `INSERT INTO active.class_handover_children (tenant_id, handover_id, student_id, absent) VALUES (?, ?, ?, false)`,
		school, handover, student.ID)

	assert.Equal(t, "db_done", eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school)).Status)
	var left int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM active.class_handovers WHERE tenant_id = ?)
		+ (SELECT count(*) FROM active.class_handover_children WHERE tenant_id = ?)`, school, school).Scan(t.Context(), &left))
	assert.Zero(t, left)
}
