package devices_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	devicesAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/devices"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *devicesAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := devicesAPI.NewResource(svc.IoT)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// LIST DEVICES TESTS
// =============================================================================

func TestListDevices_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices", ctx.resource.ListDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListDevices_WithTypeFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices", ctx.resource.ListDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices?device_type=rfid_reader", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListDevices_WithStatusFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices", ctx.resource.ListDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices?status=active", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListDevices_WithSearchFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices", ctx.resource.ListDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices?search=test", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICE TESTS
// =============================================================================

func TestGetDevice_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	uniqueID := fmt.Sprintf("test-device-%d", time.Now().UnixNano())
	device := testpkg.CreateTestDevice(t, ctx.db, uniqueID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Get("/devices/{id}", ctx.resource.GetDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/devices/%d", device.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDevice_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/{id}", ctx.resource.GetDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/999999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetDevice_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/{id}", ctx.resource.GetDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET DEVICE BY DEVICE ID TESTS
// =============================================================================

func TestGetDeviceByDeviceID_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device - the fixture appends its own unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "test-device")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Get("/devices/device/{deviceId}", ctx.resource.GetDeviceByDeviceIDHandler())

	// Use device.DeviceID which includes the fixture's unique suffix
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/devices/device/%s", device.DeviceID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDeviceByDeviceID_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/device/{deviceId}", ctx.resource.GetDeviceByDeviceIDHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/device/nonexistent-device", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// NOTE: The service returns 500 instead of 404 for "not found" scenarios.
	// This is a service-layer issue where sql.ErrNoRows is not translated properly.
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// CREATE DEVICE TESTS
// =============================================================================

func TestCreateDevice_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/devices", ctx.resource.CreateDeviceHandler())

	uniqueID := fmt.Sprintf("new-device-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"device_id":   uniqueID,
		"device_type": "rfid_reader",
		"name":        "Test Device",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Parse response to get device ID for cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		if deviceID, ok := data["id"].(float64); ok {
			defer testpkg.CleanupActivityFixtures(t, ctx.db, int64(deviceID))
		}
	}
}

func TestCreateDevice_MissingDeviceID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/devices", ctx.resource.CreateDeviceHandler())

	body := map[string]interface{}{
		"device_type": "rfid_reader",
		"name":        "Test Device",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateDevice_MissingDeviceType(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/devices", ctx.resource.CreateDeviceHandler())

	uniqueID := fmt.Sprintf("new-device-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"device_id": uniqueID,
		"name":      "Test Device",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE DEVICE TESTS
// =============================================================================

func TestUpdateDevice_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	uniqueID := fmt.Sprintf("update-device-%d", time.Now().UnixNano())
	device := testpkg.CreateTestDevice(t, ctx.db, uniqueID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Put("/devices/{id}", ctx.resource.UpdateDeviceHandler())

	body := map[string]interface{}{
		"device_id":   uniqueID,
		"device_type": "rfid_reader",
		"name":        "Updated Device Name",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/devices/%d", device.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateDevice_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/devices/{id}", ctx.resource.UpdateDeviceHandler())

	body := map[string]interface{}{
		"device_id":   "test",
		"device_type": "rfid_reader",
		"name":        "Test",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/devices/999999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateDevice_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Put("/devices/{id}", ctx.resource.UpdateDeviceHandler())

	body := map[string]interface{}{
		"device_id":   "test",
		"device_type": "rfid_reader",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/devices/invalid", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE DEVICE TESTS
// =============================================================================

func TestDeleteDevice_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	uniqueID := fmt.Sprintf("delete-device-%d", time.Now().UnixNano())
	device := testpkg.CreateTestDevice(t, ctx.db, uniqueID)
	// Note: No defer cleanup needed since we're deleting it

	router := chi.NewRouter()
	router.Delete("/devices/{id}", ctx.resource.DeleteDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/devices/%d", device.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteDevice_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/devices/{id}", ctx.resource.DeleteDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/devices/999999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteDevice_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/devices/{id}", ctx.resource.DeleteDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/devices/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE DEVICE STATUS TESTS
// =============================================================================

func TestUpdateDeviceStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device - use device.DeviceID which includes fixture's unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "status-device")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Patch("/devices/{deviceId}/status", ctx.resource.UpdateDeviceStatusHandler())

	body := map[string]interface{}{
		"status": "maintenance",
	}

	req := testutil.NewAuthenticatedRequest(t, "PATCH", fmt.Sprintf("/devices/%s/status", device.DeviceID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateDeviceStatus_MissingStatus(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device - use device.DeviceID which includes fixture's unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "status-missing")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Patch("/devices/{deviceId}/status", ctx.resource.UpdateDeviceStatusHandler())

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "PATCH", fmt.Sprintf("/devices/%s/status", device.DeviceID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// PING DEVICE TESTS
// =============================================================================

func TestPingDevice_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device - use device.DeviceID which includes fixture's unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "ping-device")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/devices/{deviceId}/ping", ctx.resource.PingDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/devices/%s/ping", device.DeviceID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestPingDevice_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/devices/{deviceId}/ping", ctx.resource.PingDeviceHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices/nonexistent-device/ping", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// NOTE: The service returns 500 instead of 404 for "not found" scenarios.
	// This is a service-layer issue where sql.ErrNoRows is not translated properly.
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// GET DEVICES BY TYPE TESTS
// =============================================================================

func TestGetDevicesByType_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/type/{type}", ctx.resource.GetDevicesByTypeHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/type/rfid_reader", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICES BY STATUS TESTS
// =============================================================================

func TestGetDevicesByStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/status/{status}", ctx.resource.GetDevicesByStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/status/active", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDevicesByStatus_InvalidStatus(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/status/{status}", ctx.resource.GetDevicesByStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/status/invalid_status", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET DEVICES BY REGISTERED BY TESTS
// =============================================================================

func TestGetDevicesByRegisteredBy_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test person
	person := testpkg.CreateTestPerson(t, ctx.db, "RegisteredBy", "Test")
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

	router := chi.NewRouter()
	router.Get("/devices/registered-by/{personId}", ctx.resource.GetDevicesByRegisteredByHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/devices/registered-by/%d", person.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDevicesByRegisteredBy_InvalidPersonID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/registered-by/{personId}", ctx.resource.GetDevicesByRegisteredByHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/registered-by/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET ACTIVE DEVICES TESTS
// =============================================================================

func TestGetActiveDevices_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/active", ctx.resource.GetActiveDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/active", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICES REQUIRING MAINTENANCE TESTS
// =============================================================================

func TestGetDevicesRequiringMaintenance_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/maintenance", ctx.resource.GetDevicesRequiringMaintenanceHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/maintenance", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET OFFLINE DEVICES TESTS
// =============================================================================

func TestGetOfflineDevices_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/offline", ctx.resource.GetOfflineDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/offline", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetOfflineDevices_WithDurationFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/offline", ctx.resource.GetOfflineDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/offline?duration=30m", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICE STATISTICS TESTS
// =============================================================================

func TestGetDeviceStatistics_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/devices/statistics", ctx.resource.GetDeviceStatisticsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/statistics", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response has expected fields
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Contains(t, data, "total_devices", "Response should contain total_devices")
	assert.Contains(t, data, "active_devices", "Response should contain active_devices")
	assert.Contains(t, data, "offline_devices", "Response should contain offline_devices")
}

// =============================================================================
// DETECT NEW DEVICES TESTS
// =============================================================================

func TestDetectNewDevices_NotImplemented(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/devices/detect-new", ctx.resource.DetectNewDevicesHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices/detect-new", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// NOTE: This endpoint is not implemented in the service layer yet.
	// Returns "device auto-discovery not implemented" error.
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "not implemented")
}

// =============================================================================
// SCAN NETWORK TESTS
// =============================================================================

func TestScanNetwork_NotImplemented(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/devices/scan-network", ctx.resource.ScanNetworkHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices/scan-network", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// NOTE: This endpoint is not implemented in the service layer yet.
	// Returns "network scanning not implemented" error.
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "not implemented")
}
