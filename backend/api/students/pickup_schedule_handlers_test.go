package students_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Get Pickup Schedules Tests
// =============================================================================

func TestGetStudentPickupSchedules(t *testing.T) {
	tc := setupTestContext(t)

	// Create student for tests
	student := testpkg.CreateTestStudent(t, tc.db, "PickupGet", "Test", "PG1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentPickupSchedulesHandler(), "id")

	t.Run("success_returns_empty_schedules", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "schedules")
		assert.Contains(t, rr.Body.String(), "exceptions")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "Staff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:read"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Pickup Schedules Tests
// =============================================================================

func TestUpdateStudentPickupSchedules(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouterWithMethods(tc.resource.UpdateStudentPickupSchedulesHandler(), "id", []string{"PUT"})

	t.Run("bad_request_empty_schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupEmpty", "Test", "PE1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]interface{}{
			"schedules": []map[string]interface{}{},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupWeekday", "Test", "PW1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]interface{}{
			"schedules": []map[string]interface{}{
				{"weekday": 7, "pickup_time": "15:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_weekday_zero", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupWeekdayZero", "Test", "PW0")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]interface{}{
			"schedules": []map[string]interface{}{
				{"weekday": 0, "pickup_time": "15:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_time_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupTime", "Test", "PT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]interface{}{
			"schedules": []map[string]interface{}{
				{"weekday": 1, "pickup_time": "invalid"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_time", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupNoTime", "Test", "PNT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]interface{}{
			"schedules": []map[string]interface{}{
				{"weekday": 1},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_notes_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupNotes", "Test", "PN1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		longNotes := make([]byte, 501)
		for i := range longNotes {
			longNotes[i] = 'a'
		}
		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "pickup_time": "15:30", "notes": string(longNotes)},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupForbidden", "Test", "PF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "UpdateStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "pickup_time": "15:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Create Pickup Exception Tests
// =============================================================================

func TestCreateStudentPickupException(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouterWithMethods(tc.resource.CreateStudentPickupExceptionHandler(), "id", []string{"POST"})

	t.Run("bad_request_missing_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionNoDate", "Test", "END1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"pickup_time": "12:00",
			"reason":      "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionBadDate", "Test", "EBD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "15-02-2026",
			"pickup_time":    "12:00",
			"reason":         "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_pickup_time", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionNoTime", "Test", "ENT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"reason":         "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_empty_pickup_time", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionEmptyTime", "Test", "EET1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "",
			"reason":         "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_pickup_time_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionBadTime", "Test", "EBT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "invalid",
			"reason":         "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_reason", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionNoReason", "Test", "ENR1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_reason_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionLongReason", "Test", "ELR1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		longReason := make([]byte, 256)
		for i := range longReason {
			longReason[i] = 'a'
		}
		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         string(longReason),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionForbidden", "Test", "EF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ExceptionStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Pickup Exception Tests
// =============================================================================

func TestUpdateStudentPickupException(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdateInvalid", "Test", "EUI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupExceptionRouter(tc.resource.UpdateStudentPickupExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/abc", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdateForbidden", "Test", "EUF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "UpdateExcStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := setupExceptionRouter(tc.resource.UpdateStudentPickupExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/1", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Delete Pickup Exception Tests
// =============================================================================

func TestDeleteStudentPickupException(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteInvalid", "Test", "EDI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupExceptionRouter(tc.resource.DeleteStudentPickupExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/invalid", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteForbidden", "Test", "EDF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "DeleteExcStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := setupExceptionRouter(tc.resource.DeleteStudentPickupExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/1", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Helper functions
// =============================================================================

// setupRouterWithMethods creates a router that only handles specific HTTP methods
func setupRouterWithMethods(handler http.HandlerFunc, urlParam string, methods []string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	for _, method := range methods {
		switch method {
		case "GET":
			router.Get(fmt.Sprintf("/{%s}", urlParam), handler)
		case "POST":
			router.Post(fmt.Sprintf("/{%s}", urlParam), handler)
		case "PUT":
			router.Put(fmt.Sprintf("/{%s}", urlParam), handler)
		case "DELETE":
			router.Delete(fmt.Sprintf("/{%s}", urlParam), handler)
		}
	}
	return router
}

// setupExceptionRouter creates a router for exception endpoints with nested URL params
func setupExceptionRouter(handler http.HandlerFunc, method string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	switch method {
	case "GET":
		router.Get("/{id}/{exceptionId}", handler)
	case "POST":
		router.Post("/{id}/{exceptionId}", handler)
	case "PUT":
		router.Put("/{id}/{exceptionId}", handler)
	case "DELETE":
		router.Delete("/{id}/{exceptionId}", handler)
	}
	return router
}
