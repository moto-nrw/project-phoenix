package migrations

import (
	"context"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolDayHintConstraints pins what 1.15.542 lets users.student_notes
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

// TestSchoolDayHintGrant pins who 1.15.542 lets write a day hint: the system
// roles lehrkraft and schulverwaltung, the second because it holds every
// Lehrkraft permission (#4789) and was created before this migration; a
// school's own role of the same name gets nothing, also on a rerun.
func TestSchoolDayHintGrant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	holds := func(t *testing.T, roleID int64) bool {
		t.Helper()
		var granted bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.role_permissions rp
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = ? AND p.name = 'class_day:day_hint_write')`, roleID).Scan(ctx, &granted))
		return granted
	}
	systemRole := func(t *testing.T, name string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE name = ? AND tenant_id IS NULL`, name).Scan(ctx, &id))
		return id
	}

	assert.True(t, holds(t, systemRole(t, "lehrkraft")))
	assert.True(t, holds(t, systemRole(t, "schulverwaltung")))

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	own := testpkg.CreateTestRoleForTenant(t, db, "schulverwaltung", tenantID)
	require.NoError(t, schoolDayHintsUp(ctx, db), "the migration is idempotent")
	assert.False(t, holds(t, own.ID), "a school's own role of the same name gets nothing")
	assert.True(t, holds(t, systemRole(t, "schulverwaltung")))
}
