package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolAbsenceReportGrant pins who 1.15.683 lets enter a report
// afterwards (#4791): the system role schulverwaltung alone, not the
// Lehrkraft, not the Betreuer role and not a school's own role of the same
// name, also on a rerun.
func TestSchoolAbsenceReportGrant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	holds := func(t *testing.T, roleID int64) bool {
		t.Helper()
		var granted bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.role_permissions rp
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = ? AND p.name = 'class_day:absence_report_write')`, roleID).Scan(ctx, &granted))
		return granted
	}
	systemRole := func(t *testing.T, name string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE name = ? AND tenant_id IS NULL`, name).Scan(ctx, &id))
		return id
	}

	assert.True(t, holds(t, systemRole(t, "schulverwaltung")))
	assert.False(t, holds(t, systemRole(t, "lehrkraft")), "the Lehrkraft marks the morning, the office reports")
	assert.False(t, holds(t, systemRole(t, "user")))

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	own := testpkg.CreateTestRoleForTenant(t, db, "schulverwaltung", tenantID)
	require.NoError(t, schoolAbsenceReportsUp(ctx, db), "the migration is idempotent")
	assert.False(t, holds(t, own.ID), "a school's own role of the same name gets nothing")
	assert.True(t, holds(t, systemRole(t, "schulverwaltung")))
}

// TestSchoolAbsenceReportSource pins the status day source 1.15.683 allows:
// the school's, beside the existing ones, and nothing else.
func TestSchoolAbsenceReportSource(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Sam", "Schule", "4c")
	insert := func(day, source string) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
			VALUES (?, ?, ?::date, 'sick', NOW(), ?)`, testpkg.Tenant(t), student.ID, day, source)
		return err
	}
	require.NoError(t, insert("2099-03-02", "school"))
	assert.Error(t, insert("2099-03-03", "office"))
}
