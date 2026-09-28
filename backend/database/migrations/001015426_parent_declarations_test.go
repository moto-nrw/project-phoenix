package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentDeclarationsRollbackKeepsUntrackedPermissionGrant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	require.NoError(t, parentDeclarationsDown(ctx, db))

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Erklärung", "Kind", "1a")
	introduced := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID,
		testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Neue", "Berechtigung", "introduced").ID,
		"primary_guardian")
	manual := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID,
		testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Manuelle", "Berechtigung", "manual").ID,
		"emergency_contact")

	for _, grant := range []struct {
		relationshipID int64
		permissions    string
	}{
		{introduced.ID, `{"parent_portal.access": true}`},
		{manual.ID, `{"parent_portal.declarations.submit": true}`},
	} {
		_, err := db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = ?::jsonb
			WHERE tenant_id = ? AND relationship_id = ?`, grant.permissions, tenantID, grant.relationshipID)
		require.NoError(t, err)
	}

	require.NoError(t, parentDeclarationsUp(ctx, db))
	var tracked int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.parent_declaration_permission_grants
		WHERE tenant_id = ?`, tenantID).Scan(ctx, &tracked))
	assert.Equal(t, 1, tracked)

	require.NoError(t, parentDeclarationsDown(ctx, db))
	for _, grant := range []struct {
		relationshipID int64
		expected       bool
	}{
		{introduced.ID, false},
		{manual.ID, true},
	} {
		var hasPermission bool
		require.NoError(t, db.NewRaw(`SELECT jsonb_exists(permissions, 'parent_portal.declarations.submit')
			FROM auth.guardian_student_access WHERE tenant_id = ? AND relationship_id = ?`, tenantID, grant.relationshipID).
			Scan(ctx, &hasPermission))
		assert.Equal(t, grant.expected, hasPermission)
	}
}
