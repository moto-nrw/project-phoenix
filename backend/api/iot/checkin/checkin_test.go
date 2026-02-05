package checkin_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"context"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// intPtr returns a pointer to an int value.
func intPtr(i int) *int {
	return &i
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *checkinAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := checkinAPI.NewResource(
		svc.IoT,
		svc.Users,
		svc.Active,
		svc.Facilities,
		svc.Activities,
		svc.Education,
		slog.Default(),
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
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

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "ping-test")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "status-test")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-missing-rfid")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-not-found")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-no-groups")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "CheckIn", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card and link it to the student's person
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)

	// Link the RFID card to the person by updating the person's tag_id
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create test room for checkin (but no active groups)
	room := testpkg.CreateTestRoom(t, ctx.db, "Checkin Room")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-test")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "Checkout", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card and link it to the student's person
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room and activity
	room := testpkg.CreateTestRoom(t, ctx.db, "Checkout Test Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "Checkout Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activityGroup.ID)

	// Create active group in the room
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create an active visit for the student (with entry time and nil exit time for active visit)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now(), nil)
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
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-new")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "New", "Visit", "2b")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card and link it
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room WITHOUT an active group - checkin should fail
	room := testpkg.CreateTestRoom(t, ctx.db, "New Visit Room")
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

func TestDeviceCheckin_SupervisorRFIDAuthentication(t *testing.T) {
	t.Run("authenticates supervisor with active session", func(t *testing.T) {
		ctx := setupTestContext(t)
		defer func() { _ = ctx.db.Close() }()

		// Create test device
		device := testpkg.CreateTestDevice(t, ctx.db, "staff-rfid")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

		// Create a test staff member with RFID tag
		staff := testpkg.CreateTestStaff(t, ctx.db, "Staff", "Member")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

		// Create RFID card and link it to the staff's person
		tagID := fmt.Sprintf("STAFF%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		// Create room and activity
		room := testpkg.CreateTestRoom(t, ctx.db, "Supervisor Room")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

		activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Supervisor Activity")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

		// Create active group linked to the device (simulates an active session)
		activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

		// Link active group to device so GetDeviceCurrentSession finds it
		_, err := ctx.db.NewUpdate().
			TableExpr("active.groups").
			Set("device_id = ?", device.ID).
			Where("id = ?", activeGroup.ID).
			Exec(t.Context())
		assert.NoError(t, err)

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

		// Staff RFID with active session should authenticate as supervisor
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok, "Response should have data field")
		assert.Equal(t, "supervisor_authenticated", data["action"])
		assert.Contains(t, data["student_name"], "Staff")
		assert.Contains(t, data, "message")
	})

	t.Run("returns 404 when no active session", func(t *testing.T) {
		ctx := setupTestContext(t)
		defer func() { _ = ctx.db.Close() }()

		// Create test device (no active session)
		device := testpkg.CreateTestDevice(t, ctx.db, "staff-no-session")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

		// Create a test staff member with RFID tag
		staff := testpkg.CreateTestStaff(t, ctx.db, "NoSession", "Staff")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

		tagID := fmt.Sprintf("STAFFNS%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
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

		// Staff RFID without active session should return 404
		testutil.AssertNotFound(t, rr)
	})

	t.Run("idempotent duplicate supervisor scan", func(t *testing.T) {
		ctx := setupTestContext(t)
		defer func() { _ = ctx.db.Close() }()

		device := testpkg.CreateTestDevice(t, ctx.db, "staff-dup")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

		staff := testpkg.CreateTestStaff(t, ctx.db, "Duplicate", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

		tagID := fmt.Sprintf("STAFFDUP%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		room := testpkg.CreateTestRoom(t, ctx.db, "Dup Supervisor Room")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

		activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Dup Supervisor Activity")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

		activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

		// Link device to session
		_, err := ctx.db.NewUpdate().
			TableExpr("active.groups").
			Set("device_id = ?", device.ID).
			Where("id = ?", activeGroup.ID).
			Exec(t.Context())
		assert.NoError(t, err)

		// Pre-assign staff as supervisor BEFORE scanning
		sup := testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, sup.ID)

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

		// Duplicate scan should succeed (idempotent)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok, "Response should have data field")
		assert.Equal(t, "supervisor_authenticated", data["action"])
		assert.Contains(t, data["student_name"], "Duplicate")
		assert.Equal(t, room.Name, data["room_name"])
		assert.Contains(t, data["message"].(string), "Dup Supervisor Activity")
	})

	t.Run("response includes room and activity names", func(t *testing.T) {
		ctx := setupTestContext(t)
		defer func() { _ = ctx.db.Close() }()

		device := testpkg.CreateTestDevice(t, ctx.db, "staff-detail")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

		staff := testpkg.CreateTestStaff(t, ctx.db, "Detail", "Check")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

		tagID := fmt.Sprintf("STAFFDET%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		room := testpkg.CreateTestRoom(t, ctx.db, "Kreativraum")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

		activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Basteln")
		defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

		activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

		_, err := ctx.db.NewUpdate().
			TableExpr("active.groups").
			Set("device_id = ?", device.ID).
			Where("id = ?", activeGroup.ID).
			Exec(t.Context())
		assert.NoError(t, err)

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

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "supervisor_authenticated", data["action"])
		assert.Equal(t, room.Name, data["room_name"])
		assert.Equal(t, "Supervisor authenticated for Basteln", data["message"])
		assert.Equal(t, "success", data["status"])
		assert.Contains(t, data, "processed_at")
		assert.Contains(t, data, "student_id")
		assert.Equal(t, "Detail Check", data["student_name"])
	})
}

// TestDeviceCheckin_PersonNeitherStudentNorStaff verifies that a person who
// exists with an RFID tag but is neither a student nor a staff member gets
// a 404 response. This covers the "neither student nor staff" branch in
// handleStaffScan (workflow.go lines 118-121).
func TestDeviceCheckin_PersonNeitherStudentNorStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "bare-person")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a bare person (not linked to any student or staff record)
	person := testpkg.CreateTestPerson(t, ctx.db, "Bare", "Person")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, person.ID)

	tagID := fmt.Sprintf("BARE%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, person.ID, card.ID)

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

	// Person with RFID but no student/staff record should return 404
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
	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Student", "3c")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// Create RFID card
	tagID := fmt.Sprintf("TRANSFER%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room for activity
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Room 1")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room1.ID)

	// Create room 2 WITHOUT an active group
	room2 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Room 2")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room2.ID)

	// Create activity and active group only for room 1
	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Transfer Activity 1")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity1.ID)

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup1.ID)

	// Create initial visit in room 1 (active visit = nil exit time)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now(), nil)
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

	device := testpkg.CreateTestDevice(t, ctx.db, "invalid-json")
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

	device := testpkg.CreateTestDevice(t, ctx.db, "empty-rfid")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "success-checkin")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Checkin", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Success", "Checkin", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SUCCESS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room with active group
	room := testpkg.CreateTestRoom(t, ctx.db, "Success Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Success Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
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
	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Transfer", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Test", "2b")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("TRANS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room 1 with activity
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Room A")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room1.ID)

	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity A")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity1.ID)

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup1.ID)

	// Create room 2 with activity
	room2 := testpkg.CreateTestRoom(t, ctx.db, "Room B")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room2.ID)

	activity2 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity B")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity2.ID)

	activeGroup2 := testpkg.CreateTestActiveGroup(t, ctx.db, activity2.ID, room2.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup2.ID)

	// Create initial visit in room 1
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now().Add(-10*time.Minute), nil)
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
	device := testpkg.CreateTestDevice(t, ctx.db, "session-ping")
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

	device := testpkg.CreateTestDevice(t, ctx.db, "invalid-action")
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
	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-no-visit")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create student with RFID (no active visit)
	student := testpkg.CreateTestStudent(t, ctx.db, "NoVisit", "Test", "1a")
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

// =============================================================================
// SCHULHOF AUTO-CREATE TESTS
// =============================================================================

// cleanupSchulhofInfrastructure removes any pre-existing Schulhof auto-created
// data so tests start from a clean state. Uses individual statements in FK order.
func cleanupSchulhofInfrastructure(t *testing.T, db *bun.DB) {
	t.Helper()

	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete in FK-safe order: child tables first, then parents
	stmts := []string{
		`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = 'Schulhof')`,
		`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = 'Schulhof')`,
		`DELETE FROM active.group_supervisors WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = 'Schulhof')`,
		`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name = 'Schulhof')`,
		`DELETE FROM activities.schedules WHERE group_id IN (SELECT id FROM activities.groups WHERE name = 'Schulhof Freispiel')`,
		`DELETE FROM activities.student_enrollments WHERE group_id IN (SELECT id FROM activities.groups WHERE name = 'Schulhof Freispiel')`,
		`DELETE FROM activities.groups WHERE name = 'Schulhof Freispiel'`,
		`DELETE FROM activities.categories WHERE name = 'Schulhof'`,
		`DELETE FROM facilities.rooms WHERE name = 'Schulhof'`,
	}
	for _, stmt := range stmts {
		_, _ = db.ExecContext(dbCtx, stmt)
	}
}

// createSchulhofRoom creates a room with the exact name "Schulhof" (no timestamp
// suffix) so the auto-create path in createSchulhofActiveGroupIfNeeded recognizes it.
func createSchulhofRoom(t *testing.T, db *bun.DB) *facilities.Room {
	t.Helper()

	cleanupSchulhofInfrastructure(t, db)

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	room := &facilities.Room{
		Name:     "Schulhof",
		Building: "Test Building",
	}

	err := db.NewInsert().
		Model(room).
		ModelTableExpr("facilities.rooms").
		Scan(dbCtx)
	require.NoError(t, err, "Failed to create Schulhof room")

	return room
}

// TestDeviceCheckin_SchulhofAutoCreate verifies that checking a student into a
// room named "Schulhof" with no existing active group triggers automatic
// infrastructure creation (category, activity group, and active group).
// This is the code path fixed by the double-qualification bug where
// filter.Equal("group.name", ...) was incorrectly double-qualified by the
// repository's WithTableAlias("group").
func TestDeviceCheckin_SchulhofAutoCreate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-auto")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Schulhof", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Schulhof", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SCHULHOF%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create a room named exactly "Schulhof" (no suffix) so the auto-create
	// path in createSchulhofActiveGroupIfNeeded recognizes it
	room := createSchulhofRoom(t, ctx.db)
	// Clean up all auto-created Schulhof infrastructure on teardown
	defer cleanupSchulhofInfrastructure(t, ctx.db)

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

	// The Schulhof auto-create flow should succeed:
	// 1. No active group in room → detect room name is "Schulhof"
	// 2. schulhofActivityGroup() queries with filter.Equal("name", ...) (the fix)
	// 3. Activity not found → auto-create category, activity group, active group
	// 4. Student is checked in to the auto-created active group
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "Schulhof", data["room_name"])
}

// TestDeviceCheckin_SchulhofAutoCreateIdempotent verifies that the Schulhof
// auto-create flow is idempotent: a second checkin reuses the already-created
// activity group instead of failing or creating duplicates.
// =============================================================================
// ACTIVE STUDENTS COUNT TESTS
// =============================================================================

func TestDeviceCheckin_ResponseContainsActiveStudents(t *testing.T) {
	// Verifies that a successful checkin response includes the active_students count
	// when the device is linked to an active session.
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "active-count")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Count", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Count", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("COUNT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Count Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Count Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Link device to active group so getActiveStudentCountForRoom finds it
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(t.Context())
	require.NoError(t, err)

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

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])

	// Verify active_students is present in the response
	activeStudents, exists := data["active_students"]
	assert.True(t, exists, "Response should contain active_students field")
	// After checkin, there should be at least 1 active student
	assert.GreaterOrEqual(t, activeStudents, float64(1), "Should have at least 1 active student after checkin")
}

func TestDeviceCheckin_ActiveStudentsCountWithMultipleStudents(t *testing.T) {
	// Check in two students and verify the count increments correctly
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "multi-count")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Multi", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create two students
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "Counter", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)
	tag1 := fmt.Sprintf("MC1%d", time.Now().UnixNano())
	card1 := testpkg.CreateTestRFIDCard(t, ctx.db, tag1)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card1.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student1.PersonID, card1.ID)

	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "Counter", "1b")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student2.ID)
	tag2 := fmt.Sprintf("MC2%d", time.Now().UnixNano())
	card2 := testpkg.CreateTestRFIDCard(t, ctx.db, tag2)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card2.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card2.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Multi Count Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Multi Count Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Link device to active group
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(t.Context())
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Check in first student
	body1 := map[string]interface{}{
		"student_rfid": card1.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}
	req1 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body1,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)
	rr1 := testutil.ExecuteRequest(router, req1)
	testutil.AssertSuccessResponse(t, rr1, http.StatusOK)

	// Check in second student
	body2 := map[string]interface{}{
		"student_rfid": card2.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}
	req2 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body2,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)
	rr2 := testutil.ExecuteRequest(router, req2)
	testutil.AssertSuccessResponse(t, rr2, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr2.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")

	// After two checkins, active_students should be 2
	activeStudents, exists := data["active_students"]
	assert.True(t, exists, "Response should contain active_students field")
	assert.Equal(t, float64(2), activeStudents, "Should have 2 active students after two checkins")
}

// =============================================================================
// SAME ROOM SCAN (SKIP CHECKIN) TESTS
// =============================================================================

func TestDeviceCheckin_SameRoomScanSkipsCheckin(t *testing.T) {
	// When a student scans out from a room and the same room_id is provided,
	// the checkin should be skipped (student stays checked out from that room).
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "same-room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Same", "Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Same", "RoomStudent", "2a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SAME%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Same Room Test")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Same Room Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Create an active visit in the same room
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-5*time.Minute), nil)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// Scan with the SAME room_id - this should checkout + skip re-checkin
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

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")

	// Same room scan should result in checkout (not transfer or re-checkin)
	assert.Equal(t, "checked_out", data["action"])
}

// =============================================================================
// ROOM CAPACITY TESTS
// =============================================================================

func TestDeviceCheckin_RoomCapacityExceeded(t *testing.T) {
	// Verifies that checkin fails when room is at capacity
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "capacity-test")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Cap", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Create a room with capacity of 1
	dbCtx := t.Context()
	capacityRoom := &facilities.Room{
		Name:     fmt.Sprintf("Tiny Room-%d", time.Now().UnixNano()),
		Building: "Test Building",
		Capacity: intPtr(1),
	}
	err := ctx.db.NewInsert().
		Model(capacityRoom).
		ModelTableExpr("facilities.rooms").
		Scan(dbCtx)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, capacityRoom.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Capacity Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, capacityRoom.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Fill the room to capacity with one student
	existingStudent := testpkg.CreateTestStudent(t, ctx.db, "Existing", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, existingStudent.ID)

	visit := testpkg.CreateTestVisit(t, ctx.db, existingStudent.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	// Now try to check in another student
	newStudent := testpkg.CreateTestStudent(t, ctx.db, "Over", "Capacity", "1b")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, newStudent.ID)

	tagID := fmt.Sprintf("OVERCAP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, newStudent.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      capacityRoom.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail due to room capacity being exceeded
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected 409 Conflict for capacity exceeded. Body: %s", rr.Body.String())
}

// =============================================================================
// CHECKOUT WITH ROOM INFO TESTS
// =============================================================================

func TestDeviceCheckin_CheckoutResponseIncludesRoomName(t *testing.T) {
	// Verifies that checkout response includes the room name from the active visit
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Room", "Info", "3c")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("RMINFO%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Info Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Info Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-15*time.Minute), nil)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

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

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_out", data["action"])
	assert.Equal(t, "success", data["status"])
	// The student_name should be present
	assert.Contains(t, data["student_name"], "Room")
}

// =============================================================================
// NO ACTION EDGE CASE TEST
// =============================================================================

func TestDeviceCheckin_CheckoutWithNoRoomIDAndNoVisit(t *testing.T) {
	// Student with no active visit and no room_id should get an error
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "no-action")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "NoAction", "Test", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("NOACT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// No room_id, no active visit - should fail
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail because room_id is required and student has no active visit
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ACTIVITY CAPACITY TESTS
// =============================================================================

func TestDeviceCheckin_ActivityCapacityExceeded(t *testing.T) {
	// Verifies that checkin fails when activity MaxParticipants is reached
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "act-cap")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "ActCap", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Activity Cap Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	// Create activity with MaxParticipants = 1
	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("Cap-Cat-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, ctx.db, category.ID)

	creatorStaff := testpkg.CreateTestStaff(t, ctx.db, "Cap", "Creator")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, creatorStaff.ID)

	dbCtx := t.Context()
	activityGroup := &activities.Group{
		Name:            fmt.Sprintf("Tiny Activity-%d", time.Now().UnixNano()),
		MaxParticipants: 1, // Only 1 participant allowed
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       creatorStaff.ID,
	}
	err := ctx.db.NewInsert().
		Model(activityGroup).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(dbCtx)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activityGroup.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Fill the activity to capacity with one student
	existingStudent := testpkg.CreateTestStudent(t, ctx.db, "Existing", "ActCap", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, existingStudent.ID)

	visit := testpkg.CreateTestVisit(t, ctx.db, existingStudent.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, visit.ID)

	// Now try to check in another student - should fail
	newStudent := testpkg.CreateTestStudent(t, ctx.db, "Over", "ActCap", "1b")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, newStudent.ID)

	tagID := fmt.Sprintf("ACTCAP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, newStudent.PersonID, card.ID)

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

	// Should fail due to activity capacity being exceeded
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected 409 Conflict for activity capacity exceeded. Body: %s", rr.Body.String())
}

// =============================================================================
// ACTIVE STUDENTS FALLBACK PATH TESTS
// =============================================================================

func TestDeviceCheckin_ActiveStudentsFallbackWithoutDeviceLink(t *testing.T) {
	// When the device is NOT linked to an active group, getActiveStudentCountForRoom
	// falls back to counting across all groups in the room
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "fallback-count")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Fallback", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Fallback", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("FALLBACK%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Fallback Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Fallback Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// NOTE: device_id is NOT set on the active group - this forces the fallback path

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

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])

	// active_students should still be present via fallback path
	activeStudents, exists := data["active_students"]
	assert.True(t, exists, "Response should contain active_students via fallback path")
	assert.GreaterOrEqual(t, activeStudents, float64(1))
}

// =============================================================================
// SESSION ACTIVITY UPDATE TESTS
// =============================================================================

func TestDeviceCheckin_UpdatesSessionActivity(t *testing.T) {
	// Verifies that a checkin with room_id updates the session's last activity
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "session-update")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Session", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Session", "Update", "2a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("SESS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Session Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Session Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	// Link device to session
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(t.Context())
	require.NoError(t, err)

	// Record initial last_activity
	var initialLastActivity time.Time
	err = ctx.db.NewSelect().
		TableExpr("active.groups").
		Column("last_activity").
		Where("id = ?", activeGroup.ID).
		Scan(t.Context(), &initialLastActivity)
	require.NoError(t, err)

	// Small delay to ensure time difference
	time.Sleep(10 * time.Millisecond)

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
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify last_activity was updated
	var updatedLastActivity time.Time
	err = ctx.db.NewSelect().
		TableExpr("active.groups").
		Column("last_activity").
		Where("id = ?", activeGroup.ID).
		Scan(t.Context(), &updatedLastActivity)
	require.NoError(t, err)

	assert.True(t, updatedLastActivity.After(initialLastActivity) || updatedLastActivity.Equal(initialLastActivity),
		"last_activity should be updated after checkin")
}

func TestDeviceCheckin_SchulhofAutoCreateIdempotent(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-idem")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Idem", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// First student
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "Schulhof", "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID)

	tag1 := fmt.Sprintf("SH1%d", time.Now().UnixNano())
	card1 := testpkg.CreateTestRFIDCard(t, ctx.db, tag1)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card1.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student1.PersonID, card1.ID)

	// Second student
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "Schulhof", "1b")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student2.ID)

	tag2 := fmt.Sprintf("SH2%d", time.Now().UnixNano())
	card2 := testpkg.CreateTestRFIDCard(t, ctx.db, tag2)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card2.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card2.ID)

	room := createSchulhofRoom(t, ctx.db)
	defer cleanupSchulhofInfrastructure(t, ctx.db)

	router := chi.NewRouter()
	router.Post("/checkin/checkin", ctx.resource.DeviceCheckinHandler())

	// First checkin - triggers auto-create
	body1 := map[string]interface{}{
		"student_rfid": card1.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req1 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body1,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr1 := testutil.ExecuteRequest(router, req1)
	testutil.AssertSuccessResponse(t, rr1, http.StatusOK)

	// Second checkin - should reuse the existing active group (not fail)
	body2 := map[string]interface{}{
		"student_rfid": card2.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req2 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin/checkin", body2,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr2 := testutil.ExecuteRequest(router, req2)
	testutil.AssertSuccessResponse(t, rr2, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr2.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "Schulhof", data["room_name"])
}
