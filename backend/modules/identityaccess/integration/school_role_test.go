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
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, TenantRuntime: testpkg.TenantRuntime(t, db), Observe: func(identityCompose.Observation) {}})
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
	for _, role := range roles {
		require.Equal(t, strings.ToLower(role.Name), role.NameKey, "the list carries the key FindSchoolRoleByName compares")
	}
	permissions, err := access.FindRolePermissionsByRoleIDs(testpkg.Ctx(t), []int64{local.ID, foreign.ID, system.ID})
	require.NoError(t, err)
	require.Equal(t, []string{permission.Name}, permissions[local.ID])
	require.NotContains(t, permissions, foreign.ID, "another school's role grants nothing here")
	require.NotContains(t, permissions, system.ID, "a role without permissions has no entry")
	permissions, err = access.FindRolePermissionsByRoleIDs(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	require.Empty(t, permissions)
	_, err = access.FindSchoolRoleByName(testpkg.Ctx(t), foreign.Name)
	require.ErrorIs(t, err, identityaccess.ErrRoleNotFound)
	_, err = access.ListSchoolRoles(context.Background())
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
}

func TestSchoolRoleNameCaseCollisionChoosesSmallestID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, TenantRuntime: testpkg.TenantRuntime(t, db), Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)

	for _, global := range []bool{false, true} {
		name := "tenant"
		if global {
			name = "global"
		}
		t.Run(name, func(t *testing.T) {
			create := testpkg.CreateTestRole
			if global {
				create = testpkg.CreateTestSystemRole
			}
			first := create(t, db, "admin")
			second := create(t, db, "other")
			require.Less(t, first.ID, second.ID)
			// The unique indexes compare name, not LOWER(name): these two
			// case variants are valid rows of the same rank, not duplicates.
			_, err := db.NewRaw(`UPDATE auth.roles SET name = ? WHERE id = ?`, strings.ToUpper(first.Name), second.ID).Exec(t.Context())
			require.NoError(t, err)
			// Move the smaller ID's heap tuple behind the larger one. Without
			// an ID tiebreaker, a sequential scan can pick the larger ID.
			_, err = db.NewRaw(`UPDATE auth.roles SET description = description WHERE id = ?`, first.ID).Exec(t.Context())
			require.NoError(t, err)

			counter := testpkg.CaptureQueriesForContext(t, db)
			role, err := access.FindSchoolRoleByName(counter.Context(testpkg.Ctx(t)), first.Name)
			require.NoError(t, err)
			require.Len(t, counter.Selects("auth.roles"), 1)
			require.Zero(t, counter.WriteRows())
			t.Logf("role lookup: %d SELECT, %d written rows", len(counter.Selects("auth.roles")), counter.WriteRows())
			require.Equal(t, first.ID, role.ID, "case collisions of equal rank choose the smallest ID, not heap order")
		})
	}
}
