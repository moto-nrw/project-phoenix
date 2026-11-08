//go:build testhooks

package account_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func TestRBACPermissionsListQueryBudget(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixture(t)
	resource := fmt.Sprintf("rbac-budget-%d", f.actor)
	foreignActor := testpkg.CreateTestAccount(t, f.db, "rbac-catalogue-other-school")
	foreignID := tenant.FromContext(f.foreign)
	testpkg.EnsureAccountTenant(t, f.db, foreignActor.ID, foreignID)
	testpkg.GrantTestPermission(t, f.db, foreignID, foreignActor.ID, "permissions:read")
	foreignToken, _, err := f.module.Auth.IssueTokensForAuthenticatedAccount(f.foreign, foreignActor.ID, foreignID, "", "")
	require.NoError(t, err)
	baseline, err := f.module.AccountAuthentication.ListPermissions(testpkg.Ctx(t), identityaccess.PermissionFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, baseline, "N0 is an empty resource filter, not an empty global catalogue")
	global := make([]map[string]any, 0, len(baseline)+9)
	for _, permission := range baseline {
		global = append(global, rbacBudgetPermissionDTO(permission.ID, permission.Name, permission.Description,
			permission.Resource, permission.Action, permission.CreatedAt, permission.UpdatedAt))
	}
	firstAction := fmt.Sprintf("read-%d-0", f.actor)
	decoyName := resource + "-other:" + firstAction
	decoy := f.permission(t, decoyName, "Other resource remains globally readable", resource+"-other", firstAction)
	decoyDTO := rbacBudgetPermissionDTO(decoy.ID, decoyName, "Other resource remains globally readable", resource+"-other", firstAction, decoy.CreatedAt, decoy.UpdatedAt)
	global = append(global, decoyDTO)
	want := []map[string]any{}
	var measurements []testpkg.QueryMeasurement
	for _, n := range []int{0, 3, 8} {
		for len(want) < n {
			action := fmt.Sprintf("read-%d-%d", f.actor, len(want))
			name, description := resource+":"+action, fmt.Sprintf("Independent catalogue permission %d", len(want))
			permission := f.permission(t, name, description, resource, action)
			dto := rbacBudgetPermissionDTO(permission.ID, name, description, resource, action, permission.CreatedAt, permission.UpdatedAt)
			want = append(want, dto)
			global = append(global, dto)
		}
		t.Logf("resource=%s persisted_permissions=%d", resource, n)
		rr, queries := f.get(t, "/auth/permissions/?resource="+url.QueryEscape(resource), f.token)
		rbacBudgetCollection(t, rr, "Permissions retrieved successfully", want)
		f.successFrame(t, "auth.permissions")
		measurements = append(measurements, queries)
	}
	for _, filter := range []struct {
		query string
		want  []map[string]any
	}{
		{"resource=" + strings.ToUpper(resource), want},
		{"resource=" + resource + "&action=" + strings.ToUpper(firstAction), []map[string]any{want[0]}},
		{"action=" + strings.ToUpper(firstAction), []map[string]any{want[0], decoyDTO}},
		{"resource=" + resource + "&action=" + firstAction[:len(firstAction)-1], []map[string]any{}},
		{"resource=" + resource[:len(resource)-1], []map[string]any{}},
		{"resource=" + resource + "-missing", []map[string]any{}},
		{"", global},
	} {
		rr, _ := f.get(t, "/auth/permissions/?"+filter.query, f.token)
		rbacBudgetCollection(t, rr, "Permissions retrieved successfully", filter.want)
	}
	// This catalogue is global, not owned by the school that created a role
	// using it. A properly permitted second school reads the exact same rows.
	rr, _ := f.get(t, "/auth/permissions/?resource="+resource, foreignToken)
	rbacBudgetCollection(t, rr, "Permissions retrieved successfully", want)
	for _, queries := range measurements {
		querybudgets.AssertQueryBudget(t, "api.auth.permissions.list", queries)
	}
}
