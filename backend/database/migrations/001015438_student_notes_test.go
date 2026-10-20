package migrations

import (
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 1.15.438 carries the child note card. Its CHECK constraints are the
// model: they are what stops a note from being about two groups at once, from
// being a durable hint with a day, or from reaching a leadership that no group
// reference names. Go-side validation that drifts from them would let the
// database reject a write the application believed was fine, so the constraints
// are exercised against the real table rather than trusted.
//
// The table is never dropped and re-created here: a schema that disappears
// mid-run would fail every parallel test around it.

// insertStudentNote writes one note with the given extra columns and returns
// the database's verdict, so a test can assert on the constraint by name.
func insertStudentNote(
	t *testing.T, db *testpkg.DB, tenantID, studentID int64, columns string, values ...any,
) error {
	t.Helper()
	placeholders := ""
	for range values {
		placeholders += ", ?"
	}
	query := fmt.Sprintf(
		`INSERT INTO users.student_notes (tenant_id, student_id, body%s) VALUES (?, ?, ?%s)`,
		columns, placeholders,
	)
	args := append([]any{tenantID, studentID, "Eine Notiz."}, values...)
	_, err := db.ExecContext(testpkg.Ctx(t), query, args...)
	return err
}

func TestStudentNotesConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	student := testpkg.CreateTestStudent(t, db, "Mila", "Notiz", "3a")
	group := testpkg.CreateTestEducationGroupForTenant(t, db, tenantID, "Notiz-Gruppe")
	activity := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Notiz-Angebot")

	t.Run("a dated journal entry is accepted", func(t *testing.T) {
		require.NoError(t, insertStudentNote(t, db, tenantID, student.ID,
			", subject_date", "2026-09-09"))
	})

	t.Run("an undated journal entry is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID, "")
		require.ErrorContains(t, err, "chk_student_notes_permanent_undated")
	})

	t.Run("an empty body is rejected", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO users.student_notes (tenant_id, student_id, body, subject_date) VALUES (?, ?, ?, ?)`,
			tenantID, student.ID, "   ", "2026-09-09")
		require.ErrorContains(t, err, "chk_student_notes_body")
	})

	t.Run("two group references are rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID,
			", subject_date, activity_group_id, education_group_id", "2026-09-09", activity.ID, group.ID)
		require.ErrorContains(t, err, "chk_student_notes_single_reference")
	})

	t.Run("group_leads with a group reference is accepted", func(t *testing.T) {
		require.NoError(t, insertStudentNote(t, db, tenantID, student.ID,
			", visibility, subject_date, education_group_id", "group_leads", "2026-09-09", group.ID))
	})

	t.Run("deleting a referenced group retains its leadership note without the reference", func(t *testing.T) {
		var noteID int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO users.student_notes (tenant_id, student_id, visibility, body, subject_date, activity_group_id)
			VALUES (?, ?, 'group_leads', 'Kursnotiz.', '2026-09-09', ?) RETURNING id`,
			tenantID, student.ID, activity.ID).Scan(ctx, &noteID))
		_, err := db.NewRaw(`DELETE FROM activities.groups WHERE id = ?`, activity.ID).Exec(ctx)
		require.NoError(t, err)
		var activityGroupID *int64
		require.NoError(t, db.NewRaw(`SELECT activity_group_id FROM users.student_notes WHERE id = ?`, noteID).
			Scan(ctx, &activityGroupID))
		assert.Nil(t, activityGroupID)
	})

	t.Run("a durable hint with a day is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID,
			", kind, subject_date", "permanent", "2026-09-09")
		require.ErrorContains(t, err, "chk_student_notes_permanent_undated")
	})

	t.Run("an unknown visibility is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID,
			", visibility, subject_date", "everyone", "2026-09-09")
		require.ErrorContains(t, err, "chk_student_notes_visibility")
	})

	t.Run("a half-recorded deletion is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID,
			", subject_date, deleted_at", "2026-09-09", "2026-09-09 10:00:00+02")
		require.ErrorContains(t, err, "chk_student_notes_deletion")
	})

	t.Run("a carried-over hint may be retired without an invented author", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO users.student_notes (tenant_id, student_id, origin, kind, visibility, body, deleted_at)
			VALUES (?, ?, 'master_data', 'permanent', 'all_staff', 'Übernommener Hinweis.', now())`,
			tenantID, student.ID)
		require.NoError(t, err)
	})

	t.Run("a note for another tenant's child is rejected", func(t *testing.T) {
		otherTenantID, _ := testpkg.CreateTestTenant(t, db)
		foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Fremd", "Kind", "4b")
		err := insertStudentNote(t, db, tenantID, foreign.ID, ", subject_date", "2026-09-09")
		require.ErrorContains(t, err, "fk_student_notes_student")
	})
}
