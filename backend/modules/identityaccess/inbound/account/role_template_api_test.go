package account_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Role templates and the reserved permission rule over the HTTP routes
// (#4574).

func (e *accountIsolationEnv) createRole(t *testing.T, body map[string]string) authAPI.RoleResponse {
	t.Helper()
	rr := e.execute(t, e.claims, http.MethodPost, "/auth/roles", body)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var response struct {
		Data authAPI.RoleResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	return response.Data
}

func (e *accountIsolationEnv) permissionNamesOf(t *testing.T, roleID int64) []string {
	t.Helper()
	permissions, err := e.tc.resource.Sessions.GetRolePermissions(tenant.WithTenantID(t.Context(), e.tenantID), roleID)
	require.NoError(t, err)
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.FullName())
	}
	return names
}

// catalogPermissionNames lists the whole permission catalog, which the admin
// template copies minus the reserved permissions.
func (e *accountIsolationEnv) catalogPermissionNames(t *testing.T) []string {
	t.Helper()
	permissions, err := e.tc.resource.Sessions.ListPermissions(tenant.WithTenantID(t.Context(), e.tenantID), identityaccess.PermissionFilter{})
	require.NoError(t, err)
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.FullName())
	}
	return names
}

func (e *accountIsolationEnv) systemRole(t *testing.T, name string) identityaccess.Role {
	t.Helper()
	roles, err := e.tc.resource.Sessions.ListRoles(tenant.WithTenantID(t.Context(), e.tenantID), identityaccess.RoleFilter{Name: name})
	require.NoError(t, err)
	for _, role := range roles {
		if role.IsSystem {
			return role
		}
	}
	require.FailNow(t, "system role missing", name)
	return identityaccess.Role{}
}

func TestCreateRoleFromTemplateCopiesTheTemplatePermissions(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)

	for _, template := range identityaccess.RoleTemplates() {
		role := e.createRole(t, map[string]string{"name": fmt.Sprintf("Vorlage %s %d", template, e.tenantID), "template": string(template)})

		source := e.permissionNamesOf(t, e.systemRole(t, string(template)).ID)
		if template == identityaccess.RoleTemplateAdministration {
			source = e.catalogPermissionNames(t)
		}
		want := []string{}
		for _, name := range source {
			if !identityaccess.IsReservedPermission(name) {
				want = append(want, name)
			}
		}
		assert.NotEmpty(t, want, template)
		assert.ElementsMatch(t, want, e.permissionNamesOf(t, role.ID), template)
		require.NotNil(t, role.BaseRole, template)
		assert.False(t, role.IsSystem, template)
	}
}

func TestCreateRoleRejectsTemplateWithBaseRole(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)

	for _, body := range []map[string]string{
		{"name": "Beides", "template": "user", "base_role": "user"},
		{"name": "Gast", "template": "guest"},
	} {
		rr := e.execute(t, e.claims, http.MethodPost, "/auth/roles", body)
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestReplaceRolePermissionsRefusesReservedPermissions(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	ctx := tenant.WithTenantID(t.Context(), e.tenantID)
	role := testpkg.CreateTestRoleForTenant(t, e.tc.db, "platform-refusal", e.tenantID)
	groupsRead, err := e.tc.resource.Sessions.GetPermissionByName(ctx, "groups:read")
	require.NoError(t, err)

	for _, name := range reservedPermissionNames {
		platform, err := e.tc.resource.Sessions.GetPermissionByName(ctx, name)
		require.NoError(t, err, name)

		rr := e.execute(t, e.claims, http.MethodPut, fmt.Sprintf("/auth/roles/%d/permissions", role.ID), map[string][]string{
			"permission_ids": {fmt.Sprint(groupsRead.ID), fmt.Sprint(platform.ID)},
		})
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		var body struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "identity.role_platform_permission", body.Code, name)
		assert.Equal(t, name, body.Details["permission"])
		assert.Empty(t, e.permissionNamesOf(t, role.ID), name)
	}
}

// reservedPermissionNames is the reserved list #4574 names, written out
// so the tests do not read it back from the code under test.
var reservedPermissionNames = []string{"*:*", "admin:*", "system:manage", "permissions:create", "permissions:update", "permissions:delete"}
