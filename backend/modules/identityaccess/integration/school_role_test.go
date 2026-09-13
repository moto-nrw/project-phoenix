package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSchoolRoleQueriesPreferTenantRoleAndHideForeignPermissions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	system := testpkg.CreateTestSystemRole(t, db, "import-role-system")
	local := testpkg.CreateTestRole(t, db, "import-role-local")
	_, err := db.NewRaw(`UPDATE auth.roles SET name = ? WHERE id = ?`, system.Name, local.ID).Exec(context.Background())
	require.NoError(t, err)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestRoleForTenant(t, db, "import-role-foreign", otherTenant)
	permission := testpkg.CreateTestPermission(t, db, "import-role-permission", "users", "read")
	_, err = db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?), (?, ?)`, local.ID, permission.ID, foreign.ID, permission.ID).Exec(context.Background())
	require.NoError(t, err)
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)

	role, err := access.FindSchoolRoleByName(testpkg.Ctx(t), strings.ToUpper(system.Name))
	require.NoError(t, err)
	require.Equal(t, local.ID, role.ID)
	require.Equal(t, local.BaseRole, role.BaseRole)
	roles, err := access.ListSchoolRoles(testpkg.Ctx(t))
	require.NoError(t, err)
	ids := make([]int64, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	require.Contains(t, ids, system.ID)
	require.Contains(t, ids, local.ID)
	require.NotContains(t, ids, foreign.ID)
	permissions, err := access.FindRolePermissions(testpkg.Ctx(t), local.ID)
	require.NoError(t, err)
	require.Equal(t, []string{permission.Name}, permissions)
	permissions, err = access.FindRolePermissions(testpkg.Ctx(t), foreign.ID)
	require.NoError(t, err)
	require.Empty(t, permissions)
	_, err = access.FindSchoolRoleByName(testpkg.Ctx(t), foreign.Name)
	require.ErrorIs(t, err, identityaccess.ErrRoleNotFound)
	_, err = access.ListSchoolRoles(context.Background())
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
}
