package api

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A school created through operator provisioning starts with exactly one
// „Extern“ role (#4573): school-owned, base role user, not a system role,
// with schedules:read only, and the school can edit it like any own role.
func TestOperatorProvisioningIntegration_CreateSchool_ProvisionsExternRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	graph, identity := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID

	_, schoolID := provisionTestSchool(t, db, graph, operatorID, "extern")
	schoolCtx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolID)

	count, err := db.NewSelect().TableExpr("auth.roles").
		Where("tenant_id = ?", schoolID).
		Where("LOWER(TRIM(name)) = 'extern'").
		Count(schoolCtx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count, "the new school has exactly one Extern role")

	role, found, err := identity.FindExternalCaregiverRole(schoolCtx)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, role.TenantID)
	assert.Equal(t, schoolID, *role.TenantID)
	assert.False(t, role.IsSystem)
	require.NotNil(t, role.BaseRole)
	assert.Equal(t, identityaccess.BaseRoleUser, *role.BaseRole)

	permissions, err := identity.GetRolePermissions(schoolCtx, role.ID)
	require.NoError(t, err)
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.Name)
	}
	assert.Equal(t, []string{"schedules:read"}, names)

	// The leadership edits the role through the ordinary role administration.
	groupsRead, err := identity.GetPermissionByName(schoolCtx, "groups:read")
	require.NoError(t, err)
	require.NoError(t, identity.ReplaceRolePermissions(schoolCtx, role.ID, []int64{permissions[0].ID, groupsRead.ID}))
	role.Description = "Externe mit Gruppenansicht"
	require.NoError(t, identity.UpdateRole(schoolCtx, role))
	edited, err := identity.GetRolePermissions(schoolCtx, role.ID)
	require.NoError(t, err)
	assert.Len(t, edited, 2)
}
