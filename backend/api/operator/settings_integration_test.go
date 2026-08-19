package operator_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	operatorAPI "github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// operatorTestClaims returns JWT claims for a platform-scoped operator.
// Operators have empty permissions — access is gated by RequiresOperatorScope
// middleware at the route level, not by per-setting permission checks.
func operatorTestClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:          1,
		Sub:         "operator@example.com",
		Username:    "operator",
		FirstName:   "Test",
		LastName:    "Operator",
		Scope:       "platform",
		Permissions: []string{},
	}
}

// operatorSettingsTestContext holds dependencies for operator settings tests.
type operatorSettingsTestContext struct {
	db       *bun.DB
	resource *operatorAPI.SettingsResource
	router   chi.Router
}

func setupOperatorSettingsTest(t *testing.T) *operatorSettingsTestContext {
	t.Helper()

	// Reset viper app_env to "test" — provisioning_internal_test.go sets it to
	// "production" and doesn't clean up, which causes service factory creation
	// to fail the HTTPS FRONTEND_URL check when tests run in the same binary.
	prevEnv := viper.GetString("app_env")
	viper.Set("app_env", "test")
	t.Cleanup(func() { viper.Set("app_env", prevEnv) })

	db, svc := testutil.SetupAPITest(t)
	// Pass nil schoolRepo: the integration tests cover the mutation contract
	// (set/reset/permissions/hooks). Slug-resolution wiring is exercised end
	// to end via the platform-level integration suite.
	resource := operatorAPI.NewSettingsResource(svc.Settings, db, nil, nil, svc.Active)

	// Operator routes do not use TenantTxMiddleware — handlers call
	// tenant.WithTenantTx internally using the school ID from the URL path.
	router := chi.NewRouter()
	router.Get("/schools/{id}/settings/schema", resource.GetSchoolSettingsSchema)
	router.Get("/schools/{id}/settings/values/{key}/reveal", resource.RevealSchoolSettingValue)
	router.Put("/schools/{id}/settings/values/{key}", resource.SetSchoolSettingValue)
	router.Delete("/schools/{id}/settings/values/{key}", resource.ResetSchoolSettingValue)

	return &operatorSettingsTestContext{
		db:       db,
		resource: resource,
		router:   router,
	}
}

// newOperatorRequest builds a request with platform-scope operator claims.
// Uses testutil.WithClaims for consistency (injects claims into context).
func newOperatorRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()
	return testutil.NewAuthenticatedRequest(t, method, target, body, testutil.WithClaims(operatorTestClaims()))
}

// =============================================================================
// GET /schools/{id}/settings/schema
// =============================================================================

func TestOperatorGetSchoolSettingsSchema_Success(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/1/settings/schema", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "response should contain data")

	// Operators must see actual tabs — passing nil permissions would cause the
	// schema builder to filter out every setting that has a ReadPermission,
	// resulting in tabs: null. The handler uses a wildcard permission to bypass
	// that filter so operators see all registered settings.
	tabs, ok := data["tabs"].([]interface{})
	require.True(t, ok, "schema.tabs should be a non-null array")
	require.NotEmpty(t, tabs, "operators should see all registered settings tabs")

	// Spot-check: every item should be marked writable for operators.
	for _, tabRaw := range tabs {
		tab, ok := tabRaw.(map[string]interface{})
		require.True(t, ok)
		categories, _ := tab["categories"].([]interface{})
		for _, catRaw := range categories {
			cat, ok := catRaw.(map[string]interface{})
			require.True(t, ok)
			items, _ := cat["items"].([]interface{})
			for _, itemRaw := range items {
				item, ok := itemRaw.(map[string]interface{})
				require.True(t, ok)
				assert.True(t, item["writable"].(bool),
					"operator should have writable=true on all settings, got false for %v", item["key"])
			}
		}
	}
}

func TestOperatorGetSchoolSettingsSchema_InvalidSchoolID(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/not-a-number/settings/schema", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// PUT /schools/{id}/settings/values/{key}
// =============================================================================

func TestOperatorSetSchoolSettingValue_Success(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]interface{}{"value": "18:30"}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/operations.session_end_time", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestOperatorSetSchoolSettingValue_BypassesPermissionCheck(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// gdpr.attendance_log_enabled requires config:manage for tenant users and
	// is a shared setting operators may write. Operators have empty permissions
	// but should still succeed because the handler passes nil to SetValue
	// to bypass per-setting permission checks.
	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/gdpr.attendance_log_enabled", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestOperatorSetSchoolSettingValue_UnknownKey(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]interface{}{"value": "anything"}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/nonexistent.key", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestOperatorSetSchoolSettingValue_InvalidValue(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// session_end_enabled is a boolean — string value should fail validation.
	body := map[string]interface{}{"value": "not-a-boolean"}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/operations.session_end_enabled", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestOperatorSetSchoolSettingValue_InvalidJSON(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := httptest.NewRequest(http.MethodPut, "/schools/1/settings/values/operations.session_end_time", bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	// Apply operator claims option directly.
	testutil.WithClaims(operatorTestClaims())(req)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestOperatorSetSchoolSettingValue_InvalidSchoolID(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]interface{}{"value": "18:30"}
	req := newOperatorRequest(t, http.MethodPut, "/schools/abc/settings/values/operations.session_end_time", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// DELETE /schools/{id}/settings/values/{key}
// =============================================================================

func TestOperatorResetSchoolSettingValue_Success(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodDelete, "/schools/1/settings/values/operations.session_end_time", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

func TestOperatorResetSchoolSettingValue_UnknownKey(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodDelete, "/schools/1/settings/values/nonexistent.key", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestOperatorResetSchoolSettingValue_InvalidSchoolID(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodDelete, "/schools/xyz/settings/values/operations.session_end_time", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// GET /schools/{id}/settings/values/{key}/reveal
// =============================================================================

func TestOperatorRevealSchoolSettingValue_Success(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/1/settings/values/operations.session_end_time/reveal", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "response should contain data")
	_, hasValue := data["value"]
	assert.True(t, hasValue, "reveal response should include value field")
}

func TestOperatorRevealSchoolSettingValue_UnknownKey(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/1/settings/values/nonexistent.key/reveal", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestOperatorRevealSchoolSettingValue_InvalidSchoolID(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/bogus/settings/values/operations.session_end_time/reveal", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// AccessPolicy enforcement — operators must not touch AccessAdminOnly settings
// =============================================================================

func TestOperatorSetSchoolSettingValue_AdminOnlyForbidden(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// security.ogs_device_pin is AccessAdminOnly — operators must not change it.
	body := map[string]interface{}{"value": "1234"}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/security.ogs_device_pin", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestOperatorResetSchoolSettingValue_AdminOnlyForbidden(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodDelete, "/schools/1/settings/values/security.ogs_device_pin", nil)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestOperatorRevealSchoolSettingValue_AdminOnlyForbidden(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/1/settings/values/security.ogs_device_pin/reveal", nil)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestOperatorGetSchoolSettingsSchema_HidesAdminOnly(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	req := newOperatorRequest(t, http.MethodGet, "/schools/1/settings/schema", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	tabs, _ := data["tabs"].([]interface{})

	// Walk every item across all tabs and confirm the PIN is not present.
	for _, tabRaw := range tabs {
		tab := tabRaw.(map[string]interface{})
		categories, _ := tab["categories"].([]interface{})
		for _, catRaw := range categories {
			cat := catRaw.(map[string]interface{})
			items, _ := cat["items"].([]interface{})
			for _, itemRaw := range items {
				item := itemRaw.(map[string]interface{})
				assert.NotEqual(t, "security.ogs_device_pin", item["key"],
					"operator schema must not include AccessAdminOnly PIN setting")
			}
		}
	}
}

// =============================================================================
// OnValueSet hook — operator writes must trigger the same side effects as
// tenant writes (e.g. auto-provisioning Schulhof/WC rooms).
// =============================================================================

func TestOperatorSetSchoolSettingValue_InvokesOnValueSetHook(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// Register a hook and assert it fires with the expected args.
	var called bool
	var capturedTenantID int64
	var capturedKey string
	var capturedValue any
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		called = true
		capturedTenantID = tenantID
		capturedKey = key
		capturedValue = value
		return nil, nil
	})

	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/checkout.schulhof_enabled", body)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	require.True(t, called, "OnValueSet hook must fire when operator writes a shared setting")
	assert.Equal(t, int64(1), capturedTenantID, "hook must receive the school ID from the URL")
	assert.Equal(t, "checkout.schulhof_enabled", capturedKey)
	assert.Equal(t, true, capturedValue)
}

func TestOperatorSetSchoolSettingValue_OnValueSetErrorRollsBackWrite(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	// Count rows for this key before the failed write. The rollback test
	// only requires that the count does NOT change; tolerating any pre-
	// existing seed data.
	const testKey = "checkout.wc_enabled"
	before, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", int64(1)).
		Where("setting_key = ?", testKey).
		Count(context.Background())
	require.NoError(t, err)

	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return nil, errors.New("hook rejected the change")
	})

	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/"+testKey, body)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)

	after, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", int64(1)).
		Where("setting_key = ?", testKey).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before, after, "failed hook must roll back the write (row count should not change)")
}

func TestOperatorResetSchoolSettingValue_NonPhotoKeyDoesNotInvokeOnValueSet(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	seed := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/checkout.schulhof_enabled", map[string]interface{}{
		"value": true,
	})
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, seed), http.StatusOK)

	var called bool
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		called = true
		return nil, nil
	})

	req := newOperatorRequest(t, http.MethodDelete, "/schools/1/settings/values/checkout.schulhof_enabled", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
	assert.False(t, called, "non-photo operator reset must not fire OnValueSet")
}

// =============================================================================
// Presence-mode switch guard (must-fix #1 from review round 2)
// =============================================================================

func presenceModeAttendanceCleanup(t *testing.T, db *bun.DB, tenantID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`DELETE FROM active.attendance WHERE date = ? AND tenant_id = ?`,
		timezone.TodayDate(),
		tenantID,
	)
	require.NoError(t, err)
}

func resetPresenceMode(t *testing.T, db *bun.DB, tenantID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`,
		tenantID,
		configModel.KeyPresenceMode,
	)
	require.NoError(t, err)
}

func createPresenceModeAttendanceForTenant(
	t *testing.T,
	db *bun.DB,
	tenantID int64,
	studentID int64,
	staffID int64,
	deviceID int64,
	checkInTime time.Time,
	checkOutTime *time.Time,
) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO active.attendance (
			tenant_id,
			student_id,
			date,
			check_in_time,
			check_out_time,
			checked_in_by,
			device_id,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		tenantID,
		studentID,
		timezone.TodayDate(),
		checkInTime,
		checkOutTime,
		staffID,
		deviceID,
	)
	require.NoError(t, err)
}

func presenceModePath(tenantID int64, suffix string) string {
	return fmt.Sprintf("/schools/%d/settings/values/%s%s", tenantID, configModel.KeyPresenceMode, suffix)
}

func TestOperatorSetSchoolSettingValue_PresenceMode_BlockedByOpenAttendance(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, ctx.db, tenantID)
	presenceModeAttendanceCleanup(t, ctx.db, tenantID)
	resetPresenceMode(t, ctx.db, tenantID)
	defer presenceModeAttendanceCleanup(t, ctx.db, tenantID)
	defer resetPresenceMode(t, ctx.db, tenantID)

	student := testpkg.CreateTestStudentForTenant(t, ctx.db, tenantID, "Guard", "Block", "9a")
	staff := testpkg.CreateTestStaffForTenant(t, ctx.db, tenantID, "Guard", "Staff")
	device := testpkg.CreateTestDeviceForTenant(t, ctx.db, tenantID, "guard-device-001")
	defer testpkg.CleanupActivityFixturesForTenant(t, ctx.db, tenantID, student.ID, staff.ID, device.ID)

	checkInTime := time.Now().Add(-1 * time.Hour)
	createPresenceModeAttendanceForTenant(t, ctx.db, tenantID, student.ID, staff.ID, device.ID, checkInTime, nil)
	defer presenceModeAttendanceCleanup(t, ctx.db, tenantID)

	body := map[string]interface{}{"value": configModel.PresenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, presenceModePath(tenantID, ""), body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "Moduswechsel")
}

func TestOperatorSetSchoolSettingValue_PresenceMode_ForceBypassesOpenAttendance(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, ctx.db, tenantID)
	presenceModeAttendanceCleanup(t, ctx.db, tenantID)
	resetPresenceMode(t, ctx.db, tenantID)
	defer presenceModeAttendanceCleanup(t, ctx.db, tenantID)
	defer resetPresenceMode(t, ctx.db, tenantID)

	student := testpkg.CreateTestStudentForTenant(t, ctx.db, tenantID, "Guard", "Force", "9b")
	staff := testpkg.CreateTestStaffForTenant(t, ctx.db, tenantID, "Guard", "Staff2")
	device := testpkg.CreateTestDeviceForTenant(t, ctx.db, tenantID, "guard-device-002")
	defer testpkg.CleanupActivityFixturesForTenant(t, ctx.db, tenantID, student.ID, staff.ID, device.ID)

	checkInTime := time.Now().Add(-1 * time.Hour)
	createPresenceModeAttendanceForTenant(t, ctx.db, tenantID, student.ID, staff.ID, device.ID, checkInTime, nil)
	defer presenceModeAttendanceCleanup(t, ctx.db, tenantID)

	body := map[string]interface{}{"value": configModel.PresenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, presenceModePath(tenantID, "?force=true"), body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestOperatorSetSchoolSettingValue_PresenceMode_PassesWithNoOpenAttendance(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, ctx.db, tenantID)
	presenceModeAttendanceCleanup(t, ctx.db, tenantID)
	resetPresenceMode(t, ctx.db, tenantID)
	defer resetPresenceMode(t, ctx.db, tenantID)

	body := map[string]interface{}{"value": configModel.PresenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, presenceModePath(tenantID, ""), body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
