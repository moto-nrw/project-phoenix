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
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
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

// Deliberately NOT parallel: process-global state — t.Setenv on
// OGS_DEVICE_PIN through the device-auth helper.
func TestAssignRFIDTag_WithDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	// Create test device and student
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-reader")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "TagTest", "RT1")

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

// Deliberately NOT parallel: process-global state — t.Setenv on
// OGS_DEVICE_PIN through the device-auth helper.
func TestUnassignRFIDTag_WithDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	// Create test device and student with RFID tag
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-unassign")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "UnassignTest", "RU1")

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
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "NoDevice", "RND1")

	body := map[string]interface{}{
		"rfid_tag": "TESTTAG12345678",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID), body)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	// Without device credentials, should return unauthorized
	testutil.AssertUnauthorized(t, rr)
}

func TestUnassignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "NoDeviceUnassign", "RNDU1")

	req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/rfid", student.ID), nil)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)

	// Without device credentials, should return unauthorized
	testutil.AssertUnauthorized(t, rr)
}

// Deliberately NOT parallel: process-global state — t.Setenv on
// OGS_DEVICE_PIN through the device-auth helper.
func TestUnassignRFIDTag_WithAssignedTag(t *testing.T) {
	tc := setupTestContext(t)

	// Create device and student
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-unassign-success")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "Unassign", "RUS1")

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

// Deliberately NOT parallel: process-global state — t.Setenv on
// OGS_DEVICE_PIN through the device-auth helper.
func TestRFIDTagValidation(t *testing.T) {
	tc := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, tc.db, "rfid-validation")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "Validation", "RV1")

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

// TestRFIDTagRoutes_GraduatedStudent covers the P1 fix (#405 review): the shared
// alumnus gate 404s every per-student route, which for a graduate still holding
// a bracelet created a tag the kiosk could see but never release. Releasing must
// therefore work on an alumnus, while assigning a NEW tag to a soft-deleted
// child stays blocked.
// Deliberately NOT parallel: process-global state — t.Setenv on
// OGS_DEVICE_PIN through the device-auth helper.
func TestRFIDTagRoutes_GraduatedStudent(t *testing.T) {
	tc := setupTestContext(t)

	device := testpkg.CreateTestDevice(t, tc.db, "rfid-alumnus")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "Graduated", "RG1")

	// Assign while the child is still enrolled, then graduate them.
	tagID := fmt.Sprintf("%016X", time.Now().UnixNano())
	assignReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID),
		map[string]interface{}{"rfid_tag": tagID})
	require.Equal(t, http.StatusOK, deviceExec(t, tc, assignReq, device).Code)

	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModel.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	t.Run("rejects_assigning_a_new_tag", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/rfid", student.ID),
			map[string]interface{}{"rfid_tag": fmt.Sprintf("%016X", time.Now().UnixNano())})

		rr := deviceExec(t, tc, req, device)

		assert.Equal(t, http.StatusNotFound, rr.Code,
			"a soft-deleted child must not be given a bracelet. Body: %s", rr.Body.String())
	})

	t.Run("releases_the_tag_they_still_hold", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/rfid", student.ID), nil)

		rr := deviceExec(t, tc, req, device)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "unassigned successfully")
	})
}
