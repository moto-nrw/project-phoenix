package auth_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestTenantAndOrganizationAdminsCannotMutateGlobalPermissionCatalog(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)
	actor := testpkg.CreateTestAccount(t, tc.db, "permission-catalog-scoped-admin")
	permissions := []string{"admin:*"}

	for _, tt := range []struct {
		name  string
		scope string
	}{
		{name: "tenant", scope: tenant.ScopeTenant},
		{name: "organization", scope: tenant.ScopeOrg},
	} {
		t.Run(tt.name, func(t *testing.T) {
			claims := testutil.AdminTestClaims(int(actor.ID))
			claims.Scope = tt.scope
			if tt.scope == tenant.ScopeOrg {
				claims.OrgID = 1
			}

			listReq := testutil.NewJSONRequest(t, http.MethodGet, "/auth/permissions", nil)
			listResp := testutil.ExecuteWithAuthPermissions(t, router, listReq, claims, permissions)
			testutil.AssertSuccessResponse(t, listResp, http.StatusOK)

			createResource := fmt.Sprintf("f18-create-%s-%d", tt.name, time.Now().UnixNano())
			createReq := testutil.NewJSONRequest(t, http.MethodPost, "/auth/permissions", map[string]string{
				"name":        createResource + ":read",
				"description": "F18 scoped create attempt",
				"resource":    createResource,
				"action":      "read",
			})
			createResp := testutil.ExecuteWithAuthPermissions(t, router, createReq, claims, permissions)
			assert.Equal(t, http.StatusForbidden, createResp.Code, "Body: %s", createResp.Body.String())
			createCount, err := tc.db.NewSelect().
				TableExpr("auth.permissions").
				Where("name = ?", createResource+":read").
				Count(testpkg.Ctx(t))
			require.NoError(t, err)
			assert.Zero(t, createCount)

			updateTarget := testpkg.CreateTestPermission(t, tc.db, "F18Update", "f18-update", "read")
			updateReq := testutil.NewJSONRequest(t, http.MethodPut, fmt.Sprintf("/auth/permissions/%d", updateTarget.ID), map[string]string{
				"name":        updateTarget.Name,
				"description": "F18 scoped update attempt",
				"resource":    updateTarget.Resource,
				"action":      "write",
			})
			updateResp := testutil.ExecuteWithAuthPermissions(t, router, updateReq, claims, permissions)
			assert.Equal(t, http.StatusForbidden, updateResp.Code, "Body: %s", updateResp.Body.String())
			var action string
			err = tc.db.NewSelect().
				TableExpr("auth.permissions").
				Column("action").
				Where("id = ?", updateTarget.ID).
				Scan(testpkg.Ctx(t), &action)
			require.NoError(t, err)
			assert.Equal(t, "read", action)

			deleteTarget := testpkg.CreateTestPermission(t, tc.db, "F18Delete", "f18-delete", "read")
			deleteReq := testutil.NewJSONRequest(t, http.MethodDelete, fmt.Sprintf("/auth/permissions/%d", deleteTarget.ID), nil)
			deleteResp := testutil.ExecuteWithAuthPermissions(t, router, deleteReq, claims, permissions)
			assert.Equal(t, http.StatusForbidden, deleteResp.Code, "Body: %s", deleteResp.Body.String())
			deleteCount, err := tc.db.NewSelect().
				TableExpr("auth.permissions").
				Where("id = ?", deleteTarget.ID).
				Count(testpkg.Ctx(t))
			require.NoError(t, err)
			assert.Equal(t, 1, deleteCount)
		})
	}
}

func TestPlatformScopeCanMutateGlobalPermissionCatalog(t *testing.T) {
	t.Parallel()

	fixtureDB := testpkg.SetupTestDB(t)
	serveDB, err := database.DBConnForServe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, serveDB.Close()) })

	repos := repositories.NewFactory(serveDB)
	svc, err := services.NewFactory(repos, serveDB, slog.Default())
	require.NoError(t, err)
	authResource := authAPI.NewResource(svc.Auth, svc.Invitation, nil, serveDB)
	router := testutil.NewTenantRouter(serveDB)
	router.Mount("/auth", authResource.Router())

	actor := testpkg.CreateTestAccount(t, fixtureDB, "permission-catalog-platform")
	claims := testutil.AdminTestClaims(int(actor.ID))
	claims.Scope = tenant.ScopePlatform
	claims.TenantID = 0
	permissions := []string{"admin:*"}

	resource := fmt.Sprintf("f18-platform-%d", time.Now().UnixNano())
	createReq := testutil.NewJSONRequest(t, http.MethodPost, "/auth/permissions", map[string]string{
		"name":        resource + ":read",
		"description": "F18 platform create",
		"resource":    resource,
		"action":      "read",
	})
	createResp := testutil.ExecuteWithAuthPermissions(t, router, createReq, claims, permissions)
	require.Equal(t, http.StatusCreated, createResp.Code, "Body: %s", createResp.Body.String())
	data := testutil.ParseJSONResponse(t, createResp.Body.Bytes())["data"].(map[string]interface{})
	permissionID := int64(data["id"].(float64))

	updateReq := testutil.NewJSONRequest(t, http.MethodPut, fmt.Sprintf("/auth/permissions/%d", permissionID), map[string]string{
		"name":        resource + ":write",
		"description": "F18 platform update",
		"resource":    resource,
		"action":      "write",
	})
	updateResp := testutil.ExecuteWithAuthPermissions(t, router, updateReq, claims, permissions)
	assert.Equal(t, http.StatusNoContent, updateResp.Code, "Body: %s", updateResp.Body.String())

	deleteReq := testutil.NewJSONRequest(t, http.MethodDelete, fmt.Sprintf("/auth/permissions/%d", permissionID), nil)
	deleteResp := testutil.ExecuteWithAuthPermissions(t, router, deleteReq, claims, permissions)
	assert.Equal(t, http.StatusNoContent, deleteResp.Code, "Body: %s", deleteResp.Body.String())
}
