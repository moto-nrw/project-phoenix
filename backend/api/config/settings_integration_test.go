package config_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// adminClaimsWithConfigPerms returns admin claims with explicit config permissions.
// While "admin:*" now works via wildcard matching, explicit permissions make
// tests clearer about which permissions are being exercised.
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

// =============================================================================
// GET /login-image
// =============================================================================

func TestSettingsGetLoginImage_Success(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Get("/login-image", ctx.resource.GetLoginImage())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/login-image", nil,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "response should contain data")
	assert.Nil(t, data["login_image_url"], "default school should have no login image")
	assert.True(t, data["can_edit"].(bool), "admin should have edit permission")
}

func TestSettingsGetLoginImage_ReadOnlyUser(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := testutil.NewTenantRouter(ctx.db)
	router.Get("/login-image", ctx.resource.GetLoginImage())

	// Teacher has config:read but not config:update or config:manage
	req := testutil.NewAuthenticatedRequest(t, "GET", "/login-image", nil,
		testutil.WithClaims(testutil.TeacherTestClaims(2)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "response should contain data")
	assert.False(t, data["can_edit"].(bool), "teacher should not have edit permission")
}

// =============================================================================
// POST /login-image
// =============================================================================

func TestSettingsUploadLoginImage_Success(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// uploadLoginImage uses WithAdminTx internally — no tenant tx middleware
	router := chi.NewRouter()
	router.Post("/login-image", ctx.resource.UploadLoginImage())

	// Minimal valid PNG (1x1 pixel)
	pngContent := string([]byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00,
		0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92, 0xef,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D',
		0xae, 'B', 0x60, 0x82,
	})

	req := testutil.NewMultipartRequest(t, "POST", "/login-image",
		"login_image", "test-login.png", pngContent,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	assert.Equal(t, http.StatusOK, rr.Code, "upload should succeed. Body: %s", rr.Body.String())

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "response should contain data")
	imageURL, _ := data["login_image_url"].(string)
	assert.Contains(t, imageURL, "/uploads/login-images/")
	assert.Contains(t, imageURL, ".png")

	// Clean up the uploaded file
	t.Cleanup(func() {
		filePath := filepath.Join("public", filepath.FromSlash(imageURL[1:]))
		_ = os.Remove(filePath)
	})
}

func TestSettingsUploadLoginImage_ReplacesOldImage(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/login-image", ctx.resource.UploadLoginImage())

	// Minimal valid PNG (1x1 pixel)
	pngContent := string([]byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00,
		0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92, 0xef,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D',
		0xae, 'B', 0x60, 0x82,
	})

	// Upload first image
	req1 := testutil.NewMultipartRequest(t, "POST", "/login-image",
		"login_image", "first.png", pngContent,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)
	rr1 := testutil.ExecuteRequest(router, req1)
	assert.Equal(t, http.StatusOK, rr1.Code, "first upload should succeed. Body: %s", rr1.Body.String())

	response1 := testutil.ParseJSONResponse(t, rr1.Body.Bytes())
	data1 := response1["data"].(map[string]interface{})
	firstURL := data1["login_image_url"].(string)
	firstPath := filepath.Join("public", filepath.FromSlash(firstURL[1:]))

	// Verify first file exists on disk
	_, err := os.Stat(firstPath)
	assert.NoError(t, err, "first uploaded file should exist on disk")

	// Upload second image (should replace the first)
	req2 := testutil.NewMultipartRequest(t, "POST", "/login-image",
		"login_image", "second.png", pngContent,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)
	rr2 := testutil.ExecuteRequest(router, req2)
	assert.Equal(t, http.StatusOK, rr2.Code, "second upload should succeed. Body: %s", rr2.Body.String())

	response2 := testutil.ParseJSONResponse(t, rr2.Body.Bytes())
	data2 := response2["data"].(map[string]interface{})
	secondURL := data2["login_image_url"].(string)
	secondPath := filepath.Join("public", filepath.FromSlash(secondURL[1:]))

	// Verify old file was cleaned up
	_, err = os.Stat(firstPath)
	assert.True(t, os.IsNotExist(err), "old file should have been deleted after re-upload")

	// Verify new file exists
	_, err = os.Stat(secondPath)
	assert.NoError(t, err, "new uploaded file should exist on disk")

	// Clean up
	t.Cleanup(func() {
		_ = os.Remove(secondPath)
	})
}

func TestSettingsUploadLoginImage_InvalidFileType(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/login-image", ctx.resource.UploadLoginImage())

	// Plain text content — not an allowed image type
	req := testutil.NewMultipartRequest(t, "POST", "/login-image",
		"login_image", "not-an-image.txt", "this is not an image",
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// DELETE /login-image
// =============================================================================

func TestSettingsDeleteLoginImage_NoExistingImage(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// deleteLoginImage uses WithAdminTx internally — no tenant tx middleware
	router := chi.NewRouter()
	router.Delete("/login-image", ctx.resource.DeleteLoginImage())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/login-image", nil,
		testutil.WithClaims(adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

func TestSettingsDeleteLoginImage_NoTenantContext(t *testing.T) {
	ctx := setupSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/login-image", ctx.resource.DeleteLoginImage())

	// Claims with TenantID=0 — no tenant context
	claims := adminClaimsWithConfigPerms()
	claims.TenantID = 0
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/login-image", nil,
		testutil.WithClaims(claims),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}
