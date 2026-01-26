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
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
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

	t.Run("success_returns_schedules_and_exceptions_with_data", func(t *testing.T) {
		// Create a new student for this test
		studentWithData := testpkg.CreateTestStudent(t, tc.db, "PickupData", "Test", "PD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, studentWithData.ID)

		// Insert a pickup schedule directly into the database
		pickupTime := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)
		notes := "Mit Schwester"
		schedule := &scheduleModel.StudentPickupSchedule{
			StudentID:  studentWithData.ID,
			Weekday:    1, // Monday
			PickupTime: pickupTime,
			Notes:      &notes,
			CreatedBy:  1,
		}
		_, err := tc.db.NewInsert().Model(schedule).
			ModelTableExpr("schedule.student_pickup_schedules").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
				ModelTableExpr("schedule.student_pickup_schedules").
				Where("student_id = ?", studentWithData.ID).
				Exec(context.Background())
		}()

		// Insert a pickup exception directly into the database
		exceptionDate := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
		exceptionTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
		exception := &scheduleModel.StudentPickupException{
			StudentID:     studentWithData.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &exceptionTime,
			Reason:        "Arzttermin",
			CreatedBy:     1,
		}
		_, err = tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
				ModelTableExpr("schedule.student_pickup_exceptions").
				Where("student_id = ?", studentWithData.ID).
				Exec(context.Background())
		}()

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", studentWithData.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Verify schedules data
		assert.Contains(t, rr.Body.String(), "15:30", "Should contain pickup time")
		assert.Contains(t, rr.Body.String(), "Montag", "Should contain weekday name")
		assert.Contains(t, rr.Body.String(), "Mit Schwester", "Should contain notes")
		// Verify exceptions data
		assert.Contains(t, rr.Body.String(), "2026-02-15", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "12:00", "Should contain exception pickup time")
		assert.Contains(t, rr.Body.String(), "Arzttermin", "Should contain reason")
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

	t.Run("success_updates_schedules_as_teacher", func(t *testing.T) {
		// Create a student
		student := testpkg.CreateTestStudent(t, tc.db, "PickupSuccess", "Test", "PST1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create a teacher with account for auth
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "Teacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "pickup_time": "15:30"},
				{"weekday": 3, "pickup_time": "16:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "15:30", "Should contain first pickup time")
		assert.Contains(t, rr.Body.String(), "16:00", "Should contain second pickup time")

		// Cleanup created schedules
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
			ModelTableExpr("schedule.student_pickup_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("bad_request_empty_schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupEmpty", "Test", "PE1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupWeekday", "Test", "PW1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{
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

		body := map[string]any{
			"schedules": []map[string]any{
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

		body := map[string]any{
			"schedules": []map[string]any{
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

		body := map[string]any{
			"schedules": []map[string]any{
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

	t.Run("success_creates_exception_as_teacher", func(t *testing.T) {
		// Create a student
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionCreate", "Test", "ECT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create a teacher with account for auth
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Create", "ExcTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"exception_date": "2026-03-15",
			"pickup_time":    "12:00",
			"reason":         "Doctor appointment",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-15", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "12:00", "Should contain pickup time")
		assert.Contains(t, rr.Body.String(), "Doctor appointment", "Should contain reason")

		// Cleanup created exception
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("success_creates_exception_without_pickup_time_absent", func(t *testing.T) {
		// Create a student
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionAbsent", "Test", "EAT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create a teacher with account for auth
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Absent", "ExcTeacher")
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

		// Cleanup created exception
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

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

	// Note: nil/empty pickup_time is NOW VALID (for absent students).
	// Validation is tested in pickup_schedule_bind_test.go.
	// Integration tests for successful creation would require a full account+person setup.

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

	t.Run("success_updates_exception_as_teacher", func(t *testing.T) {
		// Create a student
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdate", "Test", "EUT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create a teacher with account for auth
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ExcTeacher2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		// Create an exception to update
		exceptionDate := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
		exceptionTime := time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC)
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &exceptionTime,
			Reason:        "Original reason",
			CreatedBy:     1,
		}
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
				ModelTableExpr("schedule.student_pickup_exceptions").
				Where("id = ?", exception.ID).
				Exec(context.Background())
		}()

		router := setupExceptionRouter(tc.resource.UpdateStudentPickupExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date": "2026-04-15",
			"pickup_time":    "11:00",
			"reason":         "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/%d", student.ID, exception.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "11:00", "Should contain updated pickup time")
		assert.Contains(t, rr.Body.String(), "Updated reason", "Should contain updated reason")
	})

	t.Run("forbidden_exception_belongs_to_different_student", func(t *testing.T) {
		// Create two students
		student1 := testpkg.CreateTestStudent(t, tc.db, "Student1", "Test", "ST1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "Student2", "Test", "ST2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		// Create exception for student2
		exceptionDate := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student2.ID, // Belongs to student2
			ExceptionDate: exceptionDate,
			Reason:        "Test reason",
			CreatedBy:     1,
		}
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
				ModelTableExpr("schedule.student_pickup_exceptions").
				Where("id = ?", exception.ID).
				Exec(context.Background())
		}()

		router := setupExceptionRouter(tc.resource.UpdateStudentPickupExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date": "2026-05-15",
			"reason":         "Updated",
		}
		// Try to update student2's exception using student1's URL
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/%d", student1.ID, exception.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertForbidden(t, rr)
	})

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

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdateNotFound", "Test", "EUNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupExceptionRouter(tc.resource.UpdateStudentPickupExceptionHandler(), "PUT")

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Updated reason",
		}
		// Use a valid but nonexistent exception ID
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/999999", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
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

	t.Run("success_deletes_exception_as_teacher", func(t *testing.T) {
		// Create a student
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDelete", "Test", "EDT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create exception to delete
		exceptionDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        "To be deleted",
			CreatedBy:     1,
		}
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		router := setupExceptionRouter(tc.resource.DeleteStudentPickupExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/%d", student.ID, exception.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("forbidden_delete_exception_belongs_to_different_student", func(t *testing.T) {
		// Create two students
		student1 := testpkg.CreateTestStudent(t, tc.db, "DeleteSt1", "Test", "DS1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "DeleteSt2", "Test", "DS2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		// Create exception for student2
		exceptionDate := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student2.ID, // Belongs to student2
			ExceptionDate: exceptionDate,
			Reason:        "Test reason",
			CreatedBy:     1,
		}
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
				ModelTableExpr("schedule.student_pickup_exceptions").
				Where("id = ?", exception.ID).
				Exec(context.Background())
		}()

		router := setupExceptionRouter(tc.resource.DeleteStudentPickupExceptionHandler(), "DELETE")

		// Try to delete student2's exception using student1's URL
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/%d", student1.ID, exception.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteInvalid", "Test", "EDI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupExceptionRouter(tc.resource.DeleteStudentPickupExceptionHandler(), "DELETE")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/invalid", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteNotFound", "Test", "EDNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupExceptionRouter(tc.resource.DeleteStudentPickupExceptionHandler(), "DELETE")

		// Use a valid but nonexistent exception ID
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/999999", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
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
// Get Bulk Pickup Times Tests
// =============================================================================

func TestGetBulkPickupTimes(t *testing.T) {
	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/", tc.resource.GetBulkPickupTimesHandler())

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
		student1 := testpkg.CreateTestStudent(t, tc.db, "BulkTest1", "Student", "BT1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "BulkTest2", "Student", "BT2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		body := map[string]any{
			"student_ids": []int64{student1.ID, student2.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_specific_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkDateTest", "Student", "BDT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"student_ids": []int64{student.ID},
			"date":        "2026-01-27",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_returns_empty_for_unauthorized_students", func(t *testing.T) {
		// Create a staff member who doesn't supervise any groups
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoGroups", "Staff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

		// Create a student in no particular group
		student := testpkg.CreateTestStudent(t, tc.db, "UnauthorizedTest", "Student", "UTS1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"student_ids": []int64{student.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := executeWithAuth(router, req, claims, []string{"students:read"})

		// Should return 200 OK with empty data (no authorized students)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "[]") // Empty array
	})

	t.Run("success_filters_nonexistent_student_ids", func(t *testing.T) {
		// Admin requests non-existent students - should still succeed with empty results
		body := map[string]any{
			"student_ids": []int64{999998, 999999}, // Non-existent IDs
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_returns_pickup_times_with_data", func(t *testing.T) {
		// Create a student with pickup schedule
		student := testpkg.CreateTestStudent(t, tc.db, "BulkWithData", "Test", "BWD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Insert a pickup schedule for Monday
		pickupTime := time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC)
		notes := "Regular pickup"
		schedule := &scheduleModel.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    1, // Monday
			PickupTime: pickupTime,
			Notes:      &notes,
			CreatedBy:  1,
		}
		_, err := tc.db.NewInsert().Model(schedule).
			ModelTableExpr("schedule.student_pickup_schedules").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
				ModelTableExpr("schedule.student_pickup_schedules").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		// Request for a Monday date
		body := map[string]any{
			"student_ids": []int64{student.ID},
			"date":        "2026-01-26", // Monday
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "14:30", "Should contain pickup time")
		assert.Contains(t, rr.Body.String(), "Montag", "Should contain weekday name")
	})

	t.Run("success_returns_exception_override", func(t *testing.T) {
		// Create student with both schedule and exception
		student := testpkg.CreateTestStudent(t, tc.db, "BulkException", "Test", "BEX1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Insert base schedule for Monday
		baseTime := time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
		schedule := &scheduleModel.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    1,
			PickupTime: baseTime,
			CreatedBy:  1,
		}
		_, err := tc.db.NewInsert().Model(schedule).
			ModelTableExpr("schedule.student_pickup_schedules").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
				ModelTableExpr("schedule.student_pickup_schedules").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		// Insert exception for specific date
		exceptionDate := time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC) // Monday
		exceptionTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &exceptionTime,
			Reason:        "Early pickup",
			CreatedBy:     1,
		}
		_, err = tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
				ModelTableExpr("schedule.student_pickup_exceptions").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		body := map[string]any{
			"student_ids": []int64{student.ID},
			"date":        "2026-01-26",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		// Exception should override base time
		assert.Contains(t, rr.Body.String(), "12:00", "Should contain exception pickup time")
		assert.Contains(t, rr.Body.String(), "is_exception", "Should indicate exception")
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
