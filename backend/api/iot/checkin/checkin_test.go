package checkin_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
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
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains expected fields
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
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
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains expected fields
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
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
	body := map[string]interface{}{
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
	body := map[string]interface{}{
		"action": "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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

	body := map[string]interface{}{
		"student_rfid": "nonexistent-rfid-tag",
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout", // Explicit checkout action
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed with checkout
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
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

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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
	body := map[string]interface{}{
		"student_rfid": 12345, // wrong type - should be string
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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

	body := map[string]interface{}{
		"student_rfid": "",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// With proper staff context, checkin should succeed
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
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
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// With proper staff context, room transfer should succeed
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
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
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
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

	body := map[string]interface{}{
		"student_rfid": "test-rfid",
		"action":       "invalid",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
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

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Checkout without active visit should fail - no room_id provided and nothing to checkout
	testutil.AssertBadRequest(t, rr)
}
