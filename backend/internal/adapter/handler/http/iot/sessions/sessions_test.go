// Package sessions_test tests the IoT sessions API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package sessions_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	sessionsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/sessions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/testutil"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *sessionsAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create sessions resource
	resource := sessionsAPI.NewResource(
		svc.IoT,
		svc.Users,
		svc.Active,
		svc.Activities,
		svc.Config,
		svc.Facilities,
		svc.Education,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// START SESSION TESTS
// =============================================================================

func TestStartSession_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/start", ctx.resource.StartSessionHandler())

	body := map[string]interface{}{
		"activity_id": 1,
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/start", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestStartSession_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-1")

	router := chi.NewRouter()
	router.Post("/start", ctx.resource.StartSessionHandler())

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/start", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	reqCtx := context.WithValue(req.Context(), device.CtxDevice, testDevice)
	req = req.WithContext(reqCtx)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestStartSession_MissingActivityID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-2")

	router := chi.NewRouter()
	router.Post("/start", ctx.resource.StartSessionHandler())

	body := map[string]interface{}{
		"room_id": 1, // Missing activity_id
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/start", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestStartSession_InvalidActivityID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-3")

	router := chi.NewRouter()
	router.Post("/start", ctx.resource.StartSessionHandler())

	body := map[string]interface{}{
		"activity_id": 0, // Invalid activity_id
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/start", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// END SESSION TESTS
// =============================================================================

func TestEndSession_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/end", ctx.resource.EndSessionHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/end", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestEndSession_NoActiveSession(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-4")

	router := chi.NewRouter()
	router.Post("/end", ctx.resource.EndSessionHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/end", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for no active session
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET CURRENT SESSION TESTS
// =============================================================================

func TestGetCurrentSession_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/current", ctx.resource.GetCurrentSessionHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetCurrentSession_NoActiveSession(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-5")

	router := chi.NewRouter()
	router.Get("/current", ctx.resource.GetCurrentSessionHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return success with is_active=false
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CHECK CONFLICT TESTS
// =============================================================================

func TestCheckConflict_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/check-conflict", ctx.resource.CheckConflictHandler())

	body := map[string]interface{}{
		"activity_id": 1,
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestCheckConflict_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-6")

	router := chi.NewRouter()
	router.Post("/check-conflict", ctx.resource.CheckConflictHandler())

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/check-conflict", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	reqCtx := context.WithValue(req.Context(), device.CtxDevice, testDevice)
	req = req.WithContext(reqCtx)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCheckConflict_MissingActivityID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-7")

	router := chi.NewRouter()
	router.Post("/check-conflict", ctx.resource.CheckConflictHandler())

	body := map[string]interface{}{} // Missing activity_id

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE SUPERVISORS TESTS
// =============================================================================

func TestUpdateSupervisors_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/{sessionId}/supervisors", ctx.resource.UpdateSupervisorsHandler())

	body := map[string]interface{}{
		"supervisor_ids": []int64{1},
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/1/supervisors", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestUpdateSupervisors_InvalidSessionID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-8")

	router := chi.NewRouter()
	router.Put("/{sessionId}/supervisors", ctx.resource.UpdateSupervisorsHandler())

	body := map[string]interface{}{
		"supervisor_ids": []int64{1},
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/invalid/supervisors", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateSupervisors_EmptySupervisors(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-9")

	router := chi.NewRouter()
	router.Put("/{sessionId}/supervisors", ctx.resource.UpdateSupervisorsHandler())

	body := map[string]interface{}{
		"supervisor_ids": []int64{}, // Empty list
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/1/supervisors", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET TIMEOUT CONFIG TESTS
// Note: Timeout handlers rely on middleware for device auth, so we only test with device context
// =============================================================================

func TestGetTimeoutConfig_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-10")

	router := chi.NewRouter()
	router.Get("/timeout-config", ctx.resource.GetTimeoutConfigHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeout-config", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// UPDATE ACTIVITY TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestUpdateActivity_InvalidActivityType(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-11")

	router := chi.NewRouter()
	router.Post("/activity", ctx.resource.UpdateActivityHandler())

	body := map[string]interface{}{
		"activity_type": "invalid_type",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activity", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for invalid activity type
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// VALIDATE TIMEOUT TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestValidateTimeout_MissingTimeoutMinutes(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-12")

	router := chi.NewRouter()
	router.Post("/validate-timeout", ctx.resource.ValidateTimeoutHandler())

	body := map[string]interface{}{
		"last_activity": time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/validate-timeout", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for missing timeout_minutes (may return 500 if validation fails in service)
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusInternalServerError}, rr.Code)
}

func TestValidateTimeout_InvalidTimeoutMinutes(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-13")

	router := chi.NewRouter()
	router.Post("/validate-timeout", ctx.resource.ValidateTimeoutHandler())

	body := map[string]interface{}{
		"timeout_minutes": 1000, // Too large (max 480)
		"last_activity":   time.Now().Format(time.RFC3339),
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/validate-timeout", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for invalid timeout_minutes (may return 500 if validation fails in service)
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// GET TIMEOUT INFO TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestGetTimeoutInfo_NoActiveSession(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-14")

	router := chi.NewRouter()
	router.Get("/timeout-info", ctx.resource.GetTimeoutInfoHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeout-info", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error when no active session
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// PROCESS TIMEOUT TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestProcessTimeout_NoActiveSession(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-15")

	router := chi.NewRouter()
	router.Post("/timeout", ctx.resource.ProcessTimeoutHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeout", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error when no active session
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}, rr.Code)
}
