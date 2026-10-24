package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassHomeworkRollsBack pins 1.15.510 (#4796): both tables exist only
// after the migration, with forced RLS and their checks; the deletion log
// accepts the new kind; the system role lehrkraft and a school role made from
// „Wie Lehrkraft“ hold the write permission, a tenant role merely named
// lehrkraft does not; and the rollback refuses while retention rows are
// logged.
func TestClassHomeworkRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Haus", "Aufgabe", "3a")
	suffix := time.Now().UnixNano()
	var portalRole, namesakeRole int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (name, description, tenant_id, school_portal)
		VALUES (?, 'aus Wie Lehrkraft', ?, TRUE) RETURNING id`, fmt.Sprintf("Schulrolle %d", suffix), school).Scan(ctx, &portalRole))
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (name, description, tenant_id, school_portal)
		VALUES ('lehrkraft', 'nur gleicher Name', ?, FALSE) RETURNING id`, school).Scan(ctx, &namesakeRole))

	tablesExist := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('education.class_homework') IS NOT NULL
			AND to_regclass('education.homework_feedback') IS NOT NULL`).Scan(ctx, &exists))
		return exists
	}
	holds := func(where string, args ...any) bool {
		var held bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE p.name = 'class_day:homework_write' AND `+where+`)`, args...).Scan(ctx, &held))
		return held
	}
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, 'homework_retention', 1, 'system')`, school)
		return err
	}
	entry := func(subject any, description string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO education.class_homework (tenant_id, school_class, date, subject, description)
			VALUES (?, '3a', '2026-10-21', ?, ?)`, school, subject, description)
		return err
	}
	feedback := func(status string, note any) error {
		_, err := db.ExecContext(ctx, `INSERT INTO education.homework_feedback (tenant_id, student_id, date, status, note)
			VALUES (?, ?, '2026-10-21', ?, ?)`, school, student.ID, status, note)
		return err
	}

	require.NoError(t, classHomeworkDown(ctx, db))
	assert.False(t, tablesExist())
	assert.False(t, holds("r.tenant_id IS NULL AND r.name = 'lehrkraft'"))
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.510 the data kind needs a subject")

	require.NoError(t, classHomeworkUp(ctx, db))
	assert.True(t, tablesExist())
	assert.True(t, holds("r.tenant_id IS NULL AND r.name = 'lehrkraft'"), "the system role lehrkraft holds the permission")
	assert.True(t, holds("r.id = ?", portalRole), "a school role made from Wie Lehrkraft holds the permission")
	assert.False(t, holds("r.id = ?", namesakeRole), "a tenant role that only shares the name gets nothing")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'morning_attendance_retention', 1, 'system')`, school)
	require.NoError(t, err, "the subject check keeps the kinds of 1.15.492")

	for _, table := range []string{"education.class_homework", "education.homework_feedback"} {
		var enabled, forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class
			WHERE oid = ?::regclass`, table).Scan(ctx, &enabled, &forced))
		assert.True(t, enabled, "row-level security is enabled on %s", table)
		assert.True(t, forced, "row-level security is forced on %s", table)
	}

	require.NoError(t, entry("Mathe", "Seite 12, Nummer 3"))
	require.NoError(t, entry(nil, "Lesen"), "the subject is optional")
	assert.ErrorContains(t, entry(nil, "  "), "chk_class_homework_description", "the text is required")
	assert.ErrorContains(t, entry("  ", "Lesen"), "chk_class_homework_subject", "an empty subject is stored as NULL, not blank")

	require.NoError(t, feedback("done", nil))
	assert.ErrorContains(t, feedback("difficult", "zu schwer"), "uniq_homework_feedback_student_date", "one feedback per child and day")
	assert.ErrorContains(t, feedback("late", nil), "chk_homework_feedback_status", "only done, not_done and difficult")

	require.NoError(t, logRow())
	assert.ErrorContains(t, classHomeworkDown(ctx, db), "homework retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}

// TestClassHomeworkIsTenantIsolated proves the RLS policy: a session bound to
// one school neither reads nor writes the homework of another.
func TestClassHomeworkIsTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	erasureExec(t, db, `INSERT INTO education.class_homework (tenant_id, school_class, date, description)
		VALUES (?, '3a', '2026-10-21', 'eigene'), (?, '3a', '2026-10-21', 'fremde')`, school, other)

	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_tenant`)
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school))
	require.NoError(t, err)

	var texts []string
	require.NoError(t, tx.NewRaw(`SELECT description FROM education.class_homework ORDER BY id`).Scan(t.Context(), &texts))
	assert.Equal(t, []string{"eigene"}, texts, "only the own school's rows are visible")
	_, err = tx.ExecContext(t.Context(), `INSERT INTO education.class_homework (tenant_id, school_class, date, description)
		VALUES (?, '3a', '2026-10-22', 'eingeschmuggelt')`, other)
	assert.ErrorContains(t, err, "row-level security", "a row for another school is refused")
}

// TestSchoolErasureDeletesHomework proves the reinstalled erasure phases
// cover both tables.
func TestSchoolErasureDeletesHomework(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Haus", "Aufgabe", "3a")
	erasureExec(t, db, `INSERT INTO education.class_homework (tenant_id, school_class, date, description)
		VALUES (?, '3a', '2026-10-21', 'Lesen')`, school)
	erasureExec(t, db, `INSERT INTO education.homework_feedback (tenant_id, student_id, date, status)
		VALUES (?, ?, '2026-10-21', 'done')`, school, student.ID)

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	assert.Equal(t, "db_done", result.Status)
	assert.EqualValues(t, 1, result.RowCounts["education.class_homework"])
	assert.EqualValues(t, 1, result.RowCounts["education.homework_feedback"],
		"the delete phase removes the rows itself, before the child would cascade them")
	var left int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM education.class_homework WHERE tenant_id = ?)
		+ (SELECT count(*) FROM education.homework_feedback WHERE tenant_id = ?)`, school, school).Scan(t.Context(), &left))
	assert.Zero(t, left)
}
