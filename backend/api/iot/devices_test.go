package iot

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	resource devicesTestResource
}

type devicesTestResource struct {
	*DevicesResource
	tb testing.TB
}

func (rs devicesTestResource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, granted := testutil.AuthenticationContext(r.Context())
			principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
				AccountID: int64(claims.ID), TenantID: claims.TenantID, OrganizationID: claims.OrgID,
				Scope: claims.Scope, Roles: claims.Roles, Permissions: granted, Admin: claims.IsAdmin, FamilyID: claims.FamilyID,
			})
			if err != nil {
				rs.tb.Errorf("build test security principal: %v", err)
				http.Error(w, "invalid test principal", http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r.WithContext(permissions.WithPrincipal(r.Context(), principal)))
		})
	})
	router.Mount("/", rs.DevicesResource.Router())
	return router
}

// setupDevicesModule initializes the devices route.
func setupDevicesModule(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupDeviceModule(t)

	resource := devicesTestResource{DevicesResource: NewDevicesResource(svc.IoT), tb: t}

	return &testContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// LIST DEVICES TESTS
// =============================================================================

func TestListDevices_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListDevices_WithTypeFilter(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices?device_type=terminal", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListDevices_WithStatusFilter(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices?status=active", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListDevices_WithSearchFilter(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices?search=test", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICE TESTS
// =============================================================================

func TestGetDevice_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device
	uniqueID := fmt.Sprintf("test-device-%d", time.Now().UnixNano())
	device := testpkg.CreateTestDevice(t, ctx.db, uniqueID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/devices/%d", device.ID), nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDevice_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/999999", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetDevice_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/invalid", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET DEVICE BY DEVICE ID TESTS
// =============================================================================

func TestGetDeviceByDeviceID_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device - the fixture appends its own unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "test-device")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	// Use device.DeviceID which includes the fixture's unique suffix
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/devices/device/%s", device.DeviceID), nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDeviceByDeviceID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/device/nonexistent-device", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
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
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	uniqueID := fmt.Sprintf("new-device-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"device_id":   uniqueID,
		"device_type": "terminal",
		"name":        "Test Device",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

}

func TestCreateDevice_NewDeviceHasNoRoom(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	uniqueID := fmt.Sprintf("new-device-no-room-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"device_id":   uniqueID,
		"device_type": "terminal",
		"name":        "Device Without Room",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})

	// A newly created device has no room — location is auto-derived from sessions
	assert.Nil(t, data["room_name"], "new device should have no room_name before any session")
}

func TestCreateDevice_MissingDeviceID(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	body := map[string]interface{}{
		"device_type": "terminal",
		"name":        "Test Device",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateDevice_MissingDeviceType(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	uniqueID := fmt.Sprintf("new-device-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"device_id": uniqueID,
		"name":      "Test Device",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices", body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE DEVICE TESTS
// =============================================================================

func TestUpdateDevice_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device
	uniqueID := fmt.Sprintf("update-device-%d", time.Now().UnixNano())
	device := testpkg.CreateTestDevice(t, ctx.db, uniqueID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	body := map[string]interface{}{
		"device_id":   uniqueID,
		"device_type": "terminal",
		"name":        "Updated Device Name",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/devices/%d", device.ID), body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateDevice_PreservesSessionDerivedRoom(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "update-room-device")
	room := testpkg.CreateTestRoom(t, ctx.db, "UpdateDevice-SessionRoom")

	// Simulate a session having set the device's room_id (as auto-derive would)
	_, err := ctx.db.NewUpdate().
		Model(device).
		ModelTableExpr(`iot.devices`).
		Set("room_id = ?", room.ID).
		Where("id = ?", device.ID).
		Exec(testpkg.Ctx(t))
	assert.NoError(t, err)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	// Update only the name — room_id is not in the request (auto-derived, not manual)
	body := map[string]interface{}{
		"device_id":   device.DeviceID,
		"device_type": device.DeviceType,
		"name":        "Updated Device Name",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/devices/%d", device.ID), body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})

	// The session-derived room should be preserved after updating other fields
	assert.Equal(t, room.Name, data["room_name"])
}

func TestUpdateDevice_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	body := map[string]interface{}{
		"device_id":   "test",
		"device_type": "terminal",
		"name":        "Test",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/devices/999999", body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateDevice_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	body := map[string]interface{}{
		"device_id":   "test",
		"device_type": "terminal",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/devices/invalid", body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE DEVICE TESTS
// =============================================================================

func TestDeleteDevice_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device
	uniqueID := fmt.Sprintf("delete-device-%d", time.Now().UnixNano())
	device := testpkg.CreateTestDevice(t, ctx.db, uniqueID)
	// Note: No defer cleanup needed since we're deleting it

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/devices/%d", device.ID), nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteDevice_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/devices/999999", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteDevice_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/devices/invalid", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE DEVICE STATUS TESTS
// =============================================================================

func TestUpdateDeviceStatus_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device - use device.DeviceID which includes fixture's unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "status-device")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	body := map[string]interface{}{
		"status": "maintenance",
	}

	req := testutil.NewAuthenticatedRequest(t, "PATCH", fmt.Sprintf("/devices/%s/status", device.DeviceID), body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateDeviceStatus_MissingStatus(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device - use device.DeviceID which includes fixture's unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "status-missing")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "PATCH", fmt.Sprintf("/devices/%s/status", device.DeviceID), body,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// PING DEVICE TESTS
// =============================================================================

func TestPingDevice_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test device - use device.DeviceID which includes fixture's unique suffix
	device := testpkg.CreateTestDevice(t, ctx.db, "ping-device")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/devices/%s/ping", device.DeviceID), nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:update"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestPingDevice_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices/nonexistent-device/ping", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
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
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/type/terminal", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICES BY STATUS TESTS
// =============================================================================

func TestGetDevicesByStatus_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/status/active", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDevicesByStatus_InvalidStatus(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/status/invalid_status", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET DEVICES BY REGISTERED BY TESTS
// =============================================================================

func TestGetDevicesByRegisteredBy_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	// Create test person
	person := testpkg.CreateTestPerson(t, ctx.db, "RegisteredBy", "Test")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/devices/registered-by/%d", person.ID), nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDevicesByRegisteredBy_InvalidPersonID(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/registered-by/invalid", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET ACTIVE DEVICES TESTS
// =============================================================================

func TestGetActiveDevices_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/active", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICES REQUIRING MAINTENANCE TESTS
// =============================================================================

func TestGetDevicesRequiringMaintenance_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/maintenance", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET OFFLINE DEVICES TESTS
// =============================================================================

func TestGetOfflineDevices_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/offline", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetOfflineDevices_WithDurationFilter(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/offline?duration=30m", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:read"),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET DEVICE STATISTICS TESTS
// =============================================================================

func TestGetDeviceStatistics_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/devices/statistics", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
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
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices/detect-new", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
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
	t.Parallel()
	ctx := setupDevicesModule(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/devices", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/devices/scan-network", nil,
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
		testutil.WithPermissions("iot:manage"),
	)

	rr := testutil.ExecuteRequest(router, req)

	// NOTE: This endpoint is not implemented in the service layer yet.
	// Returns "network scanning not implemented" error.
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "not implemented")
}
