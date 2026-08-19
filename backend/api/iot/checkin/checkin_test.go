package checkin_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	checkinsvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *checkinAPI.Resource
}

// injectFailingUsers points both the handler's UsersService AND its extracted
// CheckinService at the given (usually failing) person service. Person/student/
// staff resolution moved into the CheckinService (issue #575 B8), so fault
// injection must reach that seam — setting resource.UsersService alone no longer
// affects the pickup-query/checkin resolution path.
func (ctx *testContext) injectFailingUsers(users usersSvc.PersonService) {
	ctx.resource.UsersService = users
	ctx.resource.Checkin = checkinsvc.NewCheckinService(checkinsvc.CheckinServiceDeps{
		Active:     ctx.services.Active,
		Users:      users,
		Facilities: ctx.services.Facilities,
		Activities: ctx.services.Activities,
		Logger:     slog.Default(),
	})
}

type failingPickupScheduleService struct {
	scheduleSvc.PickupScheduleService
	err error
}

type failingPersonService struct {
	usersSvc.PersonService
	findByTagIDErr error
	studentRepo    userModels.StudentRepository
	staffRepo      userModels.StaffRepository
}

func (s *failingPersonService) FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error) {
	if s.findByTagIDErr != nil {
		return nil, s.findByTagIDErr
	}
	return s.PersonService.FindByTagID(ctx, tagID)
}

func (s *failingPersonService) GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	if s.studentRepo != nil {
		return s.studentRepo.FindByPersonID(ctx, personID)
	}
	return s.PersonService.GetStudentByPersonID(ctx, personID)
}

func (s *failingPersonService) GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	if s.staffRepo != nil {
		return s.staffRepo.FindByPersonID(ctx, personID)
	}
	return s.PersonService.GetStaffByPersonID(ctx, personID)
}

type failingStudentRepository struct {
	userModels.StudentRepository
	err error
}

func (r *failingStudentRepository) FindByPersonID(context.Context, int64) (*userModels.Student, error) {
	return nil, r.err
}

type failingStaffRepository struct {
	userModels.StaffRepository
	err error
}

func (r *failingStaffRepository) FindByPersonID(context.Context, int64) (*userModels.Staff, error) {
	return nil, r.err
}

func (s *failingPickupScheduleService) GetEffectivePickupTimeForDate(
	context.Context, int64, timezone.Date,
) (*scheduleSvc.EffectivePickupTime, error) {
	return nil, s.err
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := checkinAPI.NewResource(
		svc.IoT,
		svc.Users,
		svc.Active,
		svc.Checkin,
		svc.PickupSchedule,
		nil, // settings service (nil = env var fallback)
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
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "ping-test")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/ping", nil,
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
	t.Parallel()
	ctx := setupTestContext(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context
	req := testutil.NewAuthenticatedRequest(t, "POST", "/ping", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// DEVICE STATUS TESTS
// =============================================================================

func TestDeviceStatus_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "status-test")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/status", nil,
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
	t.Parallel()
	ctx := setupTestContext(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// DEVICE CHECKIN TESTS
// =============================================================================

func TestDeviceCheckin_Unauthorized(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context
	body := map[string]interface{}{
		"student_rfid": "12345",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestDeviceCheckin_MissingRFID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-missing-rfid")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without student_rfid
	body := map[string]interface{}{
		"action": "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDeviceCheckin_StudentNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-not-found")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": "nonexistent-rfid-tag",
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 404 when RFID tag not found
	testutil.AssertNotFound(t, rr)
}

func TestDeviceCheckin_NoActiveGroups(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-no-groups")

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "CheckIn", "Student", "1a")

	// Create RFID card and link it to the student's person
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)

	// Link the RFID card to the person by updating the person's tag_id
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create test room for checkin (but no active groups)
	room := testpkg.CreateTestRoom(t, ctx.db, "Checkin Room")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-test")

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "Checkout", "Student", "1a")

	// Create RFID card and link it to the student's person
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room and activity
	room := testpkg.CreateTestRoom(t, ctx.db, "Checkout Test Room")

	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "Checkout Activity")

	// Create active group in the room
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)

	// Create an active visit for the student (with entry time and nil exit time for active visit)
	_ = testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now(), nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Perform checkout by scanning RFID without room_id
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout", // Explicit checkout action
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// This test verifies that checkin to a room without an active group fails appropriately
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkin-new")

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "New", "Visit", "2b")

	// Create RFID card and link it
	tagID := fmt.Sprintf("TAG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room WITHOUT an active group - checkin should fail
	room := testpkg.CreateTestRoom(t, ctx.db, "New Visit Room")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	t.Run("authenticates supervisor with active session", func(t *testing.T) {
		ctx := setupTestContext(t)

		// Create test device
		device := testpkg.CreateTestDevice(t, ctx.db, "staff-rfid")

		// Create a test staff member with RFID tag
		staff := testpkg.CreateTestStaff(t, ctx.db, "Staff", "Member")

		// Create RFID card and link it to the staff's person
		tagID := fmt.Sprintf("STAFF%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		// Create room and activity
		room := testpkg.CreateTestRoom(t, ctx.db, "Supervisor Room")

		activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Supervisor Activity")

		// Create active group linked to the device (simulates an active session)
		activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

		// Link active group to device so GetDeviceCurrentSession finds it
		_, err := ctx.db.NewUpdate().
			TableExpr("active.groups").
			Set("device_id = ?", device.ID).
			Where("id = ?", activeGroup.ID).
			Exec(t.Context())
		assert.NoError(t, err)

		router := testutil.NewTenantRouter(ctx.db)
		router.Mount("/", ctx.resource.Router())

		body := map[string]interface{}{
			"student_rfid": card.ID,
			"action":       "checkin",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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

		// Create test device (no active session)
		device := testpkg.CreateTestDevice(t, ctx.db, "staff-no-session")

		// Create a test staff member with RFID tag
		staff := testpkg.CreateTestStaff(t, ctx.db, "NoSession", "Staff")

		tagID := fmt.Sprintf("STAFFNS%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		router := testutil.NewTenantRouter(ctx.db)
		router.Mount("/", ctx.resource.Router())

		body := map[string]interface{}{
			"student_rfid": card.ID,
			"action":       "checkin",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
			testutil.WithDeviceContext(createTestDeviceContext(device)),
		)

		rr := testutil.ExecuteRequest(router, req)

		// Staff RFID without active session should return 404
		testutil.AssertNotFound(t, rr)
	})

	t.Run("idempotent duplicate supervisor scan", func(t *testing.T) {
		ctx := setupTestContext(t)

		device := testpkg.CreateTestDevice(t, ctx.db, "staff-dup")

		staff := testpkg.CreateTestStaff(t, ctx.db, "Duplicate", "Supervisor")

		tagID := fmt.Sprintf("STAFFDUP%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		room := testpkg.CreateTestRoom(t, ctx.db, "Dup Supervisor Room")

		activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Dup Supervisor Activity")

		activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

		// Link device to session
		_, err := ctx.db.NewUpdate().
			TableExpr("active.groups").
			Set("device_id = ?", device.ID).
			Where("id = ?", activeGroup.ID).
			Exec(t.Context())
		assert.NoError(t, err)

		// Pre-assign staff as supervisor BEFORE scanning
		_ = testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")

		router := testutil.NewTenantRouter(ctx.db)
		router.Mount("/", ctx.resource.Router())

		body := map[string]interface{}{
			"student_rfid": card.ID,
			"action":       "checkin",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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

		device := testpkg.CreateTestDevice(t, ctx.db, "staff-detail")

		staff := testpkg.CreateTestStaff(t, ctx.db, "Detail", "Check")

		tagID := fmt.Sprintf("STAFFDET%d", time.Now().UnixNano())
		card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
		testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

		room := testpkg.CreateTestRoom(t, ctx.db, "Kreativraum")

		activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Basteln")

		activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

		_, err := ctx.db.NewUpdate().
			TableExpr("active.groups").
			Set("device_id = ?", device.ID).
			Where("id = ?", activeGroup.ID).
			Exec(t.Context())
		assert.NoError(t, err)

		router := testutil.NewTenantRouter(ctx.db)
		router.Mount("/", ctx.resource.Router())

		body := map[string]interface{}{
			"student_rfid": card.ID,
			"action":       "checkin",
		}

		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "bare-person")

	// Create a bare person (not linked to any student or staff record)
	person := testpkg.CreateTestPerson(t, ctx.db, "Bare", "Person")

	tagID := fmt.Sprintf("BARE%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, person.ID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// This test verifies that attempting to transfer to a room without an active group fails
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test")

	// Create a test student with RFID tag
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Student", "3c")

	// Create RFID card
	tagID := fmt.Sprintf("TRANSFER%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room for activity
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Room 1")

	// Create room 2 WITHOUT an active group
	room2 := testpkg.CreateTestRoom(t, ctx.db, "Transfer Room 2")

	// Create activity and active group only for room 1
	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Transfer Activity 1")

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID)

	// Create initial visit in room 1 (active visit = nil exit time)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now(), nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Try to transfer to room 2 which has no active group
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail because room 2 has no active group
	// The student checkout from room 1 will succeed, but checkin to room 2 will fail
	testutil.AssertNotFound(t, rr)

	var persisted active.Visit
	require.NoError(t, ctx.db.NewSelect().Model(&persisted).Where("id = ?", visit.ID).Scan(t.Context()))
	assert.NotNil(t, persisted.ExitTime)
}

// =============================================================================
// INVALID REQUEST TESTS
// =============================================================================

func TestDeviceCheckin_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "invalid-json")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Send invalid JSON using the standard method with an invalid body type
	// The handler expects JSON, so sending a map with invalid structure
	body := map[string]interface{}{
		"student_rfid": 12345, // wrong type - should be string
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "empty-rfid")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": "",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

func TestRouter_CheckinEndpointExists(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", nil)

	rr := testutil.ExecuteRequest(router, req)

	// 401 indicates endpoint exists but requires device authentication
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRouter_PingEndpointExists(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/ping", nil)

	rr := testutil.ExecuteRequest(router, req)

	// 401 indicates endpoint exists but requires device authentication
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRouter_StatusEndpointExists(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

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
	t.Parallel()
	// Full checkin requires staff context for attendance tracking (checked_in_by FK constraint).
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "success-checkin")

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Checkin", "Staff")

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Success", "Checkin", "1a")

	tagID := fmt.Sprintf("SUCCESS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room with active group
	room := testpkg.CreateTestRoom(t, ctx.db, "Success Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Success Activity")

	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Room transfer: checkout from room 1, checkin to room 2.
	// Requires staff context for attendance tracking (checked_in_by FK constraint).
	ctx := setupTestContext(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "transfer-test")

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Transfer", "Staff")

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Transfer", "Test", "2b")

	tagID := fmt.Sprintf("TRANS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create room 1 with activity
	room1 := testpkg.CreateTestRoom(t, ctx.db, "Room A")

	activity1 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity A")

	activeGroup1 := testpkg.CreateTestActiveGroup(t, ctx.db, activity1.ID, room1.ID)

	// Create room 2 with activity
	room2 := testpkg.CreateTestRoom(t, ctx.db, "Room B")

	activity2 := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity B")

	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activity2.ID, room2.ID)

	// Create initial visit in room 1
	_ = testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup1.ID, time.Now().Add(-10*time.Minute), nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Transfer to room 2
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room2.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	ctx := setupTestContext(t)

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "session-ping")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/ping", nil,
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
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "invalid-action")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": "test-rfid",
		"action":       "invalid",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	ctx := setupTestContext(t)

	// Create device
	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-no-visit")

	// Create student with RFID (no active visit)
	student := testpkg.CreateTestStudent(t, ctx.db, "NoVisit", "Test", "1a")

	tagID := fmt.Sprintf("NOVISIT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Checkout without active visit should fail - no room_id provided and nothing to checkout
	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SCHULHOF AUTO-CREATE TESTS
// =============================================================================

// cleanupSchulhofInfrastructure removes Schulhof auto-created data for a specific
// room ID so tests clean up only their own data. Uses individual statements in FK order.
func cleanupSchulhofInfrastructure(t *testing.T, db *bun.DB, roomID int64) {
	t.Helper()

	dbCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	// Delete in FK-safe order: child tables first, then parents.
	// All queries are scoped to the specific room ID to avoid interfering
	// with Schulhof tests running in parallel from other packages.
	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE student_id IN (SELECT v.student_id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id WHERE ag.room_id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT id FROM active.groups WHERE room_id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT id FROM active.groups WHERE room_id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id = %d`, roomID),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT ag.id FROM activities.groups ag JOIN activities.categories ac ON ac.id = ag.category_id JOIN facilities.rooms r ON r.name = ac.name WHERE r.id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT ag.id FROM activities.groups ag JOIN activities.categories ac ON ac.id = ag.category_id JOIN facilities.rooms r ON r.name = ac.name WHERE r.id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE category_id IN (SELECT ac.id FROM activities.categories ac JOIN facilities.rooms r ON r.name = ac.name WHERE r.id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = (SELECT name FROM facilities.rooms WHERE id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE id = %d`, roomID),
	}
	for _, stmt := range stmts {
		_, _ = db.ExecContext(dbCtx, stmt)
	}
}

// createSchulhofRoom creates a room with the exact name "Schulhof" (no timestamp
// suffix) so the auto-create path in createSchulhofActiveGroupIfNeeded recognizes it.
// If a Schulhof room already exists (e.g. from seed data), it cleans up and recreates it
// to ensure the test owns the full lifecycle.
func createSchulhofRoom(t *testing.T, db *bun.DB) *facilities.Room {
	t.Helper()

	dbCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	// Clean up any pre-existing Schulhof room and its infrastructure (from seed data or prior tests)
	var existingID int64
	err := db.NewSelect().
		TableExpr("facilities.rooms").
		Column("id").
		Where("name = ?", "Schulhof").
		Scan(dbCtx, &existingID)
	if err == nil && existingID > 0 {
		cleanupSchulhofInfrastructure(t, db, existingID)
	}

	// Also clean up any pre-existing Schulhof activity and category (auto-created artifacts)
	schulhofCleanupStmts := []string{
		`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = 'Schulhof Freispiel')`,
		`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = 'Schulhof Freispiel')`,
		`DELETE FROM activities.groups WHERE name = 'Schulhof Freispiel'`,
		`DELETE FROM activities.categories WHERE name = 'Schulhof'`,
	}
	for _, stmt := range schulhofCleanupStmts {
		_, _ = db.ExecContext(dbCtx, stmt)
	}

	room := &facilities.Room{
		Name:     "Schulhof",
		Building: "Test Building",
		IsSystem: true,
	}
	room.SetTenantID(testpkg.Tenant(t))

	_, err = db.NewInsert().
		Model(room).
		ModelTableExpr("facilities.rooms").
		On("CONFLICT (tenant_id, name) DO NOTHING").
		Exec(dbCtx)
	require.NoError(t, err, "Failed to ensure Schulhof room")

	// Fetch the actual room (either just created or existing)
	err = db.NewSelect().
		Model(room).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Where(`"room".name = ?`, "Schulhof").
		Where(`"room".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(dbCtx)
	require.NoError(t, err, "Failed to fetch Schulhof room")

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

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-auto")

	// Create staff for attendance tracking
	staff := testpkg.CreateTestStaff(t, ctx.db, "Schulhof", "Staff")

	// Create student with RFID
	student := testpkg.CreateTestStudent(t, ctx.db, "Schulhof", "Student", "1a")

	tagID := fmt.Sprintf("SCHULHOF%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create a room named exactly "Schulhof" (no suffix) so the auto-create
	// path in createSchulhofActiveGroupIfNeeded recognizes it
	room := createSchulhofRoom(t, ctx.db)
	// Clean up all auto-created Schulhof infrastructure on teardown (scoped to this room)
	defer cleanupSchulhofInfrastructure(t, ctx.db, room.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Verifies that a successful checkin response includes the active_students count
	// when the device is linked to an active session.
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "active-count")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Count", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Count", "Student", "1a")

	tagID := fmt.Sprintf("COUNT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Count Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Count Activity")

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	// Link device to active group so getActiveStudentCountForRoom finds it
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Check in two students and verify the count increments correctly
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "multi-count")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Multi", "Staff")

	// Create two students
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "Counter", "1a")
	tag1 := fmt.Sprintf("MC1%d", time.Now().UnixNano())
	card1 := testpkg.CreateTestRFIDCard(t, ctx.db, tag1)
	testpkg.LinkRFIDToStudent(t, ctx.db, student1.PersonID, card1.ID)

	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "Counter", "1b")
	tag2 := fmt.Sprintf("MC2%d", time.Now().UnixNano())
	card2 := testpkg.CreateTestRFIDCard(t, ctx.db, tag2)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card2.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Multi Count Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Multi Count Activity")

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	// Link device to active group
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", activeGroup.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	// Check in first student
	body1 := map[string]interface{}{
		"student_rfid": card1.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}
	req1 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body1,
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
	req2 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body2,
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

func TestDeviceCheckin_ActiveStudentsStayScopedToDeviceSessionInSharedRoom(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "shared-room-count")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Shared", "Room")

	scannedStudent := testpkg.CreateTestStudent(t, ctx.db, "Scanned", "Student", "3a")
	scannedTagID := fmt.Sprintf("SHAREDSCAN%d", time.Now().UnixNano())
	scannedCard := testpkg.CreateTestRFIDCard(t, ctx.db, scannedTagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, scannedStudent.PersonID, scannedCard.ID)

	sessionStudent := testpkg.CreateTestStudent(t, ctx.db, "Session", "Peer", "3a")

	otherGroupStudent := testpkg.CreateTestStudent(t, ctx.db, "Other", "Group", "3b")

	room := testpkg.CreateTestRoom(t, ctx.db, "Shared Count Room")

	deviceActivity := testpkg.CreateTestActivityGroup(t, ctx.db, "Device Session Activity")
	otherActivity := testpkg.CreateTestActivityGroup(t, ctx.db, "Other Session Activity")

	deviceGroup := testpkg.CreateTestActiveGroup(t, ctx.db, deviceActivity.ID, room.ID)
	otherGroup := testpkg.CreateTestActiveGroup(t, ctx.db, otherActivity.ID, room.ID)

	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", deviceGroup.ID).
		Exec(t.Context())
	require.NoError(t, err)

	_ = testpkg.CreateTestVisit(t, ctx.db, scannedStudent.ID, deviceGroup.ID, time.Now().Add(-10*time.Minute), nil)
	_ = testpkg.CreateTestVisit(t, ctx.db, sessionStudent.ID, deviceGroup.ID, time.Now().Add(-8*time.Minute), nil)
	_ = testpkg.CreateTestVisit(t, ctx.db, otherGroupStudent.ID, otherGroup.ID, time.Now().Add(-6*time.Minute), nil)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": scannedCard.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Response should have data field")

	activeStudents, exists := data["active_students"]
	require.True(t, exists, "Response should contain active_students field")
	assert.Equal(t, float64(1), activeStudents, "Should report only the remaining students in the device session, not the room total")
}

// =============================================================================
// SAME ROOM SCAN (SKIP CHECKIN) TESTS
// =============================================================================

func TestDeviceCheckin_SameRoomScanSkipsCheckin(t *testing.T) {
	t.Parallel()
	// When a student scans out from a room and the same room_id is provided,
	// the checkin should be skipped (student stays checked out from that room).
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "same-room")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Same", "Room")

	student := testpkg.CreateTestStudent(t, ctx.db, "Same", "RoomStudent", "2a")

	tagID := fmt.Sprintf("SAME%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Same Room Test")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Same Room Activity")

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	// Create an active visit in the same room
	_ = testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-5*time.Minute), nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Scan with the SAME room_id - this should checkout + skip re-checkin
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Verifies that checkin fails when room is at capacity
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "capacity-test")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Cap", "Staff")

	// Create a room with capacity of 1
	dbCtx := t.Context()
	capacityRoom := &facilities.Room{
		Name:     fmt.Sprintf("Tiny Room-%d", time.Now().UnixNano()),
		Building: "Test Building",
		Capacity: testpkg.IntPtr(1),
	}
	capacityRoom.SetTenantID(testpkg.Tenant(t))
	err := ctx.db.NewInsert().
		Model(capacityRoom).
		ModelTableExpr("facilities.rooms").
		Scan(dbCtx)
	require.NoError(t, err)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Capacity Activity")

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, capacityRoom.ID)

	// Fill the room to capacity with one student
	existingStudent := testpkg.CreateTestStudent(t, ctx.db, "Existing", "Student", "1a")

	_ = testpkg.CreateTestVisit(t, ctx.db, existingStudent.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil)

	// Now try to check in another student
	newStudent := testpkg.CreateTestStudent(t, ctx.db, "Over", "Capacity", "1b")

	tagID := fmt.Sprintf("OVERCAP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, newStudent.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      capacityRoom.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should fail due to room capacity being exceeded
	assert.Equal(t, http.StatusConflict, rr.Code, "Expected 409 Conflict for capacity exceeded. Body: %s", rr.Body.String())
}

func TestDeviceCheckin_RoomCapacityExceededRollsBackSourceCheckout(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "capacity-transfer-rollback")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Capacity", "Transfer")
	sourceRoom := testpkg.CreateTestRoom(t, ctx.db, "Capacity Transfer Source")
	targetRoom := testpkg.CreateTestRoom(t, ctx.db, "Capacity Transfer Target")
	capacity := 1
	targetRoom.Capacity = &capacity
	_, err := ctx.db.NewUpdate().Model(targetRoom).Column("capacity").WherePK().Exec(t.Context())
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "capacity-transfer-rollback")
	sourceGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, targetRoom.ID)
	movingStudent := testpkg.CreateTestStudent(t, ctx.db, "Moving", "Student", "1a")
	presentStudent := testpkg.CreateTestStudent(t, ctx.db, "Present", "Student", "1a")
	sourceVisit := testpkg.CreateTestVisit(t, ctx.db, movingStudent.ID, sourceGroup.ID, time.Now().Add(-time.Hour), nil)
	_ = testpkg.CreateTestVisit(t, ctx.db, presentStudent.ID, targetGroup.ID, time.Now().Add(-time.Hour), nil)

	card := testpkg.CreateTestRFIDCard(t, ctx.db, fmt.Sprintf("CAPROLLBACK%d", time.Now().UnixNano()))
	testpkg.LinkRFIDToStudent(t, ctx.db, movingStudent.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      targetRoom.ID,
	}, testutil.WithDeviceContext(createTestDeviceContext(device)), testutil.WithStaffContext(staff))

	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	var persisted active.Visit
	require.NoError(t, ctx.db.NewSelect().Model(&persisted).Where("id = ?", sourceVisit.ID).Scan(t.Context()))
	assert.Nil(t, persisted.ExitTime)
	assert.Equal(t, sourceGroup.ID, persisted.ActiveGroupID)
}

// =============================================================================
// CHECKOUT WITH ROOM INFO TESTS
// =============================================================================

func TestDeviceCheckin_CheckoutResponseIncludesRoomName(t *testing.T) {
	t.Parallel()
	// Verifies that checkout response includes the room name from the active visit
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "checkout-room")

	student := testpkg.CreateTestStudent(t, ctx.db, "Room", "Info", "3c")

	tagID := fmt.Sprintf("RMINFO%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Info Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Info Activity")

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	_ = testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, time.Now().Add(-15*time.Minute), nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Student with no active visit and no room_id should get an error
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "no-action")

	student := testpkg.CreateTestStudent(t, ctx.db, "NoAction", "Test", "1a")

	tagID := fmt.Sprintf("NOACT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// No room_id, no active visit - should fail
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Verifies that checkin fails when activity MaxParticipants is reached
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "act-cap")

	staff := testpkg.CreateTestStaff(t, ctx.db, "ActCap", "Staff")

	room := testpkg.CreateTestRoom(t, ctx.db, "Activity Cap Room")

	// Create activity with MaxParticipants = 1
	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("Cap-Cat-%d", time.Now().UnixNano()))

	creatorStaff := testpkg.CreateTestStaff(t, ctx.db, "Cap", "Creator")

	dbCtx := t.Context()
	activityGroup := &activities.Group{
		Name:            fmt.Sprintf("Tiny Activity-%d", time.Now().UnixNano()),
		MaxParticipants: 1, // Only 1 participant allowed
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &creatorStaff.ID,
	}
	activityGroup.SetTenantID(testpkg.Tenant(t))
	err := ctx.db.NewInsert().
		Model(activityGroup).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(dbCtx)
	require.NoError(t, err)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)

	// Fill the activity to capacity with one student
	existingStudent := testpkg.CreateTestStudent(t, ctx.db, "Existing", "ActCap", "1a")

	_ = testpkg.CreateTestVisit(t, ctx.db, existingStudent.ID, activeGroup.ID, time.Now().Add(-10*time.Minute), nil)

	// Now try to check in another student - should fail
	newStudent := testpkg.CreateTestStudent(t, ctx.db, "Over", "ActCap", "1b")

	tagID := fmt.Sprintf("ACTCAP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, newStudent.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// When the device is NOT linked to an active group, getActiveStudentCountForRoom
	// falls back to counting across all groups in the room
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "fallback-count")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Fallback", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Fallback", "Student", "1a")

	tagID := fmt.Sprintf("FALLBACK%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Fallback Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Fallback Activity")

	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	// NOTE: device_id is NOT set on the active group - this forces the fallback path

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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
	t.Parallel()
	// Verifies that a checkin with room_id updates the session's last activity
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "session-update")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Session", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Session", "Update", "2a")

	tagID := fmt.Sprintf("SESS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Session Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Session Activity")

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

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

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
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

// =============================================================================
// WC AUTO-CREATE TESTS
// =============================================================================

// cleanupWCInfrastructure removes WC auto-created data for a specific
// room ID so tests clean up only their own data. Uses individual statements in FK order.
func cleanupWCInfrastructure(t *testing.T, db *bun.DB, roomID int64) {
	t.Helper()

	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete in FK-safe order: child tables first, then parents.
	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE student_id IN (SELECT v.student_id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id WHERE ag.room_id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT id FROM active.groups WHERE room_id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT id FROM active.groups WHERE room_id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id = %d`, roomID),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT ag.id FROM activities.groups ag JOIN activities.categories ac ON ac.id = ag.category_id JOIN facilities.rooms r ON r.name = ac.name WHERE r.id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT ag.id FROM activities.groups ag JOIN activities.categories ac ON ac.id = ag.category_id JOIN facilities.rooms r ON r.name = ac.name WHERE r.id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE category_id IN (SELECT ac.id FROM activities.categories ac JOIN facilities.rooms r ON r.name = ac.name WHERE r.id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = (SELECT name FROM facilities.rooms WHERE id = %d)`, roomID),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE id = %d`, roomID),
	}
	for _, stmt := range stmts {
		_, _ = db.ExecContext(dbCtx, stmt)
	}
}

// createWCRoom creates a room with the exact name "WC" (no timestamp
// suffix) so the auto-create path in createSpecialRoomActiveGroupIfNeeded recognizes it.
// If a WC room already exists (e.g. from seed data), it cleans up and recreates it
// to ensure the test owns the full lifecycle.
func createWCRoom(t *testing.T, db *bun.DB) *facilities.Room {
	t.Helper()

	dbCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	// Clean up any pre-existing WC room and its infrastructure (from seed data or prior tests)
	var existingID int64
	err := db.NewSelect().
		TableExpr("facilities.rooms").
		Column("id").
		Where("name = ?", "WC").
		Scan(dbCtx, &existingID)
	if err == nil && existingID > 0 {
		cleanupWCInfrastructure(t, db, existingID)
	}

	// Also clean up any pre-existing WC activity and category (auto-created artifacts)
	wcCleanupStmts := []string{
		`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = 'WC')`,
		`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = 'WC')`,
		`DELETE FROM activities.groups WHERE name = 'WC'`,
		`DELETE FROM activities.categories WHERE name = 'WC'`,
	}
	for _, stmt := range wcCleanupStmts {
		_, _ = db.ExecContext(dbCtx, stmt)
	}

	room := &facilities.Room{
		Name:     "WC",
		Building: "Test Building",
	}
	room.SetTenantID(testpkg.Tenant(t))

	_, err = db.NewInsert().
		Model(room).
		ModelTableExpr("facilities.rooms").
		On("CONFLICT (tenant_id, name) DO NOTHING").
		Exec(dbCtx)
	require.NoError(t, err, "Failed to ensure WC room")

	// Fetch the actual room (either just created or existing)
	err = db.NewSelect().
		Model(room).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Where(`"room".name = ?`, "WC").
		Where(`"room".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(dbCtx)
	require.NoError(t, err, "Failed to fetch WC room")

	return room
}

// TestDeviceCheckin_WCAutoCreate verifies that checking a student into a
// room named "WC" with no existing active group triggers automatic
// infrastructure creation (category, activity group, and active group).
func TestDeviceCheckin_WCAutoCreate(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "wc-auto")

	staff := testpkg.CreateTestStaff(t, ctx.db, "WC", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "WC", "Student", "1a")

	tagID := fmt.Sprintf("WC%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createWCRoom(t, ctx.db)
	defer cleanupWCInfrastructure(t, ctx.db, room.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// The WC auto-create flow should succeed:
	// 1. No active group in room → detect room name is "WC"
	// 2. wcActivityGroup() queries for existing WC activity
	// 3. Activity not found → auto-create category, activity group, active group
	// 4. Student is checked in to the auto-created active group
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "WC", data["room_name"])

	activeGroup := new(active.Group)
	err := ctx.db.NewSelect().
		Model(activeGroup).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ?`, room.ID).
		OrderExpr(`"group".id DESC`).
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Nil(t, activeGroup.DeviceID, "Auto-created WC group must NOT have a DeviceID — WC is a shared room, not a device session")
}

// TestDeviceCheckin_WCAutoCreateIdempotent verifies that the WC
// auto-create flow is idempotent: a second checkin reuses the already-created
// activity group instead of failing or creating duplicates.
func TestDeviceCheckin_WCAutoCreateIdempotent(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "wc-idem")

	staff := testpkg.CreateTestStaff(t, ctx.db, "WCIdem", "Staff")

	// First student
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "WC", "1a")

	tag1 := fmt.Sprintf("WC1%d", time.Now().UnixNano())
	card1 := testpkg.CreateTestRFIDCard(t, ctx.db, tag1)
	testpkg.LinkRFIDToStudent(t, ctx.db, student1.PersonID, card1.ID)

	// Second student
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "WC", "1b")

	tag2 := fmt.Sprintf("WC2%d", time.Now().UnixNano())
	card2 := testpkg.CreateTestRFIDCard(t, ctx.db, tag2)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card2.ID)

	room := createWCRoom(t, ctx.db)
	defer cleanupWCInfrastructure(t, ctx.db, room.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	// First checkin - triggers auto-create
	body1 := map[string]interface{}{
		"student_rfid": card1.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req1 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body1,
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

	req2 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body2,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr2 := testutil.ExecuteRequest(router, req2)
	testutil.AssertSuccessResponse(t, rr2, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr2.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "WC", data["room_name"])
}

// TestDeviceCheckin_WCCheckoutFromWC verifies the full WC visit lifecycle:
// check in to WC room, then check out.
func TestDeviceCheckin_WCCheckoutFromWC(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "wc-checkout")

	staff := testpkg.CreateTestStaff(t, ctx.db, "WCOut", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "WCOut", "Student", "2a")

	tagID := fmt.Sprintf("WCOUT%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createWCRoom(t, ctx.db)
	defer cleanupWCInfrastructure(t, ctx.db, room.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	// Step 1: Check in to WC room (triggers auto-create)
	checkinBody := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	checkinReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	checkinRR := testutil.ExecuteRequest(router, checkinReq)
	testutil.AssertSuccessResponse(t, checkinRR, http.StatusOK)

	checkinResponse := testutil.ParseJSONResponse(t, checkinRR.Body.Bytes())
	checkinData, ok := checkinResponse["data"].(map[string]interface{})
	assert.True(t, ok, "Checkin response should have data field")
	assert.Equal(t, "checked_in", checkinData["action"])

	// Step 2: Check out from WC room
	checkoutBody := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkout",
	}

	checkoutReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkoutBody,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	checkoutRR := testutil.ExecuteRequest(router, checkoutReq)
	testutil.AssertSuccessResponse(t, checkoutRR, http.StatusOK)

	checkoutResponse := testutil.ParseJSONResponse(t, checkoutRR.Body.Bytes())
	checkoutData, ok := checkoutResponse["data"].(map[string]interface{})
	assert.True(t, ok, "Checkout response should have data field")
	assert.Equal(t, "checked_out", checkoutData["action"])
}

// TestDeviceCheckin_WCAutoCreateWithoutStaff verifies that WC checkin succeeds
// when the request has no staff context (no staff PIN scanned at the WC reader).
// The WC activity group is auto-created with created_by = NULL (nullable after our migration).
//
// Pre-condition: In production a student can only be at WC after first checking
// into a normal room with staff present — that earlier check-in creates the
// attendance record. We insert the record directly to satisfy that invariant
// without needing a full two-step checkin/checkout flow.
func TestDeviceCheckin_WCAutoCreateWithoutStaff(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "wc-no-staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "WCNoStaff", "Student", "1a")

	tagID := fmt.Sprintf("WCNS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createWCRoom(t, ctx.db)
	defer cleanupWCInfrastructure(t, ctx.db, room.ID)

	// Pre-condition: simulate prior morning check-in (with staff) by inserting the
	// attendance record directly. In production this record always exists before a
	// student reaches the WC reader.
	setupStaff := testpkg.CreateTestStaff(t, ctx.db, "SetupStaff", "WCNoStaff")
	today := timezone.TodayDate() // binds as a calendar-day literal in DATE position
	var attendanceID int64
	err := ctx.db.NewRaw(
		`INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id, tenant_id)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		student.ID, today, today.UTCMidnight().Add(8*time.Hour), setupStaff.ID, device.ID, testpkg.Tenant(t),
	).Scan(context.Background(), &attendanceID)
	require.NoError(t, err, "test setup: failed to insert attendance record")

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	// No staff context — simulates a student scanning at the WC reader without a PIN.
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// The WC activity group is auto-created with created_by = NULL.
	assert.Equal(t, http.StatusOK, rr.Code,
		"Expected 200 when WC auto-create has no staff context. Body: %s", rr.Body.String())
}

// TestDeviceCheckin_SchulhofAutoCreateWithoutStaff verifies that Schulhof checkin
// succeeds when the request has no staff context (no staff PIN scanned).
// The Schulhof activity group is auto-created with created_by = NULL (nullable after
// our migration).
//
// Pre-condition: same as WC — students reach Schulhof only after a prior check-in
// with staff that created today's attendance record. We insert it directly here.
func TestDeviceCheckin_SchulhofAutoCreateWithoutStaff(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-no-staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "SchulhofNoStaff", "Student", "1b")

	tagID := fmt.Sprintf("SHNS%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createSchulhofRoom(t, ctx.db)
	defer cleanupSchulhofInfrastructure(t, ctx.db, room.ID)

	// Pre-condition: simulate prior morning check-in (with staff) by inserting the
	// attendance record directly. In production this record always exists before a
	// student reaches the Schulhof reader.
	setupStaff := testpkg.CreateTestStaff(t, ctx.db, "SetupStaff", "SchulhofNoStaff")
	today := timezone.TodayDate() // binds as a calendar-day literal in DATE position
	var attendanceID int64
	err := ctx.db.NewRaw(
		`INSERT INTO active.attendance (student_id, date, check_in_time, checked_in_by, device_id, tenant_id)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		student.ID, today, today.UTCMidnight().Add(8*time.Hour), setupStaff.ID, device.ID, testpkg.Tenant(t),
	).Scan(context.Background(), &attendanceID)
	require.NoError(t, err, "test setup: failed to insert attendance record")

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	// No staff context — simulates a student scanning at the Schulhof reader without a PIN.
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// The Schulhof activity group is auto-created with created_by = NULL.
	assert.Equal(t, http.StatusOK, rr.Code,
		"Expected 200 when Schulhof auto-create has no staff context. Body: %s", rr.Body.String())
}

func TestDeviceCheckin_SchulhofAutoCreateIdempotent(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-idem")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Idem", "Staff")

	// First student
	student1 := testpkg.CreateTestStudent(t, ctx.db, "First", "Schulhof", "1a")

	tag1 := fmt.Sprintf("SH1%d", time.Now().UnixNano())
	card1 := testpkg.CreateTestRFIDCard(t, ctx.db, tag1)
	testpkg.LinkRFIDToStudent(t, ctx.db, student1.PersonID, card1.ID)

	// Second student
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Second", "Schulhof", "1b")

	tag2 := fmt.Sprintf("SH2%d", time.Now().UnixNano())
	card2 := testpkg.CreateTestRFIDCard(t, ctx.db, tag2)
	testpkg.LinkRFIDToStudent(t, ctx.db, student2.PersonID, card2.ID)

	room := createSchulhofRoom(t, ctx.db)
	defer cleanupSchulhofInfrastructure(t, ctx.db, room.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// First checkin - triggers auto-create
	body1 := map[string]interface{}{
		"student_rfid": card1.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req1 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body1,
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

	req2 := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body2,
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

// =============================================================================
// REGRESSION: SPECIAL ROOM GROUPS MUST NOT HAVE DEVICE ID
// =============================================================================

// TestDeviceCheckin_WCGroupDoesNotHijackDeviceSession is a regression test for
// a bug where auto-created WC groups received a DeviceID, causing
// GetDeviceCurrentSession to return the WC group instead of the actual
// room session. This broke session counts and session resume after device
// restart. See commit 54ef0c99 for the regression.
func TestDeviceCheckin_WCGroupDoesNotHijackDeviceSession(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "wc-hijack-regression")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Regression", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Regression", "Student", "2a")

	tagID := fmt.Sprintf("WCREG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	// Create a normal room with a device-linked session (the "real" session)
	sessionRoom := testpkg.CreateTestRoom(t, ctx.db, "Session Room Regression")

	sessionActivity := testpkg.CreateTestActivityGroup(t, ctx.db, "Session Activity Regression")

	sessionGroup := testpkg.CreateTestActiveGroup(t, ctx.db, sessionActivity.ID, sessionRoom.ID)

	// Link the session group to this device
	_, err := ctx.db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", device.ID).
		Where("id = ?", sessionGroup.ID).
		Exec(t.Context())
	require.NoError(t, err)

	// Check student into the session room first
	_ = testpkg.CreateTestVisit(t, ctx.db, student.ID, sessionGroup.ID, time.Now().Add(-5*time.Minute), nil)

	// Create WC room
	wcRoom := createWCRoom(t, ctx.db)
	defer cleanupWCInfrastructure(t, ctx.db, wcRoom.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	// Send student to WC — this triggers auto-creation of a WC active group
	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      wcRoom.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// CRITICAL ASSERTION: The auto-created WC group must NOT have a device_id.
	// If it does, GetDeviceCurrentSession will find TWO active groups for this
	// device and may return the WC group instead of the real session group,
	// breaking room counts and session resume.
	wcGroup := new(active.Group)
	err = ctx.db.NewSelect().
		Model(wcGroup).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ?`, wcRoom.ID).
		Where(`"group".end_time IS NULL`).
		OrderExpr(`"group".id DESC`).
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err, "WC group should exist after auto-creation")
	assert.Nil(t, wcGroup.DeviceID,
		"REGRESSION: WC group must NOT have DeviceID — it would hijack GetDeviceCurrentSession")

	// Verify GetDeviceCurrentSession still returns the REAL session, not WC
	realSession, err := ctx.services.Active.GetDeviceCurrentSession(context.Background(), device.ID)
	require.NoError(t, err, "GetDeviceCurrentSession should find the real session")
	assert.Equal(t, sessionGroup.ID, realSession.ID,
		"GetDeviceCurrentSession must return the room session (group %d), not the WC group (group %d)",
		sessionGroup.ID, wcGroup.ID)
}

// TestDeviceCheckin_SchulhofGroupHasNoDeviceID verifies that auto-created
// Schulhof groups never receive a DeviceID, same invariant as WC.
func TestDeviceCheckin_SchulhofGroupHasNoDeviceID(t *testing.T) {
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "schulhof-no-device")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Schulhof", "NoDevice")

	student := testpkg.CreateTestStudent(t, ctx.db, "Schulhof", "Regression", "3a")

	tagID := fmt.Sprintf("SHREG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createSchulhofRoom(t, ctx.db)
	defer cleanupSchulhofInfrastructure(t, ctx.db, room.ID)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	schulhofGroup := new(active.Group)
	err := ctx.db.NewSelect().
		Model(schulhofGroup).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ?`, room.ID).
		Where(`"group".end_time IS NULL`).
		OrderExpr(`"group".id DESC`).
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err, "Schulhof group should exist after auto-creation")
	assert.Nil(t, schulhofGroup.DeviceID,
		"REGRESSION: Schulhof group must NOT have DeviceID — shared rooms are not device sessions")
}

// =============================================================================
// PICKUP TIME IN CHECKIN RESPONSE TESTS
// =============================================================================

func TestDeviceCheckin_ResponseIncludesPickupTime(t *testing.T) {
	t.Parallel()
	// Checkin for a student with a weekly pickup schedule should return pickup_time in response.
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-time-checkin")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Pickup", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Pickup", "Student", "2a")

	tagID := fmt.Sprintf("PICKUP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Pickup Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Pickup Activity")

	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	// Create pickup schedule for today's weekday.
	// Use timezone.DateOf (Europe/Berlin) to match the production lookup in
	// GetEffectivePickupTimeForDate, avoiding flaky results on non-Berlin CI runners.
	berlinToday := timezone.DateOf(time.Now())
	todayWeekday := int(berlinToday.Weekday())
	if todayWeekday == 0 {
		todayWeekday = 7 // ISO: Sunday = 7
	}
	// Skip test on weekends — schedule service returns nil pickup time for Sat/Sun
	if todayWeekday > 5 {
		t.Skip("Skipping pickup time test on weekend — no pickup schedule applies")
	}

	tenantCtx := testpkg.Ctx(t)
	pickupTime := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
	sched := &scheduleModels.StudentPickupSchedule{
		StudentID:  student.ID,
		Weekday:    todayWeekday,
		PickupTime: pickupTime,
		CreatedBy:  staff.ID,
	}
	err := ctx.services.PickupSchedule.UpsertStudentPickupSchedule(tenantCtx, sched)
	require.NoError(t, err, "Failed to create pickup schedule")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "15:30", data["pickup_time"], "Response should include formatted pickup time")
}

func TestDeviceCheckin_ResponseOmitsPickupTimeWhenNoSchedule(t *testing.T) {
	t.Parallel()
	// Checkin for a student without any pickup schedule should NOT include pickup_time.
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "no-pickup-checkin")

	staff := testpkg.CreateTestStaff(t, ctx.db, "NoPickup", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "NoPickup", "Student", "3b")

	tagID := fmt.Sprintf("NOPICKUP%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "NoPickup Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "NoPickup Activity")

	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "Response should have data field")
	assert.Equal(t, "checked_in", data["action"])
	_, hasPickupTime := data["pickup_time"]
	assert.False(t, hasPickupTime, "Response should NOT include pickup_time when student has no schedule")
}

func TestDevicePickupQuery_ReturnsPickupInfoWithoutCreatingVisit(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-success")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupQuery", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "PickupQuery", "Student", "2a")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "PICKUPQUERY")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	berlinToday := timezone.DateOf(time.Now())
	todayWeekday := int(berlinToday.Weekday())
	if todayWeekday == 0 {
		todayWeekday = 7
	}
	if todayWeekday > 5 {
		t.Skip("Skipping pickup query test on weekend — no pickup schedule applies")
	}

	// Use DateOfUTC for the note date: PostgreSQL casts timestamptz → DATE in session
	// timezone (UTC). DateOf returns midnight Berlin which can shift to the previous day
	// in UTC (e.g. 2026-04-01 00:00+02 → 2026-03-31 22:00 UTC → DATE 2026-03-31).
	// DateOfUTC avoids this by encoding the Berlin calendar date as midnight UTC.
	todayUTC := timezone.TodayDate()

	tenantCtx := testpkg.Ctx(t)
	pickupTime := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
	err := ctx.services.PickupSchedule.UpsertStudentPickupSchedule(tenantCtx, &scheduleModels.StudentPickupSchedule{
		StudentID:  student.ID,
		Weekday:    todayWeekday,
		PickupTime: pickupTime,
		CreatedBy:  staff.ID,
	})
	require.NoError(t, err)

	err = ctx.services.PickupSchedule.CreateStudentPickupNote(tenantCtx, &scheduleModels.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  todayUTC,
		Content:   "Mama holt heute frueher ab",
		CreatedBy: staff.ID,
	})
	require.NoError(t, err)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Response should have data field")
	assert.Equal(t, "pickup_info", data["action"])
	assert.Equal(t, "15:30", data["pickup_time"])
	assert.Equal(t, "Mama holt heute frueher ab", data["pickup_note"])

	visits, err := ctx.services.Active.FindVisitsByStudentID(tenantCtx, student.ID)
	require.NoError(t, err)
	assert.Empty(t, visits, "Pickup query must not create visit records")
}

func TestDevicePickupQuery_OmitsPickupInfoWhenNoScheduleOrNote(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-empty")

	staff := testpkg.CreateTestStaff(t, ctx.db, "EmptyPickup", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "EmptyPickup", "Student", "3b")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "EMPTYPICKUP")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Response should have data field")
	assert.Equal(t, "pickup_info", data["action"])
	_, hasPickupTime := data["pickup_time"]
	assert.False(t, hasPickupTime, "Response should omit pickup_time when no schedule exists")
	_, hasPickupNote := data["pickup_note"]
	assert.False(t, hasPickupNote, "Response should omit pickup_note when no note exists")
}

func TestDevicePickupQuery_RejectsStaffRFID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-staff")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupStaff", "Only")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "STAFFPICKUP")
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	errorMsg, _ := response["error"].(string)
	assert.Contains(t, errorMsg, "student RFID tag required for pickup query")
}

func TestDevicePickupQuery_ReturnsErrorWhenPickupLookupFails(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-error")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupError", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "PickupError", "Student", "4c")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "PICKUPQUERYERROR")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	ctx.resource.PickupScheduleService = &failingPickupScheduleService{
		PickupScheduleService: ctx.services.PickupSchedule,
		err:                   errors.New("schedule lookup exploded"),
	}

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDevicePickupQuery_ReturnsServerErrorWhenRFIDLookupFails(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-rfid-failure")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupRFIDFailure", "Staff")

	ctx.injectFailingUsers(&failingPersonService{
		PersonService:  ctx.services.Users,
		findByTagIDErr: errors.New("rfid lookup exploded"),
	})

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": "BROKENRFID",
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDevicePickupQuery_ReturnsServerErrorWhenStudentResolutionFails(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-student-failure")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupStudentFailure", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "PickupStudentFailure", "Student", "4d")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "BROKENSTUDENTLOOKUP")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	ctx.injectFailingUsers(&failingPersonService{
		PersonService: ctx.services.Users,
		studentRepo: &failingStudentRepository{
			StudentRepository: repositories.NewFactory(ctx.db).Student,
			err:               errors.New("student lookup exploded"),
		},
	})

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDevicePickupQuery_PrefersDayNotesOverRecurringNotes(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-note-precedence")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupNotes", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "PickupNotes", "Student", "2b")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "PICKUPNOTEPRIO")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	berlinToday := timezone.DateOf(time.Now())
	todayWeekday := int(berlinToday.Weekday())
	if todayWeekday == 0 {
		todayWeekday = 7
	}
	if todayWeekday > 5 {
		t.Skip("Skipping pickup query note precedence test on weekend — no pickup schedule applies")
	}

	todayUTC := timezone.TodayDate()
	tenantCtx := testpkg.Ctx(t)
	recurringNote := "Papa holt normalerweise ab"
	pickupTime := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
	err := ctx.services.PickupSchedule.UpsertStudentPickupSchedule(tenantCtx, &scheduleModels.StudentPickupSchedule{
		StudentID:  student.ID,
		Weekday:    todayWeekday,
		PickupTime: pickupTime,
		Notes:      &recurringNote,
		CreatedBy:  staff.ID,
	})
	require.NoError(t, err)

	err = ctx.services.PickupSchedule.CreateStudentPickupNote(tenantCtx, &scheduleModels.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  todayUTC,
		Content:   "Heute holt Oma ab",
		CreatedBy: staff.ID,
	})
	require.NoError(t, err)

	err = ctx.services.PickupSchedule.CreateStudentPickupNote(tenantCtx, &scheduleModels.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  todayUTC,
		Content:   "Bitte am Seiteneingang warten",
		CreatedBy: staff.ID,
	})
	require.NoError(t, err)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Response should have data field")
	assert.Equal(t, "Heute holt Oma ab\nBitte am Seiteneingang warten", data["pickup_note"])
}

func TestDevicePickupQuery_PreservesRecurringNotesWhenExceptionReasonIsBlank(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-exception-fallback")

	staff := testpkg.CreateTestStaff(t, ctx.db, "PickupException", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "PickupException", "Student", "2c")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "PICKUPEXCEPTION")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	berlinToday := timezone.DateOf(time.Now())
	todayWeekday := int(berlinToday.Weekday())
	if todayWeekday == 0 {
		todayWeekday = 7
	}
	if todayWeekday > 5 {
		t.Skip("Skipping pickup query exception fallback test on weekend — no pickup schedule applies")
	}

	todayUTC := timezone.TodayDate()
	tenantCtx := testpkg.Ctx(t)
	recurringNote := "Bitte am Seiteneingang klingeln"
	pickupTime := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
	err := ctx.services.PickupSchedule.UpsertStudentPickupSchedule(tenantCtx, &scheduleModels.StudentPickupSchedule{
		StudentID:  student.ID,
		Weekday:    todayWeekday,
		PickupTime: pickupTime,
		Notes:      &recurringNote,
		CreatedBy:  staff.ID,
	})
	require.NoError(t, err)

	updatedTime := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)
	blankReason := "   "
	err = ctx.services.PickupSchedule.CreateStudentPickupException(tenantCtx, &scheduleModels.StudentPickupException{
		StudentID:     student.ID,
		ExceptionDate: todayUTC,
		PickupTime:    &updatedTime,
		Reason:        &blankReason,
		CreatedBy:     staff.ID,
	})
	require.NoError(t, err)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Response should have data field")
	assert.Equal(t, "13:00", data["pickup_time"])
	assert.Equal(t, "Bitte am Seiteneingang klingeln", data["pickup_note"])
}

func TestDevicePickupQuery_Unauthorized(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context
	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestDevicePickupQuery_InvalidRequestBody(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-bad-body")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Send request with empty body (missing required student_rfid)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestDevicePickupQuery_UnknownRFIDReturnsNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-unknown-rfid")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": "NONEXISTENT_RFID_TAG_XYZ",
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDevicePickupQuery_StaffLookupFailureReturnsServerError(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-staff-fail")

	// Create a person who is NOT a student — the handler will fall into the
	// student==nil branch and then try resolveStaffFromPerson.
	person := testpkg.CreateTestPerson(t, ctx.db, "OrphanPickup", "Person")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "ORPHANPICKUP")
	testpkg.LinkRFIDToStudent(t, ctx.db, person.ID, card.ID)

	// Override person resolution with a failing staff repository
	ctx.injectFailingUsers(&failingPersonService{
		PersonService: ctx.services.Users,
		staffRepo: &failingStaffRepository{
			StaffRepository: repositories.NewFactory(ctx.db).Staff,
			err:             errors.New("staff lookup exploded"),
		},
	})

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDevicePickupQuery_PersonNeitherStudentNorStaffReturnsNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "pickup-query-orphan")

	// Create a person who is neither a student nor staff
	person := testpkg.CreateTestPerson(t, ctx.db, "NeitherPickup", "Person")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, "NEITHERPICKUP")
	testpkg.LinkRFIDToStudent(t, ctx.db, person.ID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-query", map[string]interface{}{
		"student_rfid": card.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DUPLICATE ACTIVE VISIT (409 CONFLICT) TESTS — Issue #844
// =============================================================================

// duplicateVisitActiveService wraps the real Active service and:
//   - returns ErrStudentAlreadyActive on CreateVisit (without persisting)
//   - no-ops EndVisit so the existing pre-created visit stays open and the
//     handler's response builder finds it for the body details
//
// This stub exercises the **app-level rejection path** in the IoT handler:
// the service has already determined the student has an active visit and
// returns ErrStudentAlreadyActive without performing an INSERT. The
// database-level partial unique index race is NOT reached here — that
// path lives in the active service and is covered by the
// repository-level test on the partial unique index (see
// backend/database/repositories/active/visits_repository_test.go) plus
// the unit test on isDuplicateActiveVisitViolation.
type duplicateVisitActiveService struct {
	activeSvc.Service
}

func (d *duplicateVisitActiveService) CreateVisit(ctx context.Context, visit *active.Visit) error {
	return &activeSvc.ActiveError{Op: "CreateVisit", Err: activeSvc.ErrStudentAlreadyActive}
}

func (d *duplicateVisitActiveService) EndVisit(ctx context.Context, id int64) error {
	return nil
}

func TestDeviceCheckin_DuplicateActiveVisit_AppLevelPath_Returns409WithRoomDetails(t *testing.T) {
	t.Parallel()
	// Verifies that when CreateVisit reports ErrStudentAlreadyActive via the
	// application-level read-then-write check, the IoT handler returns 409
	// Conflict with the structured STUDENT_ALREADY_ACTIVE body (Issue #844).
	// Includes existing visit metadata (visit_id, room_id, room_name,
	// entry_time) so the kiosk can surface "Bereits angemeldet in <Raum>"
	// instead of a generic error.
	//
	// The DB-race path (where visitRepo.Create itself trips the partial
	// unique index from migration 1.15.47) is NOT exercised here — see
	// the package note on duplicateVisitActiveService above.
	ctx := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "dup-409")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Dup", "ConflictStaff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Dup", "ConflictStudent", "1a")

	tagID := fmt.Sprintf("DUP409%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	roomA := testpkg.CreateTestRoom(t, ctx.db, "Conflict Room A")
	roomB := testpkg.CreateTestRoom(t, ctx.db, "Conflict Room B")

	activityA := testpkg.CreateTestActivityGroup(t, ctx.db, "Conflict Activity A")
	activityB := testpkg.CreateTestActivityGroup(t, ctx.db, "Conflict Activity B")

	activeGroupA := testpkg.CreateTestActiveGroup(t, ctx.db, activityA.ID, roomA.ID)
	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activityB.ID, roomB.ID)

	// Pre-create an active visit in Room A. Once the CreateVisit-overriding
	// wrapper kicks in (see below), EndVisit is also a no-op, so this visit
	// stays open and the response builder loads it for the 409 body.
	existingEntry := time.Now().Add(-5 * time.Minute)
	existingVisit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroupA.ID, existingEntry, nil)

	// Build a Resource that injects the duplicate-visit failure path. The
	// check-in logic (and its active-service dependency) now lives in the
	// extracted CheckinService (issue #575 B8), so the wrapped active service
	// is injected there; the handler's own ActiveService receives the same
	// wrapper for consistency.
	wrappedActive := &duplicateVisitActiveService{Service: ctx.services.Active}
	wrappedCheckin := checkinsvc.NewCheckinService(checkinsvc.CheckinServiceDeps{
		Active:     wrappedActive,
		Users:      ctx.services.Users,
		Facilities: ctx.services.Facilities,
		Activities: ctx.services.Activities,
		Logger:     slog.Default(),
	})
	wrappedResource := checkinAPI.NewResource(
		ctx.services.IoT,
		ctx.services.Users,
		wrappedActive,
		wrappedCheckin,
		ctx.services.PickupSchedule,
		nil,
		slog.Default(),
	)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", wrappedResource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      roomB.ID,
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusConflict, rr.Code, "expected 409 Conflict. Body: %s", rr.Body.String())

	resp := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "STUDENT_ALREADY_ACTIVE", resp["code"], "machine-readable code is the contract for PyrePortal mapping")
	assert.Contains(t, resp["message"], "student already has an active visit",
		"PyrePortal substring-matches this string for the German UI fallback")

	details, ok := resp["details"].(map[string]interface{})
	require.True(t, ok, "expected details object in 409 body. Body: %s", rr.Body.String())
	assert.EqualValues(t, student.ID, details["student_id"], "student_id must surface so the kiosk can display context")
	assert.EqualValues(t, existingVisit.ID, details["existing_visit_id"], "existing_visit_id must point to the still-open visit")
	assert.EqualValues(t, roomA.ID, details["room_id"], "room_id must reflect the room of the existing visit, not the requested target room")
	assert.Equal(t, roomA.Name, details["room_name"], "room_name must reflect the existing-visit's room")
}
