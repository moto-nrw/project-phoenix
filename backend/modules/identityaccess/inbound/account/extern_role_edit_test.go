package account_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The „Extern“ role every school starts with (#4573) is the school's own
// role: the leadership edits its permissions and description through the
// ordinary role routes.
func TestExternRoleIsEditableThroughTheRoleRoutes(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	identity := setupAuthModuleOn(t, tc.db).Auth
	ctx := testpkg.Ctx(t)
	adminClaims := testutil.AdminTestClaims(1)

	var role identityaccess.Role
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, tc.db, testpkg.Tenant(t), func(txCtx context.Context) error {
		var err error
		role, err = identity.EnsureExternalCaregiverRole(txCtx, testpkg.Tenant(t))
		return err
	}))
	schedulesRead, err := identity.GetPermissionByName(ctx, "schedules:read")
	require.NoError(t, err)
	groupsRead, err := identity.GetPermissionByName(ctx, "groups:read")
	require.NoError(t, err)

	req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/roles/%d/permissions", role.ID), map[string][]string{
		"permission_ids": {fmt.Sprintf("%d", schedulesRead.ID), fmt.Sprintf("%d", groupsRead.ID)},
	})
	rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:manage"})
	require.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

	permissions, err := identity.GetRolePermissions(ctx, role.ID)
	require.NoError(t, err)
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.Name)
	}
	assert.ElementsMatch(t, []string{"schedules:read", "groups:read"}, names)

	req = testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/roles/%d", role.ID), map[string]string{
		"name":        role.Name,
		"description": "Externe mit Gruppenansicht",
	})
	rr = testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:update"})
	require.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

	found, ok, err := identity.FindExternalCaregiverRole(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, role.ID, found.ID)
	assert.Equal(t, "Externe mit Gruppenansicht", found.Description)
}
