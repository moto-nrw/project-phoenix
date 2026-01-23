package checkin_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *checkinAPI.Resource
	ogsID    string
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	ogsID := testpkg.SetupTestOGS(t, db)

	resource := checkinAPI.NewResource(
		svc.IoT,
		svc.Users,
		svc.Active,
		svc.Facilities,
		svc.Activities,
		svc.Education,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// createTestDeviceContext creates a device context for testing.
func createTestDeviceContext(device *iot.Device) *iot.Device {
	// Set LastSeen to now for IsOnline() to return true
	now := time.Now()
	device.LastSeen = &now
	return device
}

// =============================================================================
// DEVICE PING TESTS
// =============================================================================

func TestDevicePing_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, ctx.db)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "ping-test", ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/ping", ctx.resource.DevicePingHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/ping", nil,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains expected fields
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Contains(t, data, "device_id", "Response should contain device_id")
	assert.Contains(t, data, "status", "Response should contain status")
	assert.Contains(t, data, "is_online", "Response should contain is_online")
	assert.Contains(t, data, "ping_time", "Response should contain ping_time")
}

func TestDevicePing_Unauthorized(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/checkin/ping", ctx.resource.DevicePingHandler())

	// Request without device context
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/ping", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// DEVICE STATUS TESTS
// =============================================================================

func TestDeviceStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "status-test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Get("/checkin/status", ctx.resource.DeviceStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/checkin/status", nil,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains expected fields
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Contains(t, data, "device", "Response should contain device")
	assert.Contains(t, data, "authenticated_at", "Response should contain authenticated_at")
}

func TestDeviceStatus_Unauthorized(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/checkin/status", ctx.resource.DeviceStatusHandler())

	// Request without device context
	req := testutil.NewAuthenticatedRequest(t, "GET", "/checkin/status", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// DEVICE CHECKIN TESTS
// =============================================================================

func TestDeviceCheckin_Unauthorized(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Request without device context
	body := map[string]any{
		"student_rfid": "12345",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestDeviceCheckin_MissingRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-missing-rfid", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Request without student_rfid
	body := map[string]any{
		"action": "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDeviceCheckin_StudentNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-not-found", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": "nonexistent-rfid-tag",
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 404 when RFID tag not found
	testutil.AssertNotFound(t, rr)
}

func TestDeviceCheckin_NoActiveGroups(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-no-groups", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "CheckIn", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card and link it to the student's person
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)

	// Link the RFID card to the person by updating the person's tag_id
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create test room for checkin (but no active groups)
	room := testpkg.CreateTestRoom(t, ctx.db, "Checkin Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Returns 404 when room has no active groups running
	// Note: A successful checkin requires an active group session in the room
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DEVICE CHECKOUT TESTS
// =============================================================================

func TestDeviceCheckin_CheckoutWithActiveVisit(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "Checkout", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card and link it to the student's person
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room and activity
	room := testpkg.CreateTestRoom(t, ctx.db, "Checkout Test Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "Checkout Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activityGroup.ID)

	// Create active group in the room
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create an active visit for the student (with entry time and nil exit time for active visit)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now(), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Perform checkout by scanning RFID without room_id
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkout", // Explicit checkout action
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed with checkout
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")

	// Verify checkout action
	if data != nil {
		assert.Equal(t, "checked_out", data["action"])
	}
}

func TestDeviceCheckin_CheckinWithNewVisitNoActiveGroup(t *testing.T) {
	// This test verifies that checkin to a room without an active group fails appropriately
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-new", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "New", "Visit", "2b", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card and link it
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room WITHOUT an active group - checkin should fail
	room := testpkg.CreateTestRoom(t, ctx.db, "New Visit Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail because no active group in room
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// STAFF RFID TESTS
// =============================================================================

func TestDeviceCheckin_StaffRFIDNotSupported(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "staff-rfid", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test staff member with RFID tag
	staff := testpkg.CreateTestStaff(t, ctx.db, "Staff", "Member", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create RFID card and link it to the staff's person
	tagID := fmt.Sprintf("STAFF%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	// Link RFID to staff's person (same approach as student)
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Staff RFID via checkin endpoint should return 404 with specific message
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// ROOM TRANSFER TESTS
// =============================================================================

func TestDeviceCheckin_RoomTransferInvalidRoom(t *testing.T) {
	// This test verifies that attempting to transfer to a room without an active group fails
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Student", "3c", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card
	tagID := fmt.Sprintf("TRANSFER%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room for activity
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Room 1", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room1.ID)

	// Create room 2 WITHOUT an active group
	room2 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Room 2", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room2.ID)

	// Create activity and active group only for room 1
	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Transfer Activity 1", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity1.ID)

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup1.ID)

	// Create initial visit in room 1 (active visit = nil exit time)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now(), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Try to transfer to room 2 which has no active group
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail because room 2 has no active group
	// The student checkout from room 1 will succeed, but checkin to room 2 will fail
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// INVALID REQUEST TESTS
// =============================================================================

func TestDeviceCheckin_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "invalid-json", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Send invalid JSON using the standard method with an invalid body type
	// The handler expects JSON, so sending a map with invalid structure
	body := map[string]any{
		"student_rfid": 12345, // wrong type - should be string
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// This should either be bad request or not found (depends on validation)
	// The RFID must be a string, but JSON marshaling may succeed
	// Let's verify the response is appropriate
	assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound,
		"Expected bad request or not found, got %d", rr.Code)
}

func TestDeviceCheckin_EmptyRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "empty-rfid", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": "",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

func TestRouter_CheckinEndpointExists(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", nil)

	rr := testutil.ExecuteRequest(router, req)

	// 401 indicates endpoint exists but requires device authentication
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRouter_PingEndpointExists(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/ping", nil)

	rr := testutil.ExecuteRequest(router, req)

	// 401 indicates endpoint exists but requires device authentication
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRouter_StatusEndpointExists(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/status", nil)

	rr := testutil.ExecuteRequest(router, req)

	// 401 indicates endpoint exists but requires device authentication
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// =============================================================================
// SUCCESSFUL CHECKIN TESTS (with active groups)
// =============================================================================

func TestDeviceCheckin_SuccessfulCheckin(t *testing.T) {
	// Full checkin requires staff context for attendance tracking (checked_in_by FK constraint).
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "success-checkin", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Checkin", "Staff", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Success", "Checkin", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SUCCESS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room with active group
	room := testpkg.CreateTestRoom(t, ctx.db, "Success Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Success Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// With proper staff context, checkin should succeed
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
}

func TestDeviceCheckin_RoomTransferSucceeds(t *testing.T) {
	// Room transfer: checkout from room 1, checkin to room 2.
	// Requires staff context for attendance tracking (checked_in_by FK constraint).
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Transfer", "Staff", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Test", "2b", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("TRANS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room 1 with activity
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Room A", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room1.ID)

	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity A", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity1.ID)

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup1.ID)

	// Create room 2 with activity
	room2 := testpkg.CreateTestRoom(t, ctx.db, "Room B", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room2.ID)

	activity2 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity B", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity2.ID)

	activeGroup2 := testpkg.CreateTestActiveGroup(t, ctx.db, activity2.ID, room2.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup2.ID)

	// Create initial visit in room 1
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now().Add(-10*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Transfer to room 2
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// With proper staff context, room transfer should succeed
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "transferred", data["action"])
}

// =============================================================================
// DEVICE SESSION ACTIVITY TESTS
// =============================================================================

func TestDevicePing_SessionActiveStatus(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "session-ping", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/ping", ctx.resource.DevicePingHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/ping", nil,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Contains(t, data, "session_active", "Should include session_active status")
	// Without active session, session_active should be false
	assert.Equal(t, false, data["session_active"])
}

// =============================================================================
// INVALID ACTION TESTS
// =============================================================================

func TestDeviceCheckin_InvalidAction(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "invalid-action", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": "test-rfid",
		"action":       "invalid",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Invalid action should fail validation
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CHECKOUT WITHOUT CHECKIN TESTS
// =============================================================================

func TestDeviceCheckin_CheckoutWithoutActiveVisit(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-no-visit", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create student with RFID (no active visit)
	student := testpkg.CreateTestStudent(t, ctx.db, "NoVisit", "Test", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("NOVISIT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkout",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Checkout without active visit should fail - no room_id provided and nothing to checkout
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ROOM CAPACITY TESTS
// =============================================================================

func TestDeviceCheckin_RoomAtCapacity(t *testing.T) {
	// Test that checkin fails when room is at capacity
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "capacity-test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Capacity", "Staff", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create room with very small capacity (1)
	room := testpkg.CreateTestRoom(t, ctx.db, "Small Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	// Update room capacity to 1
	_, err := ctx.db.NewUpdate().
		TableExpr("facilities.rooms").
		Set("capacity = ?", 1).
		Where("id = ?", room.ID).
		Exec(context.Background())
	assert.NoError(t, err)

	// Create activity
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Capacity Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	// Create active group
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create first student already in the room (room is now at capacity)
	student1 := testpkg.CreateTestStudent(t, ctx.db, "Existing", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)
	visit1 := testpkg.CreateTestVisit(t, ctx.db, student1.ID, activeGroup.ID, time.Now().Add(-5*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit1.ID)

	// Create second student trying to check in
	student2 := testpkg.CreateTestStudent(t, ctx.db, "New", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student2.ID)

	tagID := fmt.Sprintf("CAPACITY%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail because room is at capacity (409 Conflict)
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected conflict when room at capacity")
}

// =============================================================================
// ACTIVITY CAPACITY TESTS
// =============================================================================

func TestDeviceCheckin_ActivityAtCapacity(t *testing.T) {
	// Test that checkin fails when activity is at max participants
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "activity-cap", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "Activity", "Cap", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create room with large capacity
	room := testpkg.CreateTestRoom(t, ctx.db, "Large Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	// Create activity with small max participants (1)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Small Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	// Update activity max participants to 1
	_, err := ctx.db.NewUpdate().
		TableExpr(`activities.groups`).
		Set("max_participants = ?", 1).
		Where("id = ?", activity.ID).
		Exec(context.Background())
	assert.NoError(t, err)

	// Create active group
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create first student in the activity (activity now at capacity)
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "InActivity", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)
	visit1 := testpkg.CreateTestVisit(t, ctx.db, student1.ID, activeGroup.ID, time.Now().Add(-5*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit1.ID)

	// Create second student
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "ForActivity", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student2.ID)

	tagID := fmt.Sprintf("ACTCAP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail because activity is at max participants (409 Conflict)
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected conflict when activity at capacity")
}

// =============================================================================
// SAME ROOM CHECKIN (SKIP CHECKIN) TESTS
// =============================================================================

func TestDeviceCheckin_SameRoomSkipsCheckin(t *testing.T) {
	// Test that checkin to the same room results in checkout only (no re-checkin)
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "same-room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "Same", "Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create room
	room := testpkg.CreateTestRoom(t, ctx.db, "Same Room Test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	// Create activity and active group
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Same Room Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create student with active visit in the room
	student := testpkg.CreateTestStudent(t, ctx.db, "Same", "RoomTest", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SAMEROOM%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create active visit
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Scan RFID with same room_id - should checkout without re-checkin
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID, // Same room
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	// Same room scan results in checkout only
	assert.Equal(t, "checked_out", data["action"], "Same room scan should result in checkout")
}

// =============================================================================
// MULTIPLE ACTIVE GROUPS TESTS
// =============================================================================

func TestDeviceCheckin_MultipleActiveGroupsInRoom(t *testing.T) {
	// Test checkin when room has multiple active groups (should use first one)
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "multi-group", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "Multi", "Group", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create room
	room := testpkg.CreateTestRoom(t, ctx.db, "Multi Group Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	// Create two activities with active groups in the same room
	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity One", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity1.ID)
	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup1.ID)

	activity2 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity Two", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity2.ID)
	activeGroup2 := testpkg.CreateTestActiveGroup(t, ctx.db, activity2.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup2.ID)

	// Create student
	student := testpkg.CreateTestStudent(t, ctx.db, "Multi", "GroupStudent", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("MULTIGROUP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed - uses first active group
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
}

// =============================================================================
// WHITESPACE AND SPECIAL CHARACTER TESTS
// =============================================================================

func TestDeviceCheckin_WhitespaceRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "whitespace", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": "   ", // Only whitespace
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Whitespace-only RFID results in "not found" after trimming
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// VISIT WITH EXIT TIME TESTS
// =============================================================================

func TestDeviceCheckin_VisitAlreadyExited(t *testing.T) {
	// Test that a student with already-exited visit can check in again
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "exited-visit", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "Exited", "Visit", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create room and activity
	room := testpkg.CreateTestRoom(t, ctx.db, "Exited Visit Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Exited Visit Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create student
	student := testpkg.CreateTestStudent(t, ctx.db, "Exited", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("EXITED%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create visit with exit time set (already exited)
	exitTime := time.Now().Add(-5 * time.Minute)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), &exitTime, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed - previous visit is already ended, so this is a new checkin
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
}

// =============================================================================
// PERSON WITHOUT STUDENT OR STAFF TESTS
// =============================================================================

func TestDeviceCheckin_PersonNeitherStudentNorStaff(t *testing.T) {
	// Test that a person without student or staff record gets appropriate error
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "person-only", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create just a person (no student or staff record)
	person := testpkg.CreateTestPerson(t, ctx.db, "Just", "Person", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, person.ID)

	// Create RFID card and link to person
	tagID := fmt.Sprintf("PERSONONLY%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, person.ID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail - person is neither student nor staff
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DEVICE SESSION UPDATE TESTS
// =============================================================================

func TestDevicePing_WithActiveSession(t *testing.T) {
	// Test that ping with active session reports session_active=true
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "session-ping", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create room and activity
	room := testpkg.CreateTestRoom(t, ctx.db, "Session Ping Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Session Ping Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	// Create active group with device_id set
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Link device to active group
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(context.Background())
	assert.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/checkin/ping", ctx.resource.DevicePingHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/ping", nil,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, true, data["session_active"], "Should report active session")
}

// =============================================================================
// NEWRESOURCE AND ROUTER TESTS
// =============================================================================

func TestNewResource_CreatesValidResource(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	assert.NotNil(t, ctx.resource, "NewResource should create non-nil resource")
}

// =============================================================================
// DEVICE HANDLERS TESTS
// =============================================================================

func TestDeviceCheckinHandler_Returns(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	handler := ctx.resource.DeviceCheckinHandler()
	assert.NotNil(t, handler, "DeviceCheckinHandler should return non-nil handler")
}

func TestDevicePingHandler_Returns(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	handler := ctx.resource.DevicePingHandler()
	assert.NotNil(t, handler, "DevicePingHandler should return non-nil handler")
}

func TestDeviceStatusHandler_Returns(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	handler := ctx.resource.DeviceStatusHandler()
	assert.NotNil(t, handler, "DeviceStatusHandler should return non-nil handler")
}

// =============================================================================
// SESSION ACTIVITY UPDATE TESTS
// =============================================================================

func TestDeviceCheckin_UpdatesSessionActivity(t *testing.T) {
	// Test that checkin updates session activity for devices
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "session-update", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Session", "Update", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Session Update Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Session Update Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Link device to active group
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(context.Background())
	assert.NoError(t, err)

	student := testpkg.CreateTestStudent(t, ctx.db, "Session", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SESSIONACT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CHECKOUT PATH TESTS
// =============================================================================

func TestDeviceCheckin_CheckoutWithoutActiveGroup(t *testing.T) {
	// Test checkout when visit has no active group loaded
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-nogroup", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Checkout", "NoGroup", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Checkout No Group Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Checkout No Group Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Checkout", "NoGroup", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("CHECKOUTNG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create active visit
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Request checkout action (no room_id)
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkout",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_out", data["action"])
}

// =============================================================================
// ROOM CAPACITY AND ACTIVITY CAPACITY TOGETHER
// =============================================================================

func TestDeviceCheckin_RoomHasCapacityActivityFull(t *testing.T) {
	// Test when room has capacity but activity is full
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "room-ok-act-full", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "RoomOK", "ActFull", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Room with large capacity
	room := testpkg.CreateTestRoom(t, ctx.db, "Room Large Cap", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	_, err := ctx.db.NewUpdate().
		TableExpr(`facilities.rooms`).
		Set("capacity = ?", 100).
		Where("id = ?", room.ID).
		Exec(context.Background())
	assert.NoError(t, err)

	// Activity with max 1 participant
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Small Cap Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	_, err = ctx.db.NewUpdate().
		TableExpr(`activities.groups`).
		Set("max_participants = ?", 1).
		Where("id = ?", activity.ID).
		Exec(context.Background())
	assert.NoError(t, err)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// First student in activity (fills it up)
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "InAct", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)
	visit1 := testpkg.CreateTestVisit(t, ctx.db, student1.ID, activeGroup.ID, time.Now().Add(-5*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit1.ID)

	// Second student tries to check in
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "TryAct", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student2.ID)

	tagID := fmt.Sprintf("ROOMOKACT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail with activity at capacity (409 Conflict)
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected conflict when activity at capacity")
}

// =============================================================================
// TRANSFER TESTS
// =============================================================================

func TestDeviceCheckin_TransferBetweenRooms(t *testing.T) {
	// Test student transferring from one room to another
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Transfer", "Test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create two rooms
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Source Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room1.ID)

	room2 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Dest Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room2.ID)

	// Activity and active groups for both rooms
	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Transfer Source Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity1.ID)

	activity2 := testpkg.CreateTestActivityGroup(t, ctx.db, "Transfer Dest Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity2.ID)

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup1.ID)

	activeGroup2 := testpkg.CreateTestActiveGroup(t, ctx.db, activity2.ID, room2.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup2.ID)

	// Student currently in room1
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("TRANSFER%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create active visit in room1
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now().Add(-10*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Request checkin to room2 (should trigger transfer)
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "transferred", data["action"], "Should be a transfer action")
	previousRoom, ok := data["previous_room"].(string)
	assert.True(t, ok, "previous_room should be a string")
	assert.Contains(t, previousRoom, "Transfer Source Room", "Should show previous room")
}

// =============================================================================
// NO ROOM ID TESTS
// =============================================================================

func TestDeviceCheckin_NoRoomIDJustCheckout(t *testing.T) {
	// Test checkin with no room_id when student has active visit - should just checkout
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "no-room-checkout", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "NoRoom", "Checkout", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "NoRoom Checkout Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "NoRoom Checkout Activity", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "NoRoom", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("NOROOMCO%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create active visit
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil, ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Request checkin with no room_id - should just checkout
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_out", data["action"], "Should checkout when no room_id provided")
}

// =============================================================================
// ROOM NOT FOUND TESTS
// =============================================================================

func TestDeviceCheckin_NonExistentRoom(t *testing.T) {
	// Test checkin to a room that doesn't exist
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "nonexistent-room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Nonexistent", "Room", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Nonexistent", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("NOROOM%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Request checkin to non-existent room
	body := map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      int64(999999999), // Very high ID unlikely to exist
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithTenantContext(&tenant.TenantContext{OrgID: ctx.ogsID, OrgName: "Test OGS"}),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Non-existent room returns internal server error (500)
	// The code returns 500 when room lookup fails
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Expected internal server error for non-existent room")
}
