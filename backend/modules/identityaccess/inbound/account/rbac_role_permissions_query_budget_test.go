//go:build testhooks

package account_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// TestRBACRolePermissionsListQueryBudget pins the role-permission read: the
// caller's own custom role at 0/3/8 mappings in one tenant-framed statement,
// global system roles readable from every school, and a custom role of
// another school answered exactly like the role read, 404 (#5703).
func TestRBACRolePermissionsListQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixture(t)
	foreignID := tenant.FromContext(f.foreign)
	foreignActor := testpkg.CreateTestAccount(t, f.db, "rbac-role-perms-other-school")
	testpkg.EnsureAccountTenant(t, f.db, foreignActor.ID, foreignID)
	testpkg.GrantTestPermission(t, f.db, foreignID, foreignActor.ID, "roles:manage")
	foreignToken, _, err := f.module.Auth.IssueTokensForAuthenticatedAccount(f.foreign, foreignActor.ID, foreignID, "", "")
	require.NoError(t, err)

	own := testpkg.CreateTestRole(t, f.db, "rbac-role-perms-own")
	f.roleContexts[own.ID] = testpkg.Ctx(t)
	foreign := testpkg.CreateTestRoleForTenant(t, f.db, "rbac-role-perms-foreign", foreignID)
	f.roleContexts[foreign.ID] = f.foreign
	foreignPermission := f.permission(t, fmt.Sprintf("rbac-role-perms-%d:foreign", f.actor), "Foreign selection", fmt.Sprintf("rbac-role-perms-%d", f.actor), "foreign")
	require.NoError(t, f.module.AccountAuthentication.AssignPermissionToRole(f.foreign, foreign.ID, foreignPermission.ID))
	foreignWant := []map[string]any{rbacBudgetPermissionDTO(foreignPermission.ID, foreignPermission.Name, foreignPermission.Description,
		foreignPermission.Resource, foreignPermission.Action, foreignPermission.CreatedAt, foreignPermission.UpdatedAt)}

	path := func(id int64) string { return fmt.Sprintf("/auth/roles/%d/permissions/", id) }

	// The owning school still reads its selection; only the other school is refused.
	rr, _ := f.get(t, path(foreign.ID), foreignToken)
	rbacBudgetCollection(t, rr, "Role permissions retrieved successfully", foreignWant)

	// A foreign custom role and a role that does not exist answer exactly like
	// GET /auth/roles/{id}/, so the permission read reveals neither the
	// selection nor that the role exists.
	missing := foreign.ID + 1_000_000
	for _, id := range []int64{foreign.ID, missing} {
		roleRead, _ := f.get(t, fmt.Sprintf("/auth/roles/%d/", id), f.token)
		rbacBudgetProblem(t, roleRead, http.StatusNotFound, "general.input", "role not found", "anleitung-eingabe-pruefen")
		rr, queries := f.get(t, path(id), f.token)
		rbacBudgetProblem(t, rr, http.StatusNotFound, "general.input", "role not found", "anleitung-eingabe-pruefen")
		require.Equal(t, roleRead.Body.String(), rr.Body.String())
		require.NotContains(t, rr.Body.String(), foreignPermission.Name)
		t.Logf("invisible role %d statements=%d", id, len(queries.Statements))
	}

	want := []map[string]any{}
	var measurements []testpkg.QueryMeasurement
	for _, n := range []int{0, 3, 8} {
		for len(want) < n {
			action := fmt.Sprintf("read-%d", len(want))
			permission := f.permission(t, fmt.Sprintf("rbac-role-perms-%d:%s", own.ID, action), fmt.Sprintf("Own selection %d", len(want)),
				fmt.Sprintf("rbac-role-perms-%d", own.ID), action)
			require.NoError(t, f.module.AccountAuthentication.AssignPermissionToRole(testpkg.Ctx(t), own.ID, permission.ID))
			want = append(want, rbacBudgetPermissionDTO(permission.ID, permission.Name, permission.Description,
				permission.Resource, permission.Action, permission.CreatedAt, permission.UpdatedAt))
		}
		rr, queries := f.get(t, path(own.ID), f.token)
		rbacBudgetCollection(t, rr, "Role permissions retrieved successfully", want)
		f.successFrame(t, "auth.roles")
		t.Logf("own custom role N%d statements=%d", n, len(queries.Statements))
		measurements = append(measurements, queries)
	}
	// The form without trailing slash is the same route with the same
	// permission, not a separate alias (#5549).
	unslashed := path(own.ID)
	rr, queries := f.get(t, unslashed[:len(unslashed)-1], f.token)
	rbacBudgetCollection(t, rr, "Role permissions retrieved successfully", want)
	f.successFrame(t, "auth.roles")
	measurements = append(measurements, queries)

	// Global system roles stay readable, with the same selection for every school.
	roles, err := f.module.AccountAuthentication.ListRoles(testpkg.Ctx(t), identityaccess.RoleFilter{Name: "admin"})
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.True(t, roles[0].IsSystem)
	systemRead, _ := f.get(t, path(roles[0].ID), f.token)
	require.Equal(t, http.StatusOK, systemRead.Code, systemRead.Body.String())
	require.Contains(t, systemRead.Body.String(), `"data":[{"id":`, "the migrated admin role holds permissions")
	foreignSystemRead, _ := f.get(t, path(roles[0].ID), foreignToken)
	require.JSONEq(t, systemRead.Body.String(), foreignSystemRead.Body.String())

	for _, queries := range measurements {
		querybudgets.AssertQueryBudget(t, "api.auth.roles.permissions.list", queries)
	}
}
