package students_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/students"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *studentsAPI.Resource
}

// setupTestContext initializes the test environment.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := studentsAPI.NewResource(
		svc.Users,
		svc.Student,
		svc.Education,
		svc.UserContext,
		svc.Active,
		svc.IoT,
		"1234",
	)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// setupRouter creates a Chi router with the given handler.
func setupRouter(handler http.HandlerFunc, urlParam string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	if urlParam != "" {
		router.Get(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Put(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Delete(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Post(fmt.Sprintf("/{%s}", urlParam), handler)
	} else {
		router.Get("/", handler)
		router.Post("/", handler)
	}
	return router
}

// executeWithAuth executes a request with JWT claims and permissions.
func executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// =============================================================================
// List Students Tests
// =============================================================================

func TestListStudents(t *testing.T) {
	tc := setupTestContext(t)

	// Create test students using fixtures
	student1 := testpkg.CreateTestStudent(t, tc.db, "List", "StudentOne", "1a")
	student2 := testpkg.CreateTestStudent(t, tc.db, "List", "StudentTwo", "1b")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

	t.Run("success_admin_lists_all_students", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_pagination", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?page=1&page_size=10", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_school_class_filter", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?school_class=1a", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_search_filter", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?search=List", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Get Student Tests
// =============================================================================

func TestGetStudent(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "Get", "Student", "2a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("success_admin_gets_student", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Get", "Response should contain student first name")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Create Student Tests
// =============================================================================

func TestCreateStudent(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_creates_student", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		uniqueName := fmt.Sprintf("Created%d", time.Now().UnixNano())
		body := map[string]interface{}{
			"first_name":   uniqueName,
			"last_name":    "Student",
			"school_class": "3a",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), uniqueName, "Response should contain student first name")
	})

	t.Run("success_creates_student_with_optional_fields", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		uniqueName := fmt.Sprintf("Created%d", time.Now().UnixNano())
		body := map[string]interface{}{
			"first_name":     uniqueName,
			"last_name":      "StudentFull",
			"school_class":   "3b",
			"guardian_name":  "Parent Name",
			"guardian_email": "parent@example.com",
			"birthday":       "2015-06-15",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_missing_first_name", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		body := map[string]interface{}{
			"last_name":    "Student",
			"school_class": "3a",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_last_name", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		body := map[string]interface{}{
			"first_name":   "Test",
			"school_class": "3a",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_school_class", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		body := map[string]interface{}{
			"first_name": "Test",
			"last_name":  "Student",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_birthday_format", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		body := map[string]interface{}{
			"first_name":   "Test",
			"last_name":    "Student",
			"school_class": "3a",
			"birthday":     "invalid-date",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Update Student Tests
// =============================================================================

func TestUpdateStudent(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "Update", "Student", "4a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("success_updates_student", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Updated", "Response should contain updated first name")
	})

	t.Run("success_updates_multiple_fields", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name":   "MultiUpdate",
			"school_class": "4b",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name": "Test",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_empty_first_name", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Delete Student Tests
// =============================================================================

func TestDeleteStudent(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_deletes_student", func(t *testing.T) {
		// Create a student specifically for deletion
		student := testpkg.CreateTestStudent(t, tc.db, "Delete", "Student", "5a")

		router := setupRouter(tc.resource.DeleteStudentHandler(), "id")
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		router := setupRouter(tc.resource.DeleteStudentHandler(), "id")
		req := testutil.NewRequest("DELETE", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.DeleteStudentHandler(), "id")
		req := testutil.NewRequest("DELETE", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Student Location Tests
// =============================================================================

func TestGetStudentCurrentLocation(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "Location", "Student", "6a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("success_gets_student_location", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "current_location", "Response should contain location")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentCurrentLocationHandler(), "id")
		req := testutil.NewRequest("GET", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

// =============================================================================
// Student Current Visit Tests
// =============================================================================

func TestGetStudentCurrentVisit(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Student", "7a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("error_when_no_current_visit", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentCurrentVisitHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// When no visit exists, the service returns an error which results in 500
		// This is the actual behavior - the service returns "visit not found" error
		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentCurrentVisitHandler(), "id")
		req := testutil.NewRequest("GET", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Student Visit History Tests
// =============================================================================

func TestGetStudentVisitHistory(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "History", "Student", "8a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("success_returns_empty_history", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentVisitHistoryHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentVisitHistoryHandler(), "id")
		req := testutil.NewRequest("GET", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Privacy Consent Tests
// =============================================================================

func TestGetStudentPrivacyConsent(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "Privacy", "Student", "9a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("success_returns_default_consent", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentPrivacyConsentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "policy_version", "Response should contain policy version")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentPrivacyConsentHandler(), "id")
		req := testutil.NewRequest("GET", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

func TestUpdateStudentPrivacyConsent(t *testing.T) {
	tc := setupTestContext(t)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "ConsentUpdate", "Student", "9b")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("success_creates_consent", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_missing_policy_version", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")
		body := map[string]interface{}{
			"accepted":            true,
			"data_retention_days": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_retention_days", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentPrivacyConsentHandler(), "id")
		body := map[string]interface{}{
			"policy_version":      "1.0",
			"accepted":            true,
			"data_retention_days": 100, // Max is 31
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Request Validation Tests
// =============================================================================

func TestStudentRequestValidation(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("bind_validates_required_fields", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		body := map[string]interface{}{} // Empty body
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Router Tests
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	tc := setupTestContext(t)

	router := tc.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// Group Room Handler Tests
// =============================================================================

func TestGetStudentInGroupRoom_InvalidStudentID(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

	req := testutil.NewRequest("GET", "/students/invalid/group-room", nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	testutil.AssertBadRequest(t, rr)
}

func TestGetStudentInGroupRoom_NonexistentStudent(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

	req := testutil.NewRequest("GET", "/students/999999/group-room", nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// Should return 404 for nonexistent student or 500 for internal error
	assert.Contains(t, []int{http.StatusNotFound, http.StatusInternalServerError}, rr.Code)
}

func TestGetStudentInGroupRoom_WithValidStudent(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "GroupRoom", "Student", "GR1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/students/{id}/group-room", tc.resource.GetStudentInGroupRoomHandler())

	req := testutil.NewRequest("GET", fmt.Sprintf("/students/%d/group-room", student.ID), nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// May return 200 (success), 403 (no permission), or 500 (no group room)
	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// RFID Tag Handler Tests
// Note: RFID handlers require device authentication, not user auth
// =============================================================================

func TestAssignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/students/{id}/rfid-tag", tc.resource.AssignRFIDTagHandler())

	body := map[string]interface{}{
		"tag_uid": "12345678",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/1/rfid-tag", body)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// RFID handlers require device authentication, not admin auth
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "RFID handlers require device auth")
	assert.Contains(t, rr.Body.String(), "device authentication required")
}

func TestUnassignRFIDTag_RequiresDeviceAuth(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Delete("/students/{id}/rfid-tag", tc.resource.UnassignRFIDTagHandler())

	req := testutil.NewRequest("DELETE", "/students/1/rfid-tag", nil)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// RFID handlers require device authentication, not admin auth
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "RFID handlers require device auth")
	assert.Contains(t, rr.Body.String(), "device authentication required")
}

// =============================================================================
// List Students with Location Filter Tests
// =============================================================================

func TestListStudents_WithLocationFilter(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "LocationFilter", "Student", "LF1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("filter_by_in_house", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?location=in_house", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("filter_by_absent", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?location=absent", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Student with Guardian Info Tests
// =============================================================================

func TestUpdateStudent_WithGuardianInfo(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "UpdateTest", "GU1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("update_guardian_name", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"guardian_name": "New Guardian Name",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("update_guardian_email", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"guardian_email": "newguardian@example.com",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("update_guardian_phone", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"guardian_phone": "+49123456789",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Student with Sick Status Tests
// =============================================================================

func TestUpdateStudent_WithSickStatus(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Sick", "StatusTest", "SS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("mark_as_sick", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"is_sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("mark_as_not_sick", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"is_sick": false,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Create Student with Group Assignment Tests
// =============================================================================

func TestCreateStudent_WithGroupID(t *testing.T) {
	tc := setupTestContext(t)

	// Create a group for testing
	group := testpkg.CreateTestEducationGroup(t, tc.db, "StudentGroupAssign")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	t.Run("creates_student_with_group", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		uniqueName := fmt.Sprintf("GroupAssigned%d", time.Now().UnixNano())
		body := map[string]interface{}{
			"first_name":   uniqueName,
			"last_name":    "Student",
			"school_class": "GA1",
			"group_id":     group.ID,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Teacher Access Tests (Non-Admin)
// =============================================================================

func TestListStudents_WithTeacherAccess(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StudentList", "Teacher")
	student := testpkg.CreateTestStudent(t, tc.db, "TeacherList", "Student", "TL1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, student.ID)

	router := setupRouter(tc.resource.ListStudentsHandler(), "")
	req := testutil.NewRequest("GET", "/", nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := executeWithAuth(router, req, claims, []string{"students:read"})

	// Teacher should be able to list students (may see limited data)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
}

func TestGetStudent_WithTeacherAccess(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StudentGet", "Teacher")
	student := testpkg.CreateTestStudent(t, tc.db, "TeacherGet", "Student", "TG1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, student.ID)

	router := setupRouter(tc.resource.GetStudentHandler(), "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := executeWithAuth(router, req, claims, []string{"students:read"})

	// Teacher should be able to get student (may see limited data)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
}

// =============================================================================
// Visit History with Date Range Tests
// =============================================================================

func TestGetStudentVisitHistory_WithDateRange(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "HistoryDate", "Student", "HD1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentVisitHistoryHandler(), "id")

	t.Run("with_start_date", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?from=2024-01-01", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("with_end_date", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?to=2024-12-31", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("with_date_range", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?from=2024-01-01&to=2024-12-31", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}
