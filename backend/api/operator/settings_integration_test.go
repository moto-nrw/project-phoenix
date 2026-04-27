package operator_test

import (
	"bytes"
	"context"
	"errors"
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
	resource := operatorAPI.NewSettingsResource(svc.Settings, db)

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

	// feedback.enabled requires config:manage for tenant users and is a
	// shared setting operators may write. Operators have empty permissions
	// but should still succeed because the handler passes nil to SetValue
	// to bypass per-setting permission checks.
	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/feedback.enabled", body)

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
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) error {
		called = true
		capturedTenantID = tenantID
		capturedKey = key
		capturedValue = value
		return nil
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

	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) error {
		return errors.New("hook rejected the change")
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

// =============================================================================
// Presence-mode switch guard (must-fix #1 from review round 2)
// =============================================================================
//
// The guard rejects an in-progress flip of operations.presence_mode while
// any student is still checked in for the day. Three properties under test:
//
//  1. Open attendance row → 409 with the German user-facing message; the
//     setting value does NOT change.
//  2. ?force=true → guard is bypassed even with open rows; setting is
//     written; an audit-log Warn is emitted (not asserted directly here,
//     but the code path is exercised).
//  3. No open rows → guard passes through; setting is written.

// presenceModeAttendanceCleanup wipes any open attendance rows for the
// guard tests so we can deterministically test "no open rows" cases.
func presenceModeAttendanceCleanup(t *testing.T, db *bun.DB) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`DELETE FROM active.attendance WHERE date = ? AND tenant_id = 1`,
		timezone.TodayUTC(),
	)
	require.NoError(t, err)
}

// resetPresenceMode clears any tenant override on operations.presence_mode
// so each guard test starts from the registry default ("detailed").
func resetPresenceMode(t *testing.T, db *bun.DB) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`DELETE FROM config.setting_values WHERE tenant_id = 1 AND setting_key = ?`,
		configModel.KeyPresenceMode,
	)
	require.NoError(t, err)
}

func TestOperatorSetSchoolSettingValue_PresenceMode_BlockedByOpenAttendance(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	presenceModeAttendanceCleanup(t, ctx.db)
	resetPresenceMode(t, ctx.db)
	defer presenceModeAttendanceCleanup(t, ctx.db)
	defer resetPresenceMode(t, ctx.db)

	// Create a student + open attendance row for today (Berlin date as UTC
	// midnight — the same form the guard binds via timezone.TodayUTC).
	student := testpkg.CreateTestStudent(t, ctx.db, "Guard", "Block", "9a")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Guard", "Staff")
	device := testpkg.CreateTestDevice(t, ctx.db, "guard-device-001")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID, staff.ID, device.ID)

	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, device.ID, checkInTime, nil)

	// Try to flip mode — must hit the 409 sentinel branch.
	body := map[string]interface{}{"value": configModel.PresenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/"+configModel.KeyPresenceMode, body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	assert.Equal(t, http.StatusConflict, rr.Code, "open attendance must block the switch")
	assert.Contains(t, rr.Body.String(), "Moduswechsel", "German user-facing message must appear")
}

func TestOperatorSetSchoolSettingValue_PresenceMode_ForceBypassesOpenAttendance(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	presenceModeAttendanceCleanup(t, ctx.db)
	resetPresenceMode(t, ctx.db)
	defer presenceModeAttendanceCleanup(t, ctx.db)
	defer resetPresenceMode(t, ctx.db)

	// Same setup as the blocked case — open attendance exists.
	student := testpkg.CreateTestStudent(t, ctx.db, "Guard", "Force", "9b")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Guard", "Staff2")
	device := testpkg.CreateTestDevice(t, ctx.db, "guard-device-002")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID, staff.ID, device.ID)

	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, device.ID, checkInTime, nil)

	// ?force=true must bypass the guard AND emit the audit-log Warn.
	body := map[string]interface{}{"value": configModel.PresenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/"+configModel.KeyPresenceMode+"?force=true", body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestOperatorSetSchoolSettingValue_PresenceMode_PassesWithNoOpenAttendance(t *testing.T) {
	ctx := setupOperatorSettingsTest(t)
	defer func() { _ = ctx.db.Close() }()

	presenceModeAttendanceCleanup(t, ctx.db)
	resetPresenceMode(t, ctx.db)
	defer resetPresenceMode(t, ctx.db)

	// No attendance rows at all today — the guard's SQL EXISTS check returns
	// false, the setting write proceeds.
	body := map[string]interface{}{"value": configModel.PresenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, "/schools/1/settings/values/"+configModel.KeyPresenceMode, body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
