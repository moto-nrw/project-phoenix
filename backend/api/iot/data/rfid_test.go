// Package data_test tests (rfid) the IoT RFID API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package data_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// rfidTestContext holds shared test dependencies.
type rfidTestContext struct {
	db       *bun.DB
	resource *dataAPI.RFIDResource
}

// setupRFIDRoute initializes the RFID route.
func setupRFIDRoute(t *testing.T) *rfidTestContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create RFID resource
	resource := dataAPI.NewRFIDResource(svc.Users)

	return &rfidTestContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// ASSIGN RFID TAG TESTS
// =============================================================================

func TestAssignRFIDTag_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"rfid_tag": "TESTRFID001",
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/1/rfid", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestAssignRFIDTag_InvalidStaffID(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-1")

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"rfid_tag": "TESTRFID001",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/invalid/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-2")

	router := ctx.resource.Router()

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
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-3")

	router := ctx.resource.Router()

	body := map[string]interface{}{} // Missing rfid_tag

	req := testutil.NewAuthenticatedRequest(t, "POST", "/1/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_RFIDTagTooShort(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-4")

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"rfid_tag": "SHORT", // Less than 8 characters
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/1/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAssignRFIDTag_StaffNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-5")

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"rfid_tag": "TESTRFID001",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/99999/rfid", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAssignRFIDTag_Success(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-6")
	staff := testpkg.CreateTestStaff(t, ctx.db, "RFID", "Staff1")
	// Create RFID card first (card ID must be hexadecimal)
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "A1B2C3D4E5F60001")

	router := ctx.resource.Router()

	body := map[string]interface{}{
		"rfid_tag": rfidCard.ID, // Use the created card ID
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", staff.ID), body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// UNASSIGN RFID TAG TESTS
// =============================================================================

func TestUnassignRFIDTag_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/1/rfid", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestUnassignRFIDTag_InvalidStaffID(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-7")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/invalid/rfid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUnassignRFIDTag_StaffNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-8")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/99999/rfid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUnassignRFIDTag_NoTagAssigned(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-9")
	staff := testpkg.CreateTestStaff(t, ctx.db, "NoTag", "Staff")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d/rfid", staff.ID), nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUnassignRFIDTag_Success(t *testing.T) {
	t.Parallel()
	ctx := setupRFIDRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "rfid-test-device-10")
	staff := testpkg.CreateTestStaff(t, ctx.db, "HasTag", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID200")
	// Link RFID to staff's person
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, rfidCard.ID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d/rfid", staff.ID), nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
