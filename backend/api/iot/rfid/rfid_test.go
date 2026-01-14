// Package rfid_test tests the IoT RFID API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package rfid_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	rfidAPI "github.com/moto-nrw/project-phoenix/api/iot/rfid"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *rfidAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create RFID resource
	resource := rfidAPI.NewResource(svc.Users)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// ASSIGN RFID TAG TESTS
// =============================================================================

func TestAssignRFIDTag_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"rfid_tag": "TESTRFID001",
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest("POST", "/1/rfid", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestAssignRFIDTag_InvalidStaffID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-1")

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"rfid_tag": "TESTRFID001",
	}

	req := testutil.NewAuthenticatedRequest("POST", "/invalid/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-2")

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/1/rfid", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	// Add device context
	reqCtx := context.WithValue(req.Context(), device.CtxDevice, testDevice)
	req = req.WithContext(reqCtx)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_MissingRFIDTag(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-3")

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{} // Missing rfid_tag

	req := testutil.NewAuthenticatedRequest("POST", "/1/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_RFIDTagTooShort(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-4")

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"rfid_tag": "SHORT", // Less than 8 characters
	}

	req := testutil.NewAuthenticatedRequest("POST", "/1/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_StaffNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-5")

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"rfid_tag": "TESTRFID001",
	}

	req := testutil.NewAuthenticatedRequest("POST", "/99999/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAssignRFIDTag_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-6")
	staff := testpkg.CreateTestStaff(t, ctx.db, "RFID", "Staff1")
	// Create RFID card first (card ID must be hexadecimal)
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "A1B2C3D4E5F60001")

	router := chi.NewRouter()
	router.Post("/{staffId}/rfid", ctx.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"rfid_tag": rfidCard.ID, // Use the created card ID
	}

	req := testutil.NewAuthenticatedRequest("POST", fmt.Sprintf("/%d/rfid", staff.ID), body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// UNASSIGN RFID TAG TESTS
// =============================================================================

func TestUnassignRFIDTag_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/{staffId}/rfid", ctx.resource.UnassignRFIDTagHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest("DELETE", "/1/rfid", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestUnassignRFIDTag_InvalidStaffID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-7")

	router := chi.NewRouter()
	router.Delete("/{staffId}/rfid", ctx.resource.UnassignRFIDTagHandler())

	req := testutil.NewAuthenticatedRequest("DELETE", "/invalid/rfid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUnassignRFIDTag_StaffNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-8")

	router := chi.NewRouter()
	router.Delete("/{staffId}/rfid", ctx.resource.UnassignRFIDTagHandler())

	req := testutil.NewAuthenticatedRequest("DELETE", "/99999/rfid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUnassignRFIDTag_NoTagAssigned(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-9")
	staff := testpkg.CreateTestStaff(t, ctx.db, "NoTag", "Staff")

	router := chi.NewRouter()
	router.Delete("/{staffId}/rfid", ctx.resource.UnassignRFIDTagHandler())

	req := testutil.NewAuthenticatedRequest("DELETE", fmt.Sprintf("/%d/rfid", staff.ID), nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUnassignRFIDTag_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-10")
	staff := testpkg.CreateTestStaff(t, ctx.db, "HasTag", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID200")
	// Link RFID to staff's person
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Delete("/{staffId}/rfid", ctx.resource.UnassignRFIDTagHandler())

	req := testutil.NewAuthenticatedRequest("DELETE", fmt.Sprintf("/%d/rfid", staff.ID), nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
