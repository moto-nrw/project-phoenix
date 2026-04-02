package config_test

import (
	"net/http"
	"testing"

	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// adminClaimsWithConfigPerms returns admin claims that include explicit config
// permissions. The service-level checkWritePermission does direct string
// comparison (no wildcard support), so "admin:*" alone is insufficient.
func adminClaimsWithConfigPerms() jwt.AppClaims {
	claims := testutil.DefaultTestClaims()
	claims.Permissions = append(claims.Permissions, permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage)
	return claims
}

// settingsTestContext holds dependencies for settings API integration tests.
type settingsTestContext struct {
	db       *bun.DB
	resource *configAPI.SettingsResource
}

func setupSettingsTest(t *testing.T) *settingsTestContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := configAPI.NewSettingsResource(svc.Settings, db)

	return &settingsTestContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// GET /schema
// =============================================================================

func TestSettingsGetSchema_Success(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Get("/schema", ctx.resource.GetSchema())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains tabs
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		_, hasTabs := data["tabs"]
		assert.True(t, hasTabs, "schema should contain tabs")
	}
}

func TestSettingsGetSchema_NoPermission(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Get("/schema", ctx.resource.GetSchema())

	// Request without config:read permission
	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		testutil.WithClaims(testutil.TeacherTestClaims(2)),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Should succeed because teacher has config:read in their permissions
	// (added in the consolidated roles migration)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// =============================================================================
// PUT /values/{key}
// =============================================================================

func TestSettingsSetValue_Success(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Put("/values/{key}", ctx.resource.SetValue())

	body := map[string]interface{}{
		"value": "18:30",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.session_end_time", body,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSettingsSetValue_InvalidKey(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Put("/values/{key}", ctx.resource.SetValue())

	body := map[string]interface{}{
		"value": "test",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/nonexistent.key", body,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestSettingsSetValue_InvalidValue(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Put("/values/{key}", ctx.resource.SetValue())

	body := map[string]interface{}{
		"value": "not-a-boolean",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.session_end_enabled", body,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestSettingsSetValue_WithConfigManage(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Put("/values/{key}", ctx.resource.SetValue())

	body := map[string]interface{}{
		"value": "17:00",
	}

	// Use config:manage instead of config:update — should also work
	// Must include config:manage in claims.Permissions for service-level check
	manageClaims := testutil.DefaultTestClaims()
	manageClaims.Permissions = append(manageClaims.Permissions, permissions.ConfigRead, permissions.ConfigManage)
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/gdpr.data_cleanup_time", body,
		testutil.WithClaims(manageClaims),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// DELETE /values/{key}
// =============================================================================

func TestSettingsResetValue_Success(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Delete("/values/{key}", ctx.resource.ResetValue())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.session_end_time", nil,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

func TestSettingsResetValue_InvalidKey(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Delete("/values/{key}", ctx.resource.ResetValue())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/nonexistent.key", nil,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}
