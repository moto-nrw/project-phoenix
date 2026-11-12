//go:build testhooks

package account_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// ReplaceAccountRole promises one staff role, preserving guardian access.
// This is a bounded positive contract, deliberately not synthetic N3/N8 growth.
func TestIdentityAccountRolesBoundedQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	host := chi.NewRouter()
	host.Use(api.RequestSettingsCacheMiddleware, api.RequestIdentityCacheMiddleware)
	host.Mount("/", f.router)
	f.router = host
	token, _ := f.accountToken(t, "bounded-role-reader", "users:manage")
	account := testpkg.CreateTestAccount(t, f.db, "bounded-role-owner")
	testpkg.EnsureAccountTenant(t, f.db, account.ID, testpkg.Tenant(t))
	staff := testpkg.CreateTestRole(t, f.db, "bounded-staff")
	replacement := testpkg.CreateTestRole(t, f.db, "bounded-replacement")
	guardian := testpkg.CreateTestRole(t, f.db, "bounded-guardian")
	_, err := f.db.ExecContext(context.Background(), "UPDATE auth.roles SET base_role = 'guardian' WHERE id = ?", guardian.ID)
	require.NoError(t, err)
	require.NoError(t, f.module.AccountAuthentication.ReplaceAccountRole(testpkg.Ctx(t), account.ID, staff.ID))
	want := []map[string]any{}
	addRole := func(id int64) {
		role, err := f.module.AccountAuthentication.GetRole(testpkg.Ctx(t), id)
		require.NoError(t, err)
		want = append(want, rbacBudgetRoleDTO(role))
	}
	addRole(staff.ID)
	path := fmt.Sprintf("/auth/accounts/%d/roles/", account.ID)
	rr, queries := f.get(t, path, token)
	rbacBudgetCollection(t, rr, "Account roles retrieved successfully", want)
	querybudgets.AssertQueryBudget(t, "api.auth.accounts.roles.bounded_positive", queries)
	// Guardian assignment is a relationship fixture, not a staff-role assignment.
	_, err = f.db.ExecContext(context.Background(), "INSERT INTO auth.account_roles (account_id,role_id,tenant_id) VALUES (?,?,?)", account.ID, guardian.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.NoError(t, f.module.AccountAuthentication.ReplaceAccountRole(testpkg.Ctx(t), account.ID, replacement.ID))
	roles, err := f.module.AccountAuthentication.GetAccountRoles(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 2)
	var staffCount, guardianCount int
	for _, role := range roles {
		if identityaccess.IsGuardianTierRole(&identityaccess.RoleFacts{BaseRole: role.BaseRole, Name: role.Name, IsSystem: role.IsSystem, TenantID: role.TenantID}) {
			guardianCount++
		} else {
			staffCount++
		}
	}
	require.Equal(t, 1, staffCount)
	require.Equal(t, 1, guardianCount)
	want = nil
	addRole(replacement.ID)
	addRole(guardian.ID)
	rr, queries = f.get(t, path, token)
	rbacBudgetCollection(t, rr, "Account roles retrieved successfully", want)
	querybudgets.AssertQueryBudget(t, "api.auth.accounts.roles.bounded_positive", queries)
}
