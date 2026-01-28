package students_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// RFID Handler Tests (Device Authentication)
// =============================================================================

func TestAssignRFIDTag_WithDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	// Create test device and student
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-reader")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "TagTest", "RT1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID, student.ID)

	t.Run("success_assigns_rfid_tag", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		// RFID tag must be hexadecimal format (at least 8 chars)
		body := map[string]interface{}{
			"rfid_tag": fmt.Sprintf("%016X", time.Now().UnixNano()),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "success", "Response should indicate success")
	})

	t.Run("bad_request_missing_rfid_tag", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		body := map[string]interface{}{}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_empty_rfid_tag", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		body := map[string]interface{}{
			"rfid_tag": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_student", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		body := map[string]interface{}{
			"rfid_tag": "TESTTAG123",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/students/999999/rfid-tag", body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_invalid_student_id", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		body := map[string]interface{}{
			"rfid_tag": "TESTTAG123",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/students/invalid/rfid-tag", body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

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
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Delete("/students/{id}/rfid-tag", tc.resource.UnassignRFIDTagHandler())

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/students/%d/rfid-tag", student.ID), nil,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		// Student has no RFID tag assigned
		testutil.AssertNotFound(t, rr)
		assert.Contains(t, rr.Body.String(), "no RFID tag assigned")
	})

	t.Run("not_found_nonexistent_student", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Delete("/students/{id}/rfid-tag", tc.resource.UnassignRFIDTagHandler())

		req := testutil.NewRequest("DELETE", "/students/999999/rfid-tag", nil,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_invalid_student_id", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Delete("/students/{id}/rfid-tag", tc.resource.UnassignRFIDTagHandler())

		req := testutil.NewRequest("DELETE", "/students/invalid/rfid-tag", nil,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})
}

func TestAssignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "NoDevice", "RND1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"rfid_tag": "TESTTAG12345678",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), body)

	rr := testutil.ExecuteRequest(router, req)

	// Without device context, should return unauthorized
	testutil.AssertUnauthorized(t, rr)
}

func TestUnassignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "NoDeviceUnassign", "RNDU1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Delete("/students/{id}/rfid-tag", tc.resource.UnassignRFIDTagHandler())

	req := testutil.NewRequest("DELETE", fmt.Sprintf("/students/%d/rfid-tag", student.ID), nil)

	rr := testutil.ExecuteRequest(router, req)

	// Without device context, should return unauthorized
	testutil.AssertUnauthorized(t, rr)
}

func TestUnassignRFIDTag_WithAssignedTag(t *testing.T) {
	tc := setupTestContext(t)

	// Create device and student
	device := testpkg.CreateTestDevice(t, tc.db, "rfid-unassign-success")
	student := testpkg.CreateTestStudent(t, tc.db, "RFID", "Unassign", "RUS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, device.ID, student.ID)

	// First assign an RFID tag
	assignRouter := chi.NewRouter()
	assignRouter.Use(render.SetContentType(render.ContentTypeJSON))
	assignRouter.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

	tagID := fmt.Sprintf("%016X", time.Now().UnixNano())
	assignBody := map[string]interface{}{
		"rfid_tag": tagID,
	}
	assignReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), assignBody,
		testutil.WithDeviceContext(device),
	)
	assignRR := testutil.ExecuteRequest(assignRouter, assignReq)
	require.Equal(t, http.StatusOK, assignRR.Code, "Tag assignment should succeed")

	// Now unassign the tag
	t.Run("success_unassigns_rfid_tag", func(t *testing.T) {
		unassignRouter := chi.NewRouter()
		unassignRouter.Use(render.SetContentType(render.ContentTypeJSON))
		unassignRouter.Delete("/students/{id}/rfid-tag", tc.resource.UnassignRFIDTagHandler())

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/students/%d/rfid-tag", student.ID), nil,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(unassignRouter, req)

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
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		// Tag must be at least 8 characters
		body := map[string]interface{}{
			"rfid_tag": "AB12",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "at least 8 characters")
	})

	t.Run("too_long_tag", func(t *testing.T) {
		router := chi.NewRouter()
		router.Use(render.SetContentType(render.ContentTypeJSON))
		router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

		// Tag must be at most 64 characters
		longTag := "AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJKKKKLLLLMMMMNNNNOOOOPPPPQQQQ" // 68 chars
		body := map[string]interface{}{
			"rfid_tag": longTag,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/students/%d/rfid-tag", student.ID), body,
			testutil.WithDeviceContext(device),
		)

		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "at most 64 characters")
	})
}
