// Package attendance_test tests the IoT attendance API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package attendance_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	attendanceAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/attendance"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *attendanceAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create attendance resource
	resource := attendanceAPI.NewResource(
		svc.Users,
		svc.Active,
		svc.Education,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// GET ATTENDANCE STATUS TESTS
// =============================================================================

func TestGetAttendanceStatus_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/A1B2C3D4", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAttendanceStatus_MissingRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device")

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	// Request with empty RFID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/", nil,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Chi routing will result in 404 for missing param in URL
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}

func TestGetAttendanceStatus_RFIDNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-2")

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	// Request with non-existent RFID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/NONEXISTENT123", nil,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetAttendanceStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device and student with RFID
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-3")
	student := testpkg.CreateTestStudent(t, ctx.db, "Attendance", "Status", "1a")
	// Create RFID card first, then link to student
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// TOGGLE ATTENDANCE TESTS
// =============================================================================

func TestToggleAttendance_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"rfid":   "A1B2C3D4",
		"action": "confirm",
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestToggleAttendance_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-1")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Send invalid JSON body - create request manually
	req := httptest.NewRequest("POST", "/toggle", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	// Add device context
	reqCtx := context.WithValue(req.Context(), device.CtxDevice, testDevice)
	req = req.WithContext(reqCtx)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_MissingRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-2")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_Cancel(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-3")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Cancel action still requires RFID
	body := map[string]interface{}{
		"rfid":   "ANYVALUE",
		"action": "cancel",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestToggleAttendance_RFIDNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-4")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"rfid":   "NONEXISTENT999",
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// NOTE: TestToggleAttendance_Success and TestToggleAttendance_DailyCheckout require
// complex staff context setup for permission checking. These scenarios are better
// covered by Bruno API tests which have full authentication context.
// The tests above cover: device auth, validation, cancel action, and RFID not found.
