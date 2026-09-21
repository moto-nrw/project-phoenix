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

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsoperator "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
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

// bookingsAuthoritativeKey is the registry key of booking-led care.
const bookingsAuthoritativeKey = "enrollment.bookings_authoritative"

// operatorSettingsHook is the side-effect hook the operator capability runs
// on each write.
type operatorSettingsHook func(ctx context.Context, tenantID int64, key string, value any) (func(), error)

// operatorSettingsTestContext holds dependencies for operator settings tests.
type operatorSettingsTestContext struct {
	db *bun.DB
	// enableBookingsAuthoritative writes the tenant setting directly through
	// the settings service, without the operator's hook and guard.
	enableBookingsAuthoritative func(ctx context.Context) error
	careWithdrawals             testpkg.CareWithdrawalWriter
	careLifecycle               careplan.CareLifecycle
	resource                    *settingsoperator.SettingsResource
	router                      chi.Router
	// onValueSet is the side-effect hook the resource runs on each write.
	// A test replaces it before the request whose hook call it observes.
	onValueSet operatorSettingsHook
}

// runValueSetHook is the hook the resource is built with; it runs the
// context's current hook.
func (c *operatorSettingsTestContext) runValueSetHook(ctx context.Context, tenantID int64, key string, value any) (func(), error) {
	if c.onValueSet == nil {
		return nil, nil
	}
	return c.onValueSet(ctx, tenantID, key, value)
}

func setupOperatorSettingsRoute(t *testing.T) *operatorSettingsTestContext {
	t.Helper()

	db, svc := testutil.SetupOperatorSettingsModule(t)
	tc := &operatorSettingsTestContext{
		db: db,
		enableBookingsAuthoritative: func(ctx context.Context) error {
			return svc.Settings.SetValue(ctx, bookingsAuthoritativeKey, true, nil, nil)
		},
		careWithdrawals: svc.CareWithdrawals,
		careLifecycle:   svc.CareLifecycle,
		onValueSet:      svc.SettingsSideEffects.Dispatch,
	}
	// Pass nil schoolRepo: the integration tests cover the mutation contract
	// (set/reset/permissions/hooks). Slug-resolution wiring is exercised end
	// to end via the platform-level integration suite.
	resource := settingsoperator.NewSettingsResource(settingsoperator.SettingsConfig{
		Settings: svc.OperatorSchoolSettings(tc.runValueSetHook),
	})

	// Operator routes do not use TenantTxMiddleware — the operator settings
	// capability opens the tenant transaction of the school ID from the URL path.
	router := chi.NewRouter()
	router.Get("/schools/{id}/settings/schema", resource.GetSchoolSettingsSchema)
	router.Get("/schools/{id}/settings/booking-authority-impact", resource.GetBookingAuthorityImpact)
	router.Get("/schools/{id}/settings/values/{key}/reveal", resource.RevealSchoolSettingValue)
	router.Put("/schools/{id}/settings/values/{key}", resource.SetSchoolSettingValue)
	router.Delete("/schools/{id}/settings/values/{key}", resource.ResetSchoolSettingValue)

	tc.resource = resource
	tc.router = router
	return tc
}

// newOperatorRequest builds a request with platform-scope operator claims.
// Uses testutil.WithClaims for consistency (injects claims into context).
func newOperatorRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()
	return testutil.NewAuthenticatedRequest(t, method, target, body, testutil.WithClaims(t, operatorTestClaims()))
}

// schoolPath builds an operator settings path for the school this test owns.
// The school id IS the tenant id, so a hardcoded /schools/1 would pin the test
// to the bootstrap tenant (#2419).
func schoolPath(tb testing.TB, suffix string) string {
	tb.Helper()
	return fmt.Sprintf("/schools/%d%s", testpkg.Tenant(tb), suffix)
}

// =============================================================================
// GET /schools/{id}/settings/schema
// =============================================================================

func TestOperatorGetSchoolSettingsSchema_Success(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/schema"), nil)
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
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, "/schools/not-a-number/settings/schema", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// PUT /schools/{id}/settings/values/{key}
// =============================================================================

func TestOperatorSetSchoolSettingValue_Success(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	body := map[string]interface{}{"value": "18:30"}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/operations.session_end_time"), body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	previewReq := newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/booking-authority-impact"), nil)
	preview := testutil.ExecuteRequest(ctx.router, previewReq)
	testutil.AssertSuccessResponse(t, preview, http.StatusOK)
	assert.Contains(t, preview.Body.String(), `"blocking_children":[]`)

	// The preview is advisory. A child can enter the unsafe set before the
	// operator confirms; the locked write must evaluate the current rows again.
	student := testpkg.CreateTestStudent(t, ctx.db, "Ohne", "Buchung", "2a")
	body = map[string]interface{}{"value": true}
	setReq := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/enrollment.bookings_authoritative"), body)
	setResult := testutil.ExecuteRequest(ctx.router, setReq)
	testutil.AssertErrorResponse(t, setResult, http.StatusConflict)

	overrides, err := ctx.db.NewSelect().TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", bookingsAuthoritativeKey).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, overrides, "the blocked write must roll the setting override back")
	assert.Positive(t, student.ID)
}

func TestOperatorSetSchoolSettingValue_BypassesPermissionCheck(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	// feedback.enabled requires config:manage for tenant users and is a
	// shared setting operators may write. Operators have empty permissions
	// but should still succeed because the handler passes nil to SetValue
	// to bypass per-setting permission checks.
	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/feedback.enabled"), body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestOperatorSetSchoolSettingValue_UnknownKey(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	body := map[string]interface{}{"value": "anything"}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/nonexistent.key"), body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestOperatorSetSchoolSettingValue_InvalidValue(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	// session_end_enabled is a boolean — string value should fail validation.
	body := map[string]interface{}{"value": "not-a-boolean"}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/operations.session_end_enabled"), body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestOperatorSetSchoolSettingValue_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := httptest.NewRequest(http.MethodPut, schoolPath(t, "/settings/values/operations.session_end_time"), bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	// Apply operator claims option directly.
	testutil.WithClaims(t, operatorTestClaims())(req)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestOperatorSetSchoolSettingValue_InvalidSchoolID(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	body := map[string]interface{}{"value": "18:30"}
	req := newOperatorRequest(t, http.MethodPut, "/schools/abc/settings/values/operations.session_end_time", body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// DELETE /schools/{id}/settings/values/{key}
// =============================================================================

func TestOperatorResetSchoolSettingValue_Success(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)
	tenantCtx := testpkg.Ctx(t)
	require.NoError(t, ctx.enableBookingsAuthoritative(tenantCtx))
	student := testpkg.CreateTestStudent(t, ctx.db, "Reset", "Buchungsmodus", "2c")
	studentID := student.ID
	testpkg.CreateTestBookingExpiredCareWithdrawal(t, ctx.careWithdrawals, studentID)
	pendingFilter := careplan.CareWithdrawalFilter{StudentID: studentID, Page: 1, PageSize: 1}
	_, pending, err := ctx.careLifecycle.ListPendingWithdrawals(tenantCtx, pendingFilter)
	require.NoError(t, err)
	require.Equal(t, 1, pending, "the booking-expired task must be pending before the reset")

	req := newOperatorRequest(t, http.MethodDelete, schoolPath(t, "/settings/values/enrollment.bookings_authoritative"), nil)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)

	_, pending, err = ctx.careLifecycle.ListPendingWithdrawals(tenantCtx, pendingFilter)
	require.NoError(t, err)
	assert.Zero(t, pending)
}

func TestOperatorResetSchoolSettingValue_UnknownKey(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodDelete, schoolPath(t, "/settings/values/nonexistent.key"), nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestOperatorResetSchoolSettingValue_InvalidSchoolID(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodDelete, "/schools/xyz/settings/values/operations.session_end_time", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// GET /schools/{id}/settings/values/{key}/reveal
// =============================================================================

func TestOperatorRevealSchoolSettingValue_Success(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/values/operations.session_end_time/reveal"), nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "response should contain data")
	_, hasValue := data["value"]
	assert.True(t, hasValue, "reveal response should include value field")
}

func TestOperatorRevealSchoolSettingValue_UnknownKey(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/values/nonexistent.key/reveal"), nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestOperatorRevealSchoolSettingValue_InvalidSchoolID(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, "/schools/bogus/settings/values/operations.session_end_time/reveal", nil)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// =============================================================================
// AccessPolicy enforcement — operators must not touch AccessAdminOnly settings
// =============================================================================

func TestOperatorSetSchoolSettingValue_AdminOnlyForbidden(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	// security.ogs_device_pin is AccessAdminOnly — operators must not change it.
	body := map[string]interface{}{"value": "1234"}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/security.ogs_device_pin"), body)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestOperatorResetSchoolSettingValue_AdminOnlyForbidden(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodDelete, schoolPath(t, "/settings/values/security.ogs_device_pin"), nil)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestOperatorRevealSchoolSettingValue_AdminOnlyForbidden(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/values/security.ogs_device_pin/reveal"), nil)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestOperatorGetSchoolSettingsSchema_HidesAdminOnly(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	req := newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/schema"), nil)
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
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	// Register a hook and assert it fires with the expected args.
	var called bool
	var capturedTenantID int64
	var capturedKey string
	var capturedValue any
	ctx.onValueSet = func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		called = true
		capturedTenantID = tenantID
		capturedKey = key
		capturedValue = value
		return nil, nil
	}

	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/checkout.schulhof_enabled"), body)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	require.True(t, called, "OnValueSet hook must fire when operator writes a shared setting")
	assert.Equal(t, testpkg.Tenant(t), capturedTenantID, "hook must receive the school ID from the URL")
	assert.Equal(t, "checkout.schulhof_enabled", capturedKey)
	assert.Equal(t, true, capturedValue)
}

func TestOperatorSetSchoolSettingValue_OnValueSetErrorRollsBackWrite(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	// Count rows for this key before the failed write. The rollback test
	// only requires that the count does NOT change; tolerating any pre-
	// existing seed data.
	const testKey = "checkout.wc_enabled"
	before, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", testKey).
		Count(context.Background())
	require.NoError(t, err)

	ctx.onValueSet = func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return nil, errors.New("hook rejected the change")
	}

	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/")+testKey, body)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)

	after, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", testKey).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before, after, "failed hook must roll back the write (row count should not change)")
}

func TestOperatorResetSchoolSettingValue_NonPhotoKeyDoesNotInvokeOnValueSet(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	seed := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/checkout.schulhof_enabled"), map[string]interface{}{
		"value": true,
	})
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, seed), http.StatusOK)

	var called bool
	ctx.onValueSet = func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		called = true
		return nil, nil
	}

	req := newOperatorRequest(t, http.MethodDelete, schoolPath(t, "/settings/values/checkout.schulhof_enabled"), nil)
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
	assert.False(t, called, "non-photo operator reset must not fire OnValueSet")
}

// =============================================================================
// Presence-mode switch guard (must-fix #1 from review round 2)
// =============================================================================

// presenceModeBinary is the binary presence mode the guard protects against
// while attendance of the day is still open.
const presenceModeBinary = "binary"

func presenceModePath(tenantID int64, suffix string) string {
	return fmt.Sprintf("/schools/%d/settings/values/%s%s", tenantID, settings.KeyPresenceMode, suffix)
}

func TestOperatorSetSchoolSettingValue_PresenceMode_BlockedByOpenAttendance(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Guard", "Block", "9a")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Guard", "Staff")
	device := testpkg.CreateTestDevice(t, ctx.db, "guard-device-001")

	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, device.ID, checkInTime, nil)

	body := map[string]interface{}{"value": presenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, presenceModePath(tenantID, ""), body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "Moduswechsel")
}

func TestOperatorSetSchoolSettingValue_PresenceMode_ForceBypassesOpenAttendance(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Guard", "Force", "9b")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Guard", "Staff2")
	device := testpkg.CreateTestDevice(t, ctx.db, "guard-device-002")

	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, device.ID, checkInTime, nil)

	body := map[string]interface{}{"value": presenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, presenceModePath(tenantID, "?force=true"), body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestOperatorSetSchoolSettingValue_PresenceMode_PassesWithNoOpenAttendance(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	body := map[string]interface{}{"value": presenceModeBinary}
	req := newOperatorRequest(t, http.MethodPut, presenceModePath(testpkg.Tenant(t), ""), body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
