package students_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	iotModel "github.com/moto-nrw/project-phoenix/models/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// RFID Handler Tests (Device Authentication)
// =============================================================================

// testDevicePIN is the OGS device PIN used to authenticate the RFID routes. The
// students Router() wires device.DeviceAuthenticator with a nil PIN resolver, so
// the PIN is read from the OGS_DEVICE_PIN env var (set per-request by deviceExec).
const testDevicePIN = "1234"

// deviceExec authenticates as the given RFID device and runs the request through
// the production Router(), which mounts the real device.DeviceAuthenticator +
// TenantTxMiddleware chain on the /{id}/rfid routes exactly as the server does.
// The device API key goes in the Authorization header and the staff PIN in
// X-Staff-PIN; the PIN must match OGS_DEVICE_PIN because the students Router uses
// a nil PIN resolver.
func deviceExec(t *testing.T, tc *testContext, req *http.Request, device *iotModel.Device) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("OGS_DEVICE_PIN", testDevicePIN)
	req.Header.Set("Authorization", "Bearer "+*device.APIKey)
	req.Header.Set("X-Staff-PIN", testDevicePIN)
	return testutil.ExecuteRequest(tc.resource.Router(), req)
}

func TestAssignRFIDTag_WithDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	// Create test device and student
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-reader")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "TagTest", "RT1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID, student.ID)

	t.Run("success_assigns_rfid_tag", func(t *testing.T) {
		// RFID tag must be hexadecimal format (at least 8 chars)
		body := map[string]interface{}{
			"rfid_tag": fmt.Sprintf("%016X", time.Now().UnixNano()),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

		rr := deviceExec(t, tc, req, device)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "success", "Response should indicate success")
	})

	t.Run("bad_request_missing_rfid_tag", func(t *testing.T) {
		body := map[string]interface{}{}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_empty_rfid_tag", func(t *testing.T) {
		body := map[string]interface{}{
			"rfid_tag": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_student", func(t *testing.T) {
		body := map[string]interface{}{
			"rfid_tag": "TESTTAG123",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/999999/rfid", body)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_invalid_student_id", func(t *testing.T) {
		body := map[string]interface{}{
			"rfid_tag": "TESTTAG123",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/invalid/rfid", body)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUnassignRFIDTag_WithDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	// Create test device and student with RFID tag
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-unassign")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "UnassignTest", "RU1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID, student.ID)

	t.Run("error_no_tag_assigned", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/rfid", student.ID), nil)

		rr := deviceExec(t, tc, req, device)

		// Student has no RFID tag assigned
		testutil.AssertNotFound(t, rr)
		assert.Contains(t, rr.Body.String(), "no RFID tag assigned")
	})

	t.Run("not_found_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/999999/rfid", nil)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_invalid_student_id", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/invalid/rfid", nil)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertBadRequest(t, rr)
	})
}

func TestAssignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "NoDevice", "RND1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	body := map[string]interface{}{
		"rfid_tag": "TESTTAG12345678",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	// Without device credentials, should return unauthorized
	testutil.AssertUnauthorized(t, rr)
}

func TestUnassignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "NoDeviceUnassign", "RNDU1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/rfid", student.ID), nil)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	// Without device credentials, should return unauthorized
	testutil.AssertUnauthorized(t, rr)
}

func TestUnassignRFIDTag_WithAssignedTag(t *testing.T) {
	tc := setupTestContext(t)

	// Create device and student
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-unassign-success")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "Unassign", "RUS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID, student.ID)

	// First assign an RFID tag
	tagID := fmt.Sprintf("%016X", time.Now().UnixNano())
	assignBody := map[string]interface{}{
		"rfid_tag": tagID,
	}
	assignReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), assignBody)
	assignRR := deviceExec(t, tc, assignReq, device)
	require.Equal(t, http.StatusOK, assignRR.Code, "Tag assignment should succeed")

	// Now unassign the tag
	t.Run("success_unassigns_rfid_tag", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/rfid", student.ID), nil)

		rr := deviceExec(t, tc, req, device)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "success")
		assert.Contains(t, rr.Body.String(), "unassigned successfully")
	})
}

func TestRFIDTagValidation(t *testing.T) {
	tc := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, tc.db, "rfid-validation")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "Validation", "RV1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID, student.ID)

	t.Run("too_short_tag", func(t *testing.T) {
		// Tag must be at least 8 characters
		body := map[string]interface{}{
			"rfid_tag": "AB12",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "at least 8 characters")
	})

	t.Run("too_long_tag", func(t *testing.T) {
		// Tag must be at most 64 characters
		longTag := "AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJKKKKLLLLMMMMNNNNOOOOPPPPQQQQ" // 68 chars
		body := map[string]interface{}{
			"rfid_tag": longTag,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

		rr := deviceExec(t, tc, req, device)

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "at most 64 characters")
	})
}
