// Package sessions_test tests the IoT sessions API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package sessions_test

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	sessionsAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/sessions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *testpkg.DB
	resource *sessionsAPI.Resource
}

// setupSessionsRoute initializes the sessions route.
func setupSessionsRoute(t *testing.T) *testContext {
	t.Helper()

	db, svc, svcSessionEnd := setupActiveModule(t)

	// Create sessions resource
	resource := sessionsAPI.NewResource(svc.SessionLifecycle, api.BindIoTSessionEnd(svcSessionEnd.SessionEnd), testRuntime())

	return &testContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// START SESSION TESTS
// =============================================================================

func TestStartSession_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"activity_id": 1,
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/start", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestStartSession_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-1")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/start", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.WithDeviceContext(testDevice)(req)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestStartSession_MissingActivityID(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-2")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-3")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupSessionsRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/end", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestEndSession_NoActiveSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-4")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupSessionsRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetCurrentSession_NoActiveSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-5")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return success with is_active=false
	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestGetCurrentSession_WithActiveSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	// Create real fixtures for a full session
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-current-1")
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Current Session Activity")
	staff := testpkg.CreateTestStaff(t, ctx.db, "CurrentSession", "Supervisor")
	// The room is passed explicitly: with neither a manual nor a planned room,
	// determineRoomIDWithStrategy now fails with ErrNoRoomAvailable.
	room := testpkg.CreateTestRoom(t, ctx.db, "Current Session Room")

	// Start a real session with supervisors
	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	startBody := map[string]interface{}{
		"activity_id":    activity.ID,
		"room_id":        room.ID,
		"supervisor_ids": []int64{staff.ID},
	}

	startReq := testutil.NewAuthenticatedRequest(t, "POST", "/start", startBody,
		testutil.WithDeviceContext(testDevice),
	)

	startRR := testutil.ExecuteRequest(router, startReq)
	t.Logf("Start session response: %d - %s", startRR.Code, startRR.Body.String())

	require.Equal(t, 200, startRR.Code, "Session start must succeed; body: %s", startRR.Body.String())

	// Now call getCurrentSession — this exercises the supervisor lookup (lines 173-178)
	currentReq := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil,
		testutil.WithDeviceContext(testDevice),
	)

	currentRR := testutil.ExecuteRequest(router, currentReq)

	testutil.AssertSuccessResponse(t, currentRR, 200)

	// Verify the response contains session data with supervisors
	responseBody := testutil.ParseJSONResponse(t, currentRR.Body.Bytes())
	data, ok := responseBody["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in response, got: %v", responseBody)
	}

	assert.True(t, data["is_active"].(bool), "Session should be active")
	assert.NotNil(t, data["active_group_id"], "Should have active_group_id")
	assert.NotNil(t, data["activity_id"], "Should have activity_id")
	assert.Equal(t, float64(0), data["active_students"], "an empty session has zero active students")
	groupID := int64(data["active_group_id"].(float64))
	closedStudent := testpkg.CreateTestStudent(t, ctx.db, "Exited", "Session", "3a")
	exitedAt := time.Now().Add(-time.Minute)
	testpkg.CreateTestVisit(t, ctx.db, closedStudent.ID, groupID, exitedAt.Add(-time.Hour), &exitedAt)
	for _, openCount := range []int{0, 2} {
		for range openCount {
			student := testpkg.CreateTestStudent(t, ctx.db, "Open", "Session", "3a")
			testpkg.CreateTestVisit(t, ctx.db, student.ID, groupID, time.Now(), nil)
		}
		request := testutil.NewAuthenticatedRequest(t, "GET", "/current", nil, testutil.WithDeviceContext(testDevice))
		response := testutil.ExecuteRequest(router, request)
		testutil.AssertSuccessResponse(t, response, 200)
		body := testutil.ParseJSONResponse(t, response.Body.Bytes())
		assert.Equal(t, float64(openCount), body["data"].(map[string]interface{})["active_students"], "exited visits must not count")
	}

	// Verify supervisors are included in the response
	if supervisors, hasSupervisors := data["supervisors"]; hasSupervisors && supervisors != nil {
		supervisorList, ok := supervisors.([]interface{})
		assert.True(t, ok, "Supervisors should be a list")
		assert.NotEmpty(t, supervisorList, "Should have at least one supervisor")
	}
}

// =============================================================================
// CHECK CONFLICT TESTS
// =============================================================================

func TestCheckConflict_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"activity_id": 1,
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestCheckConflict_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-6")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/check-conflict", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.WithDeviceContext(testDevice)(req)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCheckConflict_MissingActivityID(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-7")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupSessionsRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"supervisor_ids": []int64{1},
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/1/supervisors", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestUpdateSupervisors_InvalidSessionID(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-8")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-9")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
// UPDATE ACTIVITY TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestUpdateActivity_InvalidActivityType(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-11")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"activity_type": "invalid_type",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activity", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// A rejected binding goes through renderError, which classifies every
	// error outside the device-scan failures as 500. This pins today's
	// status; answering 400 needs a production change.
	testutil.AssertErrorResponse(t, rr, 500)
	assert.Contains(t, rr.Body.String(), "activity_type: must be a valid value.")
}

// =============================================================================
// VALIDATE TIMEOUT TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestValidateTimeout_StatusCodes(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-validate-timeout")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	now := time.Now().Format(time.RFC3339)
	// A rejected binding goes through renderError, which classifies every
	// error outside the device-scan failures as 500. The 500 rows pin
	// today's status; answering 400 needs a production change.
	tests := []struct {
		name   string
		body   map[string]interface{}
		status int
		detail string
	}{
		{"missing timeout_minutes", map[string]interface{}{"last_activity": now}, 500, "timeout_minutes: cannot be blank."},
		{"zero timeout", map[string]interface{}{"timeout_minutes": 0, "last_activity": now}, 500, "timeout_minutes: cannot be blank."},
		{"negative timeout", map[string]interface{}{"timeout_minutes": -5, "last_activity": now}, 500, "timeout_minutes: must be no less than 1."},
		{"one above maximum", map[string]interface{}{"timeout_minutes": 481, "last_activity": now}, 500, "timeout_minutes: must be no greater than 480."},
		{"far above maximum", map[string]interface{}{"timeout_minutes": 1000, "last_activity": now}, 500, "timeout_minutes: must be no greater than 480."},
		{"missing last_activity", map[string]interface{}{"timeout_minutes": 30}, 500, "last_activity: cannot be blank."},
		{"valid request without session", map[string]interface{}{"timeout_minutes": 30, "last_activity": time.Now().Add(-10 * time.Minute).Format(time.RFC3339)}, 404, "no active session found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, "POST", "/validate-timeout", tt.body,
				testutil.WithDeviceContext(testDevice),
			)

			rr := testutil.ExecuteRequest(router, req)

			testutil.AssertErrorResponse(t, rr, tt.status)
			assert.Contains(t, rr.Body.String(), tt.detail)
		})
	}
}

// =============================================================================
// GET TIMEOUT INFO TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestGetTimeoutInfo_NoActiveSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-14")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/timeout-info", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "no active session found")
}

// =============================================================================
// PROCESS TIMEOUT TESTS
// Note: Relies on middleware for device auth, so we only test with device context
// =============================================================================

func TestProcessTimeout_NoActiveSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-15")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	req := testutil.NewAuthenticatedRequest(t, "POST", "/timeout", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "no active session - please start an activity first")
}

// =============================================================================
// ROUTER AND RESOURCE TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should return a valid chi.Router")
}

// =============================================================================
// START SESSION WITH VALID ACTIVITY TESTS
// =============================================================================

func TestStartSession_NonExistentActivity(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-16")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"activity_id": 999999, // Non-existent activity
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/start", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for non-existent activity
	assert.Contains(t, []int{400, 404, 500}, rr.Code)
}

// =============================================================================
// CHECK CONFLICT WITH REAL ACTIVITY TESTS
// =============================================================================

func TestCheckConflict_NoConflict(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-19")
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "NoConflict Activity")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"activity_id": activity.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
	data := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]interface{})
	assert.Equal(t, false, data["has_conflict"])
	assert.Equal(t, true, data["can_override"])
	assert.Equal(t, "", data["conflict_message"])
	assert.NotContains(t, data, "conflicting_device")
}

func TestCheckConflict_ActivityRunningOnAnotherDevice(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	runningDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-conflict-running")
	askingDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-conflict-asking")
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Conflict Activity")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())
	startSession(t, ctx, router, runningDevice, activity.ID)

	req := testutil.NewAuthenticatedRequest(t, "POST", "/check-conflict", map[string]interface{}{"activity_id": activity.ID},
		testutil.WithDeviceContext(askingDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, 409, rr.Code, rr.Body.String())
	details := testutil.ParseJSONResponse(t, rr.Body.Bytes())["details"].(map[string]interface{})
	assert.Equal(t, true, details["has_conflict"])
	assert.Equal(t, true, details["can_override"])
	assert.Equal(t, float64(runningDevice.ID), details["conflicting_device"])
	assert.Equal(t, fmt.Sprintf("Activity is already active on device %d", runningDevice.ID), details["conflict_message"])
}

// =============================================================================
// UPDATE SUPERVISORS ADDITIONAL TESTS
// =============================================================================

func TestUpdateSupervisors_NonExistentSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-20")
	staff := testpkg.CreateTestStaff(t, ctx.db, "UpdateSup", "Test")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"supervisor_ids": []int64{staff.ID},
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999/supervisors", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "active group not found")
}

func TestUpdateSupervisors_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-21")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Send invalid JSON body
	req := httptest.NewRequest("PUT", "/1/supervisors", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.WithDeviceContext(testDevice)(req)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SESSION ACTIVITY UPDATE TESTS
// =============================================================================

func TestUpdateActivity_ValidTypesWithoutSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-22")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	validTypes := []string{"rfid_scan", "button_press", "ui_interaction"}

	for _, activityType := range validTypes {
		t.Run(activityType, func(t *testing.T) {
			body := map[string]interface{}{
				"activity_type": activityType,
				"timestamp":     time.Now().Format(time.RFC3339),
			}

			req := testutil.NewAuthenticatedRequest(t, "POST", "/activity", body,
				testutil.WithDeviceContext(testDevice),
			)

			rr := testutil.ExecuteRequest(router, req)

			// The type passes validation; the device has no session to touch.
			testutil.AssertNotFound(t, rr)
			assert.Contains(t, rr.Body.String(), "no active session found")
		})
	}
}

// TestUpdateActivity_RefreshesActiveSession sends PyrePortal's keep-alive
// (services/api.ts updateSessionActivity: rfid_scan with an ISO timestamp)
// and pins that it answers 200 for the device's session and writes the
// session heartbeat.
func TestUpdateActivity_RefreshesActiveSession(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-activity-refresh")
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity Refresh")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())
	sessionID := startSession(t, ctx, router, testDevice, activity.ID)
	stale := backdateLastActivity(t, ctx.db, sessionID)

	body := map[string]interface{}{
		"activity_type": "rfid_scan",
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/activity", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
	data := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]interface{})
	assert.Equal(t, float64(sessionID), data["session_id"])
	assert.Equal(t, "rfid_scan", data["activity_type"])
	assert.True(t, testpkg.ActiveGroupLastActivity(t, ctx.db, sessionID).After(stale), "the keep-alive refreshes the session heartbeat")
}

func TestUpdateActivity_MissingTimestamp(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-23")
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Activity Without Timestamp")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())
	sessionID := startSession(t, ctx, router, testDevice, activity.ID)
	stale := backdateLastActivity(t, ctx.db, sessionID)

	// The timestamp is optional and defaults to now.
	body := map[string]interface{}{
		"activity_type": "rfid_scan",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activity", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.True(t, testpkg.ActiveGroupLastActivity(t, ctx.db, sessionID).After(stale), "the keep-alive refreshes the session heartbeat")
}

// startSession starts a session for the activity on the device through the
// start route, with an explicit room and one supervisor, and returns its ID.
func startSession(t *testing.T, ctx *testContext, router chi.Router, device *testpkg.Device, activityID int64) int64 {
	t.Helper()
	room := testpkg.CreateTestRoom(t, ctx.db, "Session Room")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Session", "Supervisor")
	body := map[string]interface{}{
		"activity_id":    activityID,
		"room_id":        room.ID,
		"supervisor_ids": []int64{staff.ID},
	}
	rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/start", body,
		testutil.WithDeviceContext(device),
	))
	require.Equal(t, 200, rr.Code, "Session start must succeed; body: %s", rr.Body.String())
	data := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]interface{})
	return int64(data["active_group_id"].(float64))
}

// backdateLastActivity moves a session's heartbeat an hour into the past, so
// a later read proves that the request wrote it rather than that no time
// passed. It returns the stored stale value.
func backdateLastActivity(t *testing.T, db *testpkg.DB, activeGroupID int64) time.Time {
	t.Helper()
	_, err := db.NewRaw("UPDATE active.groups SET last_activity = last_activity - interval '1 hour' WHERE id = ?", activeGroupID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return testpkg.ActiveGroupLastActivity(t, db, activeGroupID)
}

// TestGetCurrentSession_CountCarriesItsRevision pins the poll side of #5128:
// active_students comes with the session's visit revision from the same
// snapshot, and every change of the count raises it.
func TestGetCurrentSession_CountCarriesItsRevision(t *testing.T) {
	t.Parallel()
	ctx := setupSessionsRoute(t)
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "sessions-test-device-revision")
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Revision Session Activity")
	room := testpkg.CreateTestRoom(t, ctx.db, "Revision Session Room")
	group := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	testpkg.LinkDeviceToActiveGroup(t, ctx.db, group.ID, testDevice.ID)
	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())
	current := func() map[string]interface{} {
		t.Helper()
		response := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", "/current", nil, testutil.WithDeviceContext(testDevice)))
		testutil.AssertSuccessResponse(t, response, 200)
		return testutil.ParseJSONResponse(t, response.Body.Bytes())["data"].(map[string]interface{})
	}

	empty := current()
	assert.Equal(t, float64(group.ID), empty["active_group_id"])
	assert.Equal(t, float64(0), empty["active_students"])
	assert.Equal(t, float64(0), empty["active_students_revision"])

	first := testpkg.CreateTestStudent(t, ctx.db, "First", "Revision", "3a")
	second := testpkg.CreateTestStudent(t, ctx.db, "Second", "Revision", "3a")
	checkedIn := time.Now().Add(-time.Minute)
	testpkg.CreateTestVisit(t, ctx.db, first.ID, group.ID, checkedIn, nil)
	leaving := testpkg.CreateTestVisit(t, ctx.db, second.ID, group.ID, checkedIn, nil)
	two := current()
	assert.Equal(t, float64(2), two["active_students"])
	assert.Equal(t, float64(2), two["active_students_revision"])
	again := current()
	assert.Equal(t, []any{two["active_students"], two["active_students_revision"]}, []any{again["active_students"], again["active_students_revision"]},
		"an unchanged count answers the same revision")

	_, err := ctx.db.NewRaw("UPDATE active.visits SET exit_time = NOW() WHERE id = ?", leaving.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	one := current()
	assert.Equal(t, float64(1), one["active_students"])
	assert.Equal(t, float64(3), one["active_students_revision"], "a checkout raises the revision")
}
