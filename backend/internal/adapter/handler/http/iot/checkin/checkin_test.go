package checkin_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	checkinAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/checkin"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *checkinAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	t.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "15:00")

	db, svc := testutil.SetupAPITest(t)

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
