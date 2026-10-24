package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMorningAttendanceRollsBack pins 1.15.492 (#4790): the table exists only
// after the migration, with forced RLS, the one-child and status checks and
// one row per child and date; the deletion log accepts the new kind; the
// lehrkraft role holds the write permission; and the rollback refuses while
// retention rows are logged.
func TestMorningAttendanceRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Morgen", "Kind", "2b")
	entry := testpkg.CreateTestClassListEntryForTenant(t, db, school, "Liste", "Kind", "2b")

	tableExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('active.morning_attendance') IS NOT NULL`).Scan(ctx, &exists))
		return exists
	}
	lehrkraftHolds := func() bool {
		var holds bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE r.name = 'lehrkraft' AND p.name = 'class_day:morning_attendance_write')`).Scan(ctx, &holds))
		return holds
	}
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, 'morning_attendance_retention', 1, 'system')`, school)
		return err
	}
	mark := func(studentID, entryID any, status string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO active.morning_attendance (tenant_id, date, student_id, class_list_entry_id, status)
			VALUES (?, '2026-10-21', ?, ?, ?)`, school, studentID, entryID, status)
		return err
	}

	require.NoError(t, morningAttendanceDown(ctx, db))
	assert.False(t, tableExists())
	assert.False(t, lehrkraftHolds())
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.492 the data kind needs a subject")

	require.NoError(t, morningAttendanceUp(ctx, db))
	assert.True(t, tableExists())
	assert.True(t, lehrkraftHolds())
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'enrollment_upload_retention', 1, 'system')`, school)
	require.NoError(t, err, "the subject check keeps the kinds of 1.15.491")

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class
		WHERE oid = 'active.morning_attendance'::regclass`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled, "row-level security is enabled")
	assert.True(t, forced, "row-level security is forced")

	require.NoError(t, mark(student.ID, nil, "present"))
	require.NoError(t, mark(nil, entry.ID, "absent"))
	assert.ErrorContains(t, mark(student.ID, nil, "absent"), "uniq_morning_attendance_student_date", "one row per child and date")
	assert.ErrorContains(t, mark(nil, entry.ID, "present"), "uniq_morning_attendance_list_entry_date", "one row per list entry and date")
	assert.ErrorContains(t, mark(student.ID, entry.ID, "present"), "chk_morning_attendance_subject", "a row names exactly one child")
	assert.ErrorContains(t, mark(nil, nil, "present"), "chk_morning_attendance_subject", "a row names a child")
	assert.ErrorContains(t, mark(student.ID, nil, "late"), "chk_morning_attendance_status", "only present and absent")

	require.NoError(t, logRow())
	assert.ErrorContains(t, morningAttendanceDown(ctx, db), "morning attendance retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}

// TestSchoolErasureDeletesMorningAttendance proves the reinstalled erasure
// phases cover the table.
func TestSchoolErasureDeletesMorningAttendance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Morgen", "Kind", "2b")
	entry := testpkg.CreateTestClassListEntryForTenant(t, db, school, "Liste", "Kind", "2b")
	erasureExec(t, db, `INSERT INTO active.morning_attendance (tenant_id, date, student_id, class_list_entry_id, status)
		VALUES (?, '2026-10-21', ?, NULL, 'present'), (?, '2026-10-21', NULL, ?, 'absent')`, school, student.ID, school, entry.ID)

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	assert.Equal(t, "db_done", result.Status)
	assert.EqualValues(t, 2, result.RowCounts["active.morning_attendance"],
		"the delete phase removes the rows itself, before the parents would cascade them")
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.morning_attendance WHERE tenant_id = ?`, school).Scan(t.Context(), &left))
	assert.Zero(t, left)
}
