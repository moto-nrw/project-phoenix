package migrations

import (
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 1.15.437 carries the child note card. Its CHECK constraints are the
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

	t.Run("a plain journal entry is accepted", func(t *testing.T) {
		require.NoError(t, insertStudentNote(t, db, tenantID, student.ID, ""))
	})

	t.Run("an empty body is rejected", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO users.student_notes (tenant_id, student_id, body) VALUES (?, ?, ?)`,
			tenantID, student.ID, "   ")
		require.ErrorContains(t, err, "chk_student_notes_body")
	})

	t.Run("two group references are rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID,
			", activity_group_id, education_group_id", activity.ID, group.ID)
		require.ErrorContains(t, err, "chk_student_notes_single_reference")
	})

	t.Run("group_leads without a group reference is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID, ", visibility", "group_leads")
		require.ErrorContains(t, err, "chk_student_notes_group_leads_reference")
	})

	t.Run("group_leads with a group reference is accepted", func(t *testing.T) {
		require.NoError(t, insertStudentNote(t, db, tenantID, student.ID,
			", visibility, education_group_id", "group_leads", group.ID))
	})

	t.Run("a durable hint with a day is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID,
			", kind, subject_date", "permanent", "2026-09-09")
		require.ErrorContains(t, err, "chk_student_notes_permanent_undated")
	})

	t.Run("an unknown visibility is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID, ", visibility", "everyone")
		require.ErrorContains(t, err, "chk_student_notes_visibility")
	})

	t.Run("a half-recorded deletion is rejected", func(t *testing.T) {
		err := insertStudentNote(t, db, tenantID, student.ID, ", deleted_at", "2026-09-09 10:00:00+02")
		require.ErrorContains(t, err, "chk_student_notes_deletion")
	})

	t.Run("a note for another tenant's child is rejected", func(t *testing.T) {
		otherTenantID, _ := testpkg.CreateTestTenant(t, db)
		foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Fremd", "Kind", "4b")
		err := insertStudentNote(t, db, tenantID, foreign.ID, "")
		require.ErrorContains(t, err, "fk_student_notes_student")
	})
}

// TestStudentNotesBackfill runs the migration's own backfill statement against
// real rows: the Betreuernotiz of a child becomes one durable hint without an
// author, a child without one gets nothing, and running it twice duplicates
// nothing — the property the migration relies on when it is replayed after a
// partial failure.
func TestStudentNotesBackfill(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	withNotes := testpkg.CreateTestStudent(t, db, "Jonas", "MitNotiz", "3a")
	without := testpkg.CreateTestStudent(t, db, "Lea", "OhneNotiz", "3a")

	_, err := db.ExecContext(ctx, `
		UPDATE users.student_care_profiles AS care
		SET supervisor_notes = ?
		FROM users.student_school_memberships AS membership
		WHERE membership.id = care.membership_id
		  AND membership.tenant_id = care.tenant_id
		  AND membership.student_profile_id = ?`,
		"  Braucht morgens etwas Zeit.  ", withNotes.ID)
	require.NoError(t, err)

	require.NoError(t, studentNotesBackfill(ctx, db))

	type carried struct {
		Body       string `bun:"body"`
		Kind       string `bun:"kind"`
		Visibility string `bun:"visibility"`
		Origin     string `bun:"origin"`
		HasAuthor  bool   `bun:"has_author"`
	}
	var rows []carried
	require.NoError(t, db.NewRaw(`
		SELECT body, kind, visibility, origin, author_account_id IS NOT NULL AS has_author
		FROM users.student_notes WHERE tenant_id = ? AND student_id = ?`,
		tenantID, withNotes.ID).Scan(ctx, &rows))

	require.Len(t, rows, 1, "one Betreuernotiz becomes exactly one durable hint")
	assert.Equal(t, "Braucht morgens etwas Zeit.", rows[0].Body, "the text is trimmed, not reformatted")
	assert.Equal(t, "permanent", rows[0].Kind)
	assert.Equal(t, "all_staff", rows[0].Visibility)
	assert.Equal(t, "master_data", rows[0].Origin)
	assert.False(t, rows[0].HasAuthor, "a carried-over hint must not name an author")

	assert.Zero(t, countStudentNotes(t, db, tenantID, without.ID),
		"a child without a Betreuernotiz gets no note")

	// Replay: the predicate, not a unique constraint, is what keeps this safe.
	require.NoError(t, studentNotesBackfill(ctx, db))
	assert.Equal(t, 1, countStudentNotes(t, db, tenantID, withNotes.ID),
		"a replayed backfill must not duplicate the hint")

	// A hint the school removed stays removed: the predicate counts
	// soft-deleted rows as present, so a replay does not resurrect it.
	account := testpkg.CreateTestAccount(t, db, "notizen-loescher")
	_, err = db.ExecContext(ctx, `
		UPDATE users.student_notes SET deleted_at = now(), deleted_by_account_id = ?
		WHERE tenant_id = ? AND student_id = ?`, account.ID, tenantID, withNotes.ID)
	require.NoError(t, err)

	require.NoError(t, studentNotesBackfill(ctx, db))
	var live int
	require.NoError(t, db.NewRaw(`
		SELECT count(*) FROM users.student_notes
		WHERE tenant_id = ? AND student_id = ? AND deleted_at IS NULL`,
		tenantID, withNotes.ID).Scan(ctx, &live))
	assert.Zero(t, live, "a deliberately removed hint must not come back")
}

func countStudentNotes(t *testing.T, db *testpkg.DB, tenantID, studentID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(
		`SELECT count(*) FROM users.student_notes WHERE tenant_id = ? AND student_id = ?`,
		tenantID, studentID).Scan(testpkg.Ctx(t), &count))
	return count
}
