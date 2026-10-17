package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 1.15.458 drops the free-text „Betreuernotizen". Its contract is
// that the drop loses no text: whatever never reached the note card is carried
// over first, a hint the school deleted stays deleted, and a rollback hands the
// previous image the live hint back.
func TestDropSupervisorNotes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestoreStudentSupervisorNotesColumn(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	missed := testpkg.CreateTestStudent(t, db, "Ida", "NieUebernommen", "2a")
	removed := testpkg.CreateTestStudent(t, db, "Ben", "Geloescht", "2a")
	setNotes := func(studentID int64, text string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `
			UPDATE users.student_care_profiles AS care
			SET supervisor_notes = ?
			FROM users.student_school_memberships AS membership
			WHERE membership.id = care.membership_id
			  AND membership.tenant_id = care.tenant_id
			  AND membership.student_profile_id = ?`, text, studentID)
		require.NoError(t, err)
	}
	setNotes(missed.ID, " Braucht Erinnerung an die Brille. ")
	setNotes(removed.ID, "Alter Hinweis")

	// The removed child's hint was carried over earlier and then deleted.
	account := testpkg.CreateTestAccount(t, db, "betreuernotizen-loescher")
	_, err := db.ExecContext(ctx, `INSERT INTO users.student_notes
		(tenant_id, student_id, origin, kind, visibility, body, deleted_at, deleted_by_account_id)
		VALUES (?, ?, 'master_data', 'permanent', 'all_staff', 'Alter Hinweis', now(), ?)`,
		tenantID, removed.ID, account.ID)
	require.NoError(t, err)

	require.NoError(t, dropSupervisorNotesUp(ctx, db))

	var columnExists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name = 'student_care_profiles'
		  AND column_name = 'supervisor_notes')`).Scan(ctx, &columnExists))
	assert.False(t, columnExists, "the column is gone")

	var body string
	require.NoError(t, db.NewRaw(`SELECT body FROM users.student_notes
		WHERE tenant_id = ? AND student_id = ? AND origin = 'master_data' AND deleted_at IS NULL`,
		tenantID, missed.ID).Scan(ctx, &body))
	assert.Equal(t, "Braucht Erinnerung an die Brille.", body,
		"a text that never reached the note card is carried over before the drop")

	var live int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_notes
		WHERE tenant_id = ? AND student_id = ? AND deleted_at IS NULL`,
		tenantID, removed.ID).Scan(ctx, &live))
	assert.Zero(t, live, "a deliberately deleted hint must not come back with the drop")

	require.NoError(t, dropSupervisorNotesUp(ctx, db), "the drop is idempotent")

	require.NoError(t, dropSupervisorNotesDown(ctx, db))
	var restored struct {
		Missed  *string `bun:"missed"`
		Removed *string `bun:"removed"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT
			(SELECT care.supervisor_notes FROM users.student_care_profiles AS care
			 JOIN users.student_school_memberships AS m ON m.id = care.membership_id
			 WHERE m.student_profile_id = ?) AS missed,
			(SELECT care.supervisor_notes FROM users.student_care_profiles AS care
			 JOIN users.student_school_memberships AS m ON m.id = care.membership_id
			 WHERE m.student_profile_id = ?) AS removed`,
		missed.ID, removed.ID).Scan(ctx, &restored))
	require.NotNil(t, restored.Missed)
	assert.Equal(t, "Braucht Erinnerung an die Brille.", *restored.Missed,
		"a rollback hands the previous image the live hint")
	assert.Nil(t, restored.Removed, "a deleted hint is not restored into the column")
}
