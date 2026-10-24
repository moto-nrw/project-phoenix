package migrations

import (
	"context"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolDayHintConstraints pins what 1.15.494 lets users.student_notes
// hold for a Lehrkraft's day hint (#4795): a dated, all-staff journal entry of
// at most 500 characters without a group reference, one live hint per author,
// child, day and target, and a class only on a school hint.
func TestSchoolDayHintConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	student := testpkg.CreateTestStudent(t, db, "Hanna", "Hinweis", "3b")
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Lena", "Lehrerin")

	insert := func(origin, kind, visibility, schoolClass, body string) error {
		var class any
		if schoolClass != "" {
			class = schoolClass
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO users.student_notes
				(tenant_id, student_id, author_account_id, origin, kind, visibility, body, subject_date, school_class)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_DATE, ?)`,
			student.TenantID, student.ID, account.ID, origin, kind, visibility, body, class)
		return err
	}

	require.NoError(t, insert("school", "journal", "all_staff", "", "Hat heute Bauchweh"))
	require.NoError(t, insert("school", "journal", "all_staff", "3b", "Heute Sportfest"),
		"a class hint and a child hint of the same author are two targets")

	assert.Error(t, insert("school", "journal", "all_staff", "", "Zweiter Hinweis"),
		"one live child hint per author, child and day")
	assert.Error(t, insert("school", "journal", "all_staff", " 3B ", "Zweiter Klassenhinweis"),
		"the class is compared trimmed and case-insensitive")
	assert.Error(t, insert("school", "journal", "care_team", "4a", "Nur für das Team"), "a school hint is for all staff")
	assert.Error(t, insert("school", "journal", "all_staff", "4c", strings.Repeat("ä", 501)), "at most 500 characters")
	assert.Error(t, insert("staff", "journal", "all_staff", "3b", "Klassennotiz"), "only a school hint carries a class")
	assert.Error(t, insert("school", "permanent", "all_staff", "4d", "Dauerhaft"), "a school hint is a dated entry")

	_, err := db.ExecContext(ctx, `
		UPDATE users.student_notes SET deleted_at = NOW(), deleted_by_account_id = ?
		WHERE student_id = ? AND origin = 'school' AND school_class IS NULL`, account.ID, student.ID)
	require.NoError(t, err)
	assert.NoError(t, insert("school", "journal", "all_staff", "", "Neuer Hinweis"),
		"a withdrawn hint frees its slot")
}
