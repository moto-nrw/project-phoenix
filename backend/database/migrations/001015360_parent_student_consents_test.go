package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentConsentChangesAreTenantScopedAndAppendOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity
		FROM pg_class
		WHERE oid = 'audit.student_consent_changes'::regclass
	`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled)
	assert.True(t, forced)

	for _, privilege := range []string{"SELECT", "INSERT"} {
		assert.True(t, tenantHasPrivilege(t, db, "audit.student_consent_changes", privilege))
	}
	for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
		assert.False(t, tenantHasPrivilege(t, db, "audit.student_consent_changes", privilege))
	}

	var studentFKIndexed bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_indexes
			WHERE schemaname = 'audit'
			  AND tablename = 'student_consent_changes'
			  AND indexname = 'idx_student_consent_changes_student_fk'
		)
	`).Scan(ctx, &studentFKIndexed))
	assert.True(t, studentFKIndexed)

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Consent", "A", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Consent", "B", "1b")
	_, err := db.ExecContext(ctx, `
		INSERT INTO audit.student_consent_changes
			(tenant_id, student_id, consent_key, action, source)
		VALUES (?, ?, 'photo', 'granted', 'tenant_portal'),
		       (?, ?, 'photo', 'withdrawn', 'parent_portal')
	`, tenantA, studentA.ID, tenantB, studentB.ID)
	require.NoError(t, err)
	assertTenantRowsIsolated(t, db, "audit.student_consent_changes", tenantA, `
		INSERT INTO audit.student_consent_changes
			(tenant_id, student_id, consent_key, action, source)
		VALUES (?, ?, 'photo', 'withdrawn', 'parent_portal')
	`, tenantB, studentB.ID)
}

func TestParentStudentConsentsRollbackPreservesExistingPermission(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	require.NoError(t, parentStudentConsentsDown(ctx, db))
	defer func() { require.NoError(t, parentStudentConsentsUp(ctx, db)) }()

	setPermissions := func(studentID int64, permissions string) {
		t.Helper()
		_, err := db.NewRaw(`
			UPDATE users.students_guardians
			SET permissions = ?::jsonb
			WHERE student_id = ?
		`, permissions, studentID).Exec(ctx)
		require.NoError(t, err)
	}
	permissionValue := func(studentID int64, key string) string {
		t.Helper()
		var value string
		require.NoError(t, db.NewRaw(`
			SELECT COALESCE(permissions ->> ?, '<missing>')
			FROM users.students_guardians
			WHERE student_id = ?
		`, key, studentID).Scan(ctx, &value))
		return value
	}

	preExisting := testpkg.CreateTestParentGuardianChain(t, db)
	backfilled := testpkg.CreateTestParentGuardianChain(t, db)
	setPermissions(preExisting.StudentID, `{
		"parent_portal.access": true,
		"parent_portal.consent.manage": true
	}`)
	setPermissions(backfilled.StudentID, `{
		"parent_portal.access": true
	}`)

	require.NoError(t, parentStudentConsentsUp(ctx, db))
	assert.Equal(t, "true", permissionValue(preExisting.StudentID, "parent_portal.consent.manage"))
	assert.Equal(t, "true", permissionValue(backfilled.StudentID, "parent_portal.consent.manage"))

	require.NoError(t, parentStudentConsentsDown(ctx, db))
	assert.Equal(t, "true", permissionValue(preExisting.StudentID, "parent_portal.consent.manage"))
	assert.Equal(t, "<missing>", permissionValue(backfilled.StudentID, "parent_portal.consent.manage"))
	assert.Equal(t, "true", permissionValue(preExisting.StudentID, "parent_portal.access"))
	assert.Equal(t, "true", permissionValue(backfilled.StudentID, "parent_portal.access"))
}
