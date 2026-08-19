package config_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/api/common"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// init seeds JWT viper defaults before any test so SettingsRouter() (which
// calls jwt.MustNewTokenAuth) and MintTestJWT share the same signing secret.
// CI runs without a .env, so AUTH_JWT_SECRET is otherwise unset.
func init() {
	testutil.SeedTestJWTConfig()
}

// mint signs a JWT for the given claims and returns a RequestOption that sets
// the Authorization: Bearer header. Tests drive SettingsRouter() through the
// production middleware chain (Verifier → Authenticator → TenantMiddleware →
// RequiresPermission → TenantTxMiddleware), which rejects unsigned requests.
func mint(t *testing.T, claims jwt.AppClaims) testutil.RequestOption {
	t.Helper()
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))
}

// adminClaimsWithConfigPerms returns admin claims with explicit config permissions.
// While "admin:*" now works via wildcard matching, explicit permissions make
// tests clearer about which permissions are being exercised.
func adminClaimsWithConfigPerms() jwt.AppClaims {
	claims := testutil.DefaultTestClaims()
	claims.Permissions = append(claims.Permissions, permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage)
	return claims
}

// teacherClaimsWithConfigRead returns teacher (read-only) claims carrying
// config:read — the permission a teacher role holds in production (added in
// the consolidated roles migration). The route-level RequiresPermission checks
// in SettingsRouter() reject a teacher token without it; the handler still
// reports can_edit=false because the teacher lacks config:update/config:manage.
func teacherClaimsWithConfigRead() jwt.AppClaims {
	claims := testutil.TeacherTestClaims(2)
	claims.Permissions = append(claims.Permissions, permissions.ConfigRead)
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

	resource := configAPI.NewSettingsResource(svc.Settings, db, nil)
	resource.SetPayrollStatusService(svc.PayrollStatus)

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

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		mint(t, adminClaimsWithConfigPerms()),
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

	router := ctx.resource.SettingsRouter()

	// Request without config:read permission
	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, teacherClaimsWithConfigRead())),
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

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "18:30",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_daily_checkout_time", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSettingsSetValue_InvalidKey(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "test",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/nonexistent.key", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestSettingsSetValue_InvalidValue(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "not-a-boolean",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.raumwechsel_enabled", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestSettingsSetValue_WithConfigManage(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "17:00",
	}

	// Use config:manage instead of config:update — should also work
	// Must include config:manage in claims.Permissions for service-level check
	manageClaims := testutil.DefaultTestClaims()
	manageClaims.Permissions = append(manageClaims.Permissions, permissions.ConfigRead, permissions.ConfigManage)
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/gdpr.data_cleanup_time", body,
		mint(t, manageClaims),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// DELETE /values/{key}
// =============================================================================

func TestSettingsResetValue_Success(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.student_daily_checkout_time", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

func TestSettingsResetValue_InvalidKey(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/nonexistent.key", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

// =============================================================================
// GET /login-image
// =============================================================================

func TestSettingsGetLoginImage_Success(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/login-image", nil,
		mint(t, adminClaimsWithConfigPerms()),
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

	router := ctx.resource.SettingsRouter()

	// Teacher has config:read but not config:update or config:manage
	req := testutil.NewAuthenticatedRequest(t, "GET", "/login-image", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, teacherClaimsWithConfigRead())),
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

	// uploadLoginImage uses WithAdminTx internally — no tenant tx middleware
	router := ctx.resource.SettingsRouter()

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
		mint(t, adminClaimsWithConfigPerms()),
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
		filePath := uploadedFilePath(t, imageURL)
		_ = os.Remove(filePath)
	})
}

func TestSettingsUploadLoginImage_ReplacesOldImage(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

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
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr1 := testutil.ExecuteRequest(router, req1)
	assert.Equal(t, http.StatusOK, rr1.Code, "first upload should succeed. Body: %s", rr1.Body.String())

	response1 := testutil.ParseJSONResponse(t, rr1.Body.Bytes())
	data1 := response1["data"].(map[string]interface{})
	firstURL := data1["login_image_url"].(string)
	firstPath := uploadedFilePath(t, firstURL)

	// Verify first file exists on disk
	_, err := os.Stat(firstPath)
	assert.NoError(t, err, "first uploaded file should exist on disk")

	// Upload second image (should replace the first)
	req2 := testutil.NewMultipartRequest(t, "POST", "/login-image",
		"login_image", "second.png", pngContent,
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr2 := testutil.ExecuteRequest(router, req2)
	assert.Equal(t, http.StatusOK, rr2.Code, "second upload should succeed. Body: %s", rr2.Body.String())

	response2 := testutil.ParseJSONResponse(t, rr2.Body.Bytes())
	data2 := response2["data"].(map[string]interface{})
	secondURL := data2["login_image_url"].(string)
	secondPath := uploadedFilePath(t, secondURL)

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

// uploadedFilePath resolves a stored upload URL ("/uploads/login-images/x.png")
// to its location on disk the same way the application does. Building the path
// by hand from the working directory would only re-create the duplicated
// path knowledge that api/common/upload.go now owns: save, serve and delete
// all resolve through ResolvePublicDir, so the test has to as well.
func uploadedFilePath(t *testing.T, storedURL string) string {
	t.Helper()
	baseDir := "public"
	if resolved, err := common.ResolvePublicDir(); err == nil {
		baseDir = resolved
	}
	return filepath.Join(baseDir, filepath.FromSlash(strings.TrimPrefix(storedURL, "/")))
}

func TestSettingsUploadLoginImage_InvalidFileType(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	// Plain text content — not an allowed image type
	req := testutil.NewMultipartRequest(t, "POST", "/login-image",
		"login_image", "not-an-image.txt", "this is not an image",
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// DELETE /login-image
// =============================================================================

func TestSettingsDeleteLoginImage_NoExistingImage(t *testing.T) {
	ctx := setupSettingsTest(t)

	// deleteLoginImage uses WithAdminTx internally — no tenant tx middleware
	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/login-image", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

// =============================================================================
// OnValueSet Callback Tests
// =============================================================================

func TestSettingsSetValue_OnValueSetCallbackInvoked(t *testing.T) {
	ctx := setupSettingsTest(t)

	var callbackKey string
	var callbackValue any
	var callbackTenantID int64
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		callbackTenantID = tenantID
		callbackKey = key
		callbackValue = value
		return nil, nil
	})

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assert.Equal(t, "checkout.schulhof_enabled", callbackKey)
	assert.Equal(t, true, callbackValue)
	assert.Greater(t, callbackTenantID, int64(0), "callback should receive a valid tenant_id")
}

func TestSettingsSetValue_OnValueSetCallbackErrorRollsBack(t *testing.T) {
	ctx := setupSettingsTest(t)

	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return nil, errors.New("hook failed")
	})

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "17:45",
	}

	claims := adminClaimsWithConfigPerms()
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_daily_checkout_time", body,
		mint(t, claims),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)

	count, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", "operations.student_daily_checkout_time").
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "failed callback should roll back the setting update")
}

func TestSettingsSetValue_OnValueSetNotCalledOnError(t *testing.T) {
	ctx := setupSettingsTest(t)

	callbackInvoked := false
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		callbackInvoked = true
		return nil, nil
	})

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "not-a-boolean",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)

	assert.False(t, callbackInvoked, "callback should not be invoked on validation error")
}

// TestSettingsSetValue_PostCommitRunsOnSuccess locks in the contract that
// the post-commit closure returned by the OnValueSet callback runs after a
// successful tx. Photo-purge file unlinks rely on this — running them in
// the tx would mean a commit failure leaves files deleted but the photo_path
// rows untouched.
func TestSettingsSetValue_PostCommitRunsOnSuccess(t *testing.T) {
	ctx := setupSettingsTest(t)

	postCommitRan := false
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return func() {
			postCommitRan = true
		}, nil
	})

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled",
		map[string]interface{}{"value": true},
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assert.True(t, postCommitRan, "post-commit closure must run after a successful PUT")
}

// TestSettingsSetValue_PostCommitSkippedOnHookError ensures the post-commit
// closure does NOT run when the in-tx hook returns an error. If it did, a
// photo-purge file unlink would still happen for a tx that rolled back —
// the exact bug the two-phase contract was introduced to prevent.
func TestSettingsSetValue_PostCommitSkippedOnHookError(t *testing.T) {
	ctx := setupSettingsTest(t)

	postCommitRan := false
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		// Return BOTH a closure and an error — the handler must not run
		// the closure because the tx rolled back. Returning the closure
		// alongside the error proves the handler honours the contract by
		// gating on err, not on cb-presence.
		return func() {
			postCommitRan = true
		}, errors.New("hook failed")
	})

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled",
		map[string]interface{}{"value": true},
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)

	assert.False(t, postCommitRan, "post-commit closure must not run when the tx rolled back")
}

func TestSettingsSetValue_NilCallbackDoesNotPanic(t *testing.T) {
	ctx := setupSettingsTest(t)

	// No OnValueSet registered — should not panic
	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.wc_enabled", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// TestSettingsResetValue_OnValueSetCallbackInvoked guarantees the side-effect
// hook fires on reset (DELETE /values/{key}) for student_photos_enabled.
// Reset puts the setting back on its registry default, so the callback
// receives that default — otherwise resetting student_photos_enabled would
// leave already-stored photos on disk because the purge callback never runs.
func TestSettingsResetValue_OnValueSetCallbackInvoked(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	seed := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled", map[string]interface{}{
		"value": true,
	}, mint(t, adminClaimsWithConfigPerms()))
	rr := testutil.ExecuteRequest(router, seed)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Now register the callback and reset.
	var callbackKey string
	var callbackValue any
	var callbackTenantID int64
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		callbackTenantID = tenantID
		callbackKey = key
		callbackValue = value
		return nil, nil
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.student_photos_enabled", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr = testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)

	assert.Equal(t, "operations.student_photos_enabled", callbackKey)
	assert.Equal(t, false, callbackValue)
	assert.Greater(t, callbackTenantID, int64(0), "callback should receive a valid tenant_id")
}

func TestSettingsResetValue_NonPhotoKeyDoesNotInvokeOnValueSet(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	seed := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled", map[string]interface{}{
		"value": true,
	}, mint(t, adminClaimsWithConfigPerms()))
	rr := testutil.ExecuteRequest(router, seed)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var called bool
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		called = true
		return nil, nil
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/checkout.schulhof_enabled", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr = testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)

	assert.False(t, called, "non-photo reset must not fire OnValueSet")
}

// TestSettingsResetValue_OnValueSetCallbackErrorRollsBack ensures the reset
// path participates in the same tx contract as PUT for the photo flag — a
// hook error must roll back the override deletion.
func TestSettingsResetValue_OnValueSetCallbackErrorRollsBack(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	claims := adminClaimsWithConfigPerms()
	seed := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled", map[string]interface{}{
		"value": true,
	}, mint(t, claims))
	rr := testutil.ExecuteRequest(router, seed)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return nil, errors.New("hook failed")
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.student_photos_enabled", nil,
		mint(t, claims),
	)
	rr = testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)

	// Override should still exist — the failed hook rolled back the delete.
	count, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", "operations.student_photos_enabled").
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "failed hook on reset must roll back the override deletion")
}

func TestSettingsDeleteLoginImage_NoTenantContext(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	// Claims with TenantID=0 — no tenant context
	claims := adminClaimsWithConfigPerms()
	claims.TenantID = 0
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/login-image", nil,
		mint(t, claims),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// AccessPolicy enforcement — admins must not touch AccessOperatorOnly settings
// =============================================================================

func TestSettingsSetValue_OperatorOnlyForbidden(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	// operations.session_end_time is AccessOperatorOnly — tenant admins must not
	// write it even if they hold config:update. The UI hides these keys; this
	// test guards the direct-API path.
	body := map[string]interface{}{"value": "19:00"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.session_end_time", body,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestSettingsResetValue_OperatorOnlyForbidden(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.session_end_time", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestSettingsGetSchema_HidesOperatorOnly(t *testing.T) {
	ctx := setupSettingsTest(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		mint(t, adminClaimsWithConfigPerms()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, _ := response["data"].(map[string]interface{})
	tabs, _ := data["tabs"].([]interface{})

	// Walk every item and confirm no AccessOperatorOnly keys appear.
	operatorOnlyKeys := map[string]bool{
		"operations.session_end_enabled":                 true,
		"operations.session_end_time":                    true,
		"operations.session_end_timeout_minutes":         true,
		"operations.session_cleanup_enabled":             true,
		"operations.session_cleanup_interval_minutes":    true,
		"operations.session_abandoned_threshold_minutes": true,
	}
	for _, tabRaw := range tabs {
		tab := tabRaw.(map[string]interface{})
		categories, _ := tab["categories"].([]interface{})
		for _, catRaw := range categories {
			cat := catRaw.(map[string]interface{})
			items, _ := cat["items"].([]interface{})
			for _, itemRaw := range items {
				item := itemRaw.(map[string]interface{})
				key := item["key"].(string)
				assert.False(t, operatorOnlyKeys[key],
					"admin schema must not include AccessOperatorOnly key %s", key)
			}
		}
	}
}
