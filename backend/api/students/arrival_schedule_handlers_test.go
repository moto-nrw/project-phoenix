package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Get Arrival Schedules Tests
// =============================================================================

func TestGetStudentArrivalSchedules(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "ArrivalGet", "Test", "AG1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentArrivalSchedulesHandler(), "id")

	t.Run("success_returns_empty_schedules", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "schedules")
		assert.Contains(t, rr.Body.String(), "exceptions")
		assert.Contains(t, rr.Body.String(), "notes")
	})

	t.Run("success_returns_schedules_with_data", func(t *testing.T) {
		studentWithData := testpkg.CreateTestStudent(t, tc.db, "ArrivalData", "Test", "AD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, studentWithData.ID)

		arrivalTime := time.Date(2000, 1, 1, 7, 45, 0, 0, time.UTC)
		notes := "Kommt mit Schwester"
		schedule := &scheduleModel.StudentArrivalSchedule{
			StudentID:       studentWithData.ID,
			Weekday:         1, // Monday
			ExpectedArrival: arrivalTime,
			Notes:           &notes,
			CreatedBy:       1,
		}
		schedule.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(schedule).
			ModelTableExpr("schedule.student_arrival_schedules").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
				ModelTableExpr("schedule.student_arrival_schedules").
				Where("student_id = ?", studentWithData.ID).
				Exec(context.Background())
		}()

		// Insert an arrival exception
		exceptionDate := time.Date(2026, 2, 15, 12, 0, 0, 0, timezone.Berlin)
		exceptionTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
		arztterminReason := "Arzttermin"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:       studentWithData.ID,
			ExceptionDate:   exceptionDate,
			ExpectedArrival: &exceptionTime,
			Reason:          &arztterminReason,
			CreatedBy:       1,
		}
		exception.SetTenantID(1)
		_, err = tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
				ModelTableExpr("schedule.student_arrival_exceptions").
				Where("student_id = ?", studentWithData.ID).
				Exec(context.Background())
		}()

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", studentWithData.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "07:45", "Should contain arrival time")
		assert.Contains(t, rr.Body.String(), "Montag", "Should contain weekday name")
		assert.Contains(t, rr.Body.String(), "Kommt mit Schwester", "Should contain notes")
		assert.Contains(t, rr.Body.String(), "2026-02-15", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "09:00", "Should contain exception arrival time")
		assert.Contains(t, rr.Body.String(), "Arzttermin", "Should contain reason")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden_without_read_access", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:read"})

		assert.Equal(t, http.StatusForbidden, rr.Code, "Non-supervisor should be forbidden. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Arrival Schedules Tests
// =============================================================================

func TestUpdateStudentArrivalSchedules(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouterWithMethods(tc.resource.UpdateStudentArrivalSchedulesHandler(), "id", []string{"PUT"})

	t.Run("success_updates_schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalSuccess", "Test", "AST1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ArrTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "07:45"},
				{"weekday": 3, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "07:45", "Should contain first arrival time")
		assert.Contains(t, rr.Body.String(), "08:00", "Should contain second arrival time")

		// Cleanup created schedules
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
			ModelTableExpr("schedule.student_arrival_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("bad_request_empty_schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalEmpty", "Test", "AE1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalWeekday", "Test", "AW1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 7, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_time_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalTime", "Test", "AT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "invalid"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_time", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalNoTime", "Test", "ANT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalForbidden", "Test", "AF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrUpdateStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Create Arrival Exception Tests
// =============================================================================

func TestCreateStudentArrivalException(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouterWithMethods(tc.resource.CreateStudentArrivalExceptionHandler(), "id", []string{"POST"})

	t.Run("success_creates_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcCreate", "Test", "AEC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Create", "ArrExcTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"exception_date":   "2026-03-15",
			"expected_arrival": "09:00",
			"reason":           "Doctor appointment",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-15", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "09:00", "Should contain arrival time")
		assert.Contains(t, rr.Body.String(), "Doctor appointment", "Should contain reason")

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("success_creates_absent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcAbsent", "Test", "AEA1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Absent", "ArrExcTeacher2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"exception_date": "2026-03-16",
			"reason":         "Student is sick",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-16", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "Student is sick", "Should contain reason")

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("bad_request_missing_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcNoDate", "Test", "AEND1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"expected_arrival": "09:00",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcBadDate", "Test", "AEBD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date":   "15-02-2026",
			"expected_arrival": "09:00",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_arrival_time_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcBadTime", "Test", "AEBT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "invalid",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_reason_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcLongReason", "Test", "AELR1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		longReason := make([]byte, 256)
		for i := range longReason {
			longReason[i] = 'a'
		}
		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           string(longReason),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcForbidden", "Test", "AEF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrExcStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Arrival Exception Tests
// =============================================================================

func TestUpdateStudentArrivalException(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_updates_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdate", "Test", "AEU1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ArrExcTeacher3")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		exceptionDate := time.Date(2026, 4, 15, 12, 0, 0, 0, timezone.Berlin)
		exceptionTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
		originalReason := "Original reason"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   exceptionDate,
			ExpectedArrival: &exceptionTime,
			Reason:          &originalReason,
			CreatedBy:       1,
		}
		exception.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
				ModelTableExpr("schedule.student_arrival_exceptions").
				Where("id = ?", exception.ID).
				Exec(context.Background())
		}()

		router := setupArrivalExceptionRouter(tc.resource.UpdateStudentArrivalExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date":   "2026-04-15",
			"expected_arrival": "09:30",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/%d", student.ID, exception.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "09:30", "Should contain updated arrival time")
		assert.Contains(t, rr.Body.String(), "Updated reason", "Should contain updated reason")
	})

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdateInvalid", "Test", "AEUI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalExceptionRouter(tc.resource.UpdateStudentArrivalExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/abc", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdateNF", "Test", "AEUNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalExceptionRouter(tc.resource.UpdateStudentArrivalExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/999999", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdateForbid", "Test", "AEUF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrUpdateExcStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := setupArrivalExceptionRouter(tc.resource.UpdateStudentArrivalExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/1", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Delete Arrival Exception Tests
// =============================================================================

func TestDeleteStudentArrivalException(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_deletes_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDelete", "Test", "AED1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		exceptionDate := time.Date(2026, 6, 15, 12, 0, 0, 0, timezone.Berlin)
		deleteReason := "To be deleted"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        &deleteReason,
			CreatedBy:     1,
		}
		exception.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		router := setupArrivalExceptionRouter(tc.resource.DeleteStudentArrivalExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/%d", student.ID, exception.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDeleteInvalid", "Test", "AEDI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalExceptionRouter(tc.resource.DeleteStudentArrivalExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/invalid", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDeleteNF", "Test", "AEDNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalExceptionRouter(tc.resource.DeleteStudentArrivalExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/999999", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDeleteForbid", "Test", "AEDF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrDeleteExcStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := setupArrivalExceptionRouter(tc.resource.DeleteStudentArrivalExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/1", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Create Arrival Note Tests
// =============================================================================

func TestCreateStudentArrivalNote(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouterWithMethods(tc.resource.CreateStudentArrivalNoteHandler(), "id", []string{"POST"})

	t.Run("success_creates_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteCreate", "Test", "ANC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Create", "ArrNoteTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "Arrives with school bus today",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-15", "Should contain note date")
		assert.Contains(t, rr.Body.String(), "Arrives with school bus today", "Should contain content")

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalNote)(nil)).
			ModelTableExpr("schedule.student_arrival_notes").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("bad_request_missing_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteNoDate", "Test", "ANND1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"content": "Test content",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "note_date is required")
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteBadDate", "Test", "ANBD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"note_date": "15-03-2026",
			"content":   "Test content",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "invalid note_date format")
	})

	t.Run("bad_request_missing_content", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteNoContent", "Test", "ANNC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content is required")
	})

	t.Run("bad_request_content_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteLongContent", "Test", "ANLC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		longContent := make([]byte, 501)
		for i := range longContent {
			longContent[i] = 'a'
		}
		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   string(longContent),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content cannot exceed 500 characters")
	})

	t.Run("forbidden_without_full_access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteForbidden", "Test", "ANF1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrNoteStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "Test note",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:write"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Arrival Note Tests
// =============================================================================

func TestUpdateStudentArrivalNote(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_updates_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteUpdate", "Test", "ANU1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ArrNoteTeacher2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		noteDate := time.Date(2026, 4, 15, 12, 0, 0, 0, timezone.Berlin)
		note := &scheduleModel.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  noteDate,
			Content:   "Original content",
			CreatedBy: 1,
		}
		note.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_arrival_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalNote)(nil)).
				ModelTableExpr("schedule.student_arrival_notes").
				Where("id = ?", note.ID).
				Exec(context.Background())
		}()

		router := setupArrivalNoteRouter(tc.resource.UpdateStudentArrivalNoteHandler(), "PUT")

		body := map[string]any{
			"note_date": "2026-04-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/%d", student.ID, note.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Updated content", "Should contain updated content")
	})

	t.Run("bad_request_invalid_note_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteUpdateInvalid", "Test", "ANUI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalNoteRouter(tc.resource.UpdateStudentArrivalNoteHandler(), "PUT")

		body := map[string]any{
			"note_date": "2026-02-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/abc", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteUpdateNF", "Test", "ANUNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalNoteRouter(tc.resource.UpdateStudentArrivalNoteHandler(), "PUT")

		body := map[string]any{
			"note_date": "2026-02-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/999999", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

// =============================================================================
// Delete Arrival Note Tests
// =============================================================================

func TestDeleteStudentArrivalNote(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_deletes_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteDelete", "Test", "AND1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		noteDate := time.Date(2026, 6, 15, 12, 0, 0, 0, timezone.Berlin)
		note := &scheduleModel.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  noteDate,
			Content:   "To be deleted",
			CreatedBy: 1,
		}
		note.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_arrival_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		router := setupArrivalNoteRouter(tc.resource.DeleteStudentArrivalNoteHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/%d", student.ID, note.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("bad_request_invalid_note_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteDeleteInvalid", "Test", "ANDI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalNoteRouter(tc.resource.DeleteStudentArrivalNoteHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/invalid", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteDeleteNF", "Test", "ANDNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupArrivalNoteRouter(tc.resource.DeleteStudentArrivalNoteHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/999999", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

// =============================================================================
// Bulk Upsert Arrival Schedules Tests
// =============================================================================

func TestBulkUpsertArrivalSchedules(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/", tc.resource.BulkUpsertArrivalSchedulesHandler())

	t.Run("bad_request_missing_school_class", func(t *testing.T) {
		body := map[string]any{
			"school_class": "",
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_empty_schedules", func(t *testing.T) {
		body := map[string]any{
			"school_class": "1a",
			"schedules":    []map[string]any{},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		body := map[string]any{
			"school_class": "1a",
			"schedules": []map[string]any{
				{"weekday": 7, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("success_with_valid_request", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkArrival", "Test", "BAR1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkArr", "Teacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"school_class": "BAR1",
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "07:45"},
				{"weekday": 3, "expected_arrival": "08:15"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
			ModelTableExpr("schedule.student_arrival_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})
}

// =============================================================================
// Get Bulk Arrival Times Tests
// =============================================================================

func TestGetBulkArrivalTimes(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/", tc.resource.GetBulkArrivalTimesHandler())

	t.Run("bad_request_empty_student_ids", func(t *testing.T) {
		body := map[string]any{
			"student_ids": []int64{},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_too_many_student_ids", func(t *testing.T) {
		ids := make([]int64, 501)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		body := map[string]any{
			"student_ids": ids,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		body := map[string]any{
			"student_ids": []int64{1, 2, 3},
			"date":        "27-01-2026",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("success_with_valid_request", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, tc.db, "BulkArrTime1", "Student", "BAT1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "BulkArrTime2", "Student", "BAT2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		body := map[string]any{
			"student_ids": []int64{student1.ID, student2.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_specific_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkArrDateTest", "Student", "BADT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"student_ids": []int64{student.ID},
			"date":        "2026-01-27",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// =============================================================================
// Arrival-specific Helper Functions
// =============================================================================

// setupArrivalExceptionRouter creates a router for arrival exception endpoints with nested URL params
func setupArrivalExceptionRouter(handler http.HandlerFunc, method string) chi.Router {
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

// setupArrivalNoteRouter creates a router for arrival note endpoints with nested URL params
func setupArrivalNoteRouter(handler http.HandlerFunc, method string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	switch method {
	case "GET":
		router.Get("/{id}/{noteId}", handler)
	case "POST":
		router.Post("/{id}/{noteId}", handler)
	case "PUT":
		router.Put("/{id}/{noteId}", handler)
	case "DELETE":
		router.Delete("/{id}/{noteId}", handler)
	}
	return router
}
