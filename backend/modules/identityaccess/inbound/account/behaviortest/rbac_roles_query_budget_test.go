//go:build testhooks

package account_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

func TestRBACRolesListQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixture(t)
	reader, _ := f.accountToken(t, "rbac-budget-role-reader", "roles:read")
	creator, _ := f.accountToken(t, "rbac-budget-creator", "users:create")
	manager, _ := f.accountToken(t, "rbac-budget-manager", "users:manage")
	system := testpkg.CreateTestSystemRole(t, f.db, "rbac-visible-system")
	foreign := testpkg.CreateTestRoleForTenant(t, f.db, "rbac-foreign-role", tenant.FromContext(f.foreign))
	baseline, err := f.module.AccountAuthentication.ListRoles(testpkg.Ctx(t), identityaccess.RoleFilter{})
	require.NoError(t, err)
	want := make([]map[string]any, 0, len(baseline)+8)
	systemCount := 0
	systemNames := []string{}
	for _, role := range baseline {
		require.NotEqual(t, foreign.ID, role.ID)
		if role.IsSystem {
			systemCount++
			systemNames = append(systemNames, role.Name)
		}
		want = append(want, rbacBudgetRoleDTO(role))
	}
	require.Subset(t, systemNames, []string{"admin", "user", "guardian", "lehrkraft", "guest", "ag-leitung", "schulverwaltung", system.Name})
	require.Greater(t, systemCount, 1, "existing system catalogue must remain, alongside the independent system fixture")
	var measurements []testpkg.QueryMeasurement
	var custom []*testpkg.RoleFixture
	for _, n := range []int{0, 3, 8} {
		for len(custom) < n {
			label := fmt.Sprintf("rbac-budget-role-%02d", len(custom))
			role := testpkg.CreateTestRole(t, f.db, label)
			custom = append(custom, role)
			want = append(want, map[string]any{"id": fmt.Sprint(role.ID), "name": role.Name, "description": "Test role: " + label,
				"is_system": false, "base_role": "user", "school_portal": false,
				"created_at": role.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), "updated_at": role.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")})
		}
		t.Logf("additional_custom_roles=%d baseline_roles=%d system_roles=%d", n, len(baseline), systemCount)
		rr, queries := f.get(t, "/auth/roles/", f.token)
		rbacBudgetCollection(t, rr, "Roles retrieved successfully", want)
		f.successFrame(t, "auth.roles")
		measurements = append(measurements, queries)
	}
	for _, token := range []string{reader, creator, manager} {
		rr, _ := f.get(t, "/auth/roles/", token)
		rbacBudgetCollection(t, rr, "Roles retrieved successfully", want)
	}
	for _, filter := range []struct {
		name string
		want []map[string]any
	}{
		{strings.ToUpper(custom[0].Name), []map[string]any{want[len(baseline)]}},
		{custom[0].Name[:len(custom[0].Name)-1], []map[string]any{}},
		{foreign.Name, []map[string]any{}},
		{strings.ToUpper(system.Name), []map[string]any{rbacBudgetRoleDTO(identityaccess.Role{ID: system.ID, Name: system.Name,
			Description: "System role: rbac-visible-system", IsSystem: true, CreatedAt: system.CreatedAt, UpdatedAt: system.UpdatedAt})}},
	} {
		rr, _ := f.get(t, "/auth/roles/?name="+url.QueryEscape(filter.name), f.token)
		rbacBudgetCollection(t, rr, "Roles retrieved successfully", filter.want)
	}
	for _, queries := range measurements {
		querybudgets.AssertQueryBudget(t, "api.auth.roles.list", queries)
	}
}
