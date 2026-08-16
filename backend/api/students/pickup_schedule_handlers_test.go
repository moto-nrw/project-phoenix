package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	t.Run("success_returns_empty_schedules", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/pickup-schedules", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
			CreatedBy:  createStudentsAPITestStaffID(t, tc),
		}
		schedule.SetTenantID(1)
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
		exceptionDate := timezone.NewDate(2026, 2, 15) // Use noon to avoid day boundary issues
		exceptionTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
		arztterminReason := "Arzttermin"
		exception := &scheduleModel.StudentPickupException{
			StudentID:     studentWithData.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &exceptionTime,
			Reason:        &arztterminReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(1)
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

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/pickup-schedules", studentWithData.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewRequest("GET", "/999999/pickup-schedules", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden_for_account_without_staff_record", func(t *testing.T) {
		// Guests and guardians hold users:read but no staff record in the tenant.
		guest := testpkg.CreateTestAccount(t, tc.db, "pickup-schedule-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, guest.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/pickup-schedules", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("success_any_staff_can_read", func(t *testing.T) {
		// #2329: any verified staff member reads the pickup schedules of any
		// child in the tenant — supervision no longer narrows this.
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "AllStaff", "Reader")
		defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/pickup-schedules", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Any staff should read pickup schedules. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Pickup Schedules Tests
// =============================================================================

func TestUpdateStudentPickupSchedules(t *testing.T) {
	tc := setupTestContext(t)

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
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "15:30", "Should contain first pickup time")
		assert.Contains(t, rr.Body.String(), "16:00", "Should contain second pickup time")

		// Cleanup created schedules
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
			ModelTableExpr("schedule.student_pickup_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("success_empty_schedules_clears_existing", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupEmpty", "Test", "PE1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Clear", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		pickupTime := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)
		schedule := &scheduleModel.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    1,
			PickupTime: pickupTime,
			CreatedBy:  createStudentsAPITestStaffID(t, tc),
		}
		schedule.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(schedule).
			ModelTableExpr("schedule.student_pickup_schedules").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"schedules": []map[string]any{},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		count, err := tc.db.NewSelect().Model((*scheduleModel.StudentPickupSchedule)(nil)).
			ModelTableExpr("schedule.student_pickup_schedules").
			Where("student_id = ?", student.ID).
			Count(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupWeekday", "Test", "PW1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 7, "pickup_time": "15:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		// #2329: any verified staff member with users:update writes the plan;
		// what stays refused is an account without a staff record.
		student := testpkg.CreateTestStudent(t, tc.db, "PickupForbidden", "Test", "PF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "pickup-update-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, guest.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "pickup_time": "15:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("staff_outside_the_group_may_update", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupAllowed", "Test", "PA1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "UpdateStaff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "pickup_time": "15:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
			ModelTableExpr("schedule.student_pickup_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})
}

// =============================================================================
// Create Pickup Exception Tests
// =============================================================================

func TestCreateStudentPickupException(t *testing.T) {
	tc := setupTestContext(t)

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
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

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

	t.Run("create_response_includes_staff_source_without_refetch", func(t *testing.T) {
		// Locks the response contract: the immediate 201 body must report
		// source:staff, so a client never sees source:"" before a refetch. The
		// handler stamps the source explicitly (and bun also backfills the
		// column default via RETURNING) — this guards both against regressing.
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionSrc", "Test", "ESRC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Source", "ExcTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"exception_date": "2026-03-17",
			"pickup_time":    "14:30",
			"reason":         "Source check",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"source":"staff"`,
			"Immediate create response must carry source:staff, not the unset default. Body: %s", rr.Body.String())

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
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-16", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "Student is sick", "Should contain reason")

		// Cleanup created exception
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("create_over_guardian_row_reclaims_for_staff", func(t *testing.T) {
		// A guardian set the day from the parents portal, then staff create an
		// exception for the same day with a stale view (the modal didn't know a
		// row appeared). The unique (student_id, exception_date) index would make
		// a second insert fail with a 500; instead the create is folded into a
		// staff override that reclaims the day for staff.
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
		defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "CreateRace", "GuardianPickupExc")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		exceptionDate := timezone.NewDate(2026, 4, 17)
		originalReason := "Parent reason"
		guardian := &scheduleModel.StudentPickupException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			Reason:            &originalReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		guardian.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(guardian).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"exception_date": "2026-04-17",
			"pickup_time":    "13:45",
			"reason":         "Staff created over parent",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", chain.StudentID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created (not a 500 on the unique index). Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), scheduleModel.ExceptionSourceStaff)
		assert.Contains(t, rr.Body.String(), "13:45")

		// The day is reclaimed for staff: still a single row (no duplicate),
		// source flips, the guardian link is dropped.
		var rowCount int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &rowCount))
		assert.Equal(t, 1, rowCount, "create-over-existing must not insert a duplicate row")

		var source string
		var createdByGuardian *int64
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("source").
			ColumnExpr("created_by_guardian").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &source, &createdByGuardian))
		assert.Equal(t, scheduleModel.ExceptionSourceStaff, source)
		assert.Nil(t, createdByGuardian)
	})

	t.Run("create_over_staff_row_conflicts", func(t *testing.T) {
		// A different staff member set the day after this client loaded its
		// (empty) view, so the create races a STAFF-authored row. Unlike the
		// guardian case it must NOT be silently overwritten — refuse with a 409
		// so the client reloads and edits through the update path instead.
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
		defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "CreateRace", "StaffPickupExc")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		exceptionDate := timezone.NewDate(2026, 4, 18)
		originalReason := "Other staff reason"
		originalTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
		staffRow := &scheduleModel.StudentPickupException{
			StudentID:     chain.StudentID,
			ExceptionDate: exceptionDate,
			PickupTime:    &originalTime,
			Reason:        &originalReason,
			Source:        scheduleModel.ExceptionSourceStaff,
			CreatedBy:     teacher.StaffID,
		}
		staffRow.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(staffRow).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"exception_date": "2026-04-18",
			"pickup_time":    "13:45",
			"reason":         "Staff created over staff",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", chain.StudentID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusConflict, rr.Code, "staff-on-staff create must be a 409, not a silent overwrite. Body: %s", rr.Body.String())

		// The other staff member's row is untouched.
		var source string
		var createdBy int64
		var pickupTime *time.Time
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("source").
			ColumnExpr("created_by").
			ColumnExpr("pickup_time").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &source, &createdBy, &pickupTime))
		assert.Equal(t, scheduleModel.ExceptionSourceStaff, source)
		assert.Equal(t, teacher.StaffID, createdBy, "existing staff row must keep its author")
		require.NotNil(t, pickupTime)
		assert.Equal(t, "08:00", pickupTime.Format("15:04"), "existing staff time must be unchanged")
	})

	t.Run("bad_request_missing_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionNoDate", "Test", "END1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"pickup_time": "12:00",
			"reason":      "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("valid_request_without_reason", func(t *testing.T) {
		// Reason is now optional, omitting it should not cause a bad request.
		// Full creation still requires a valid account+person setup, so we only
		// verify the bind step doesn't reject the payload.
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionNoReason", "Test", "ENR1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Should NOT be a bad request (reason is optional). The actual status
		// depends on account setup, but it must not be 400.
		assert.NotEqual(t, http.StatusBadRequest, rr.Code,
			"Omitting reason should not cause bad request. Body: %s", rr.Body.String())
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
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionForbidden", "Test", "EF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "exceptionstaff-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, guest.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

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
		exceptionDate := timezone.NewDate(2026, 4, 15) // Use noon to avoid day boundary issues
		exceptionTime := time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC)
		originalReason := "Original reason"
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &exceptionTime,
			Reason:        &originalReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(1)
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

		body := map[string]any{
			"exception_date": "2026-04-15",
			"pickup_time":    "11:00",
			"reason":         "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-exceptions/%d", student.ID, exception.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "11:00", "Should contain updated pickup time")
		assert.Contains(t, rr.Body.String(), "Updated reason", "Should contain updated reason")
	})

	t.Run("success_reclaims_guardian_exception_for_staff", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
		defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "GuardianPickupExc")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		exceptionDate := timezone.NewDate(2026, 4, 16)
		originalReason := "Parent reason"
		exception := &scheduleModel.StudentPickupException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			Reason:            &originalReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		exception.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"exception_date": "2026-04-16",
			"pickup_time":    "13:45",
			"reason":         "Staff adjusted time",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-exceptions/%d", chain.StudentID, exception.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), scheduleModel.ExceptionSourceStaff)
		assert.Contains(t, rr.Body.String(), "13:45")

		// A staff edit reclaims the parent-authored day: source flips to staff,
		// the editing staff becomes the author, and the guardian link is dropped.
		var source string
		var createdBy *int64
		var createdByGuardian *int64
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("source").
			ColumnExpr("created_by").
			ColumnExpr("created_by_guardian").
			Where("id = ?", exception.ID).
			Scan(context.Background(), &source, &createdBy, &createdByGuardian))
		assert.Equal(t, scheduleModel.ExceptionSourceStaff, source)
		require.NotNil(t, createdBy)
		assert.Positive(t, *createdBy)
		assert.Nil(t, createdByGuardian)
	})

	t.Run("forbidden_exception_belongs_to_different_student", func(t *testing.T) {
		// Create two students
		student1 := testpkg.CreateTestStudent(t, tc.db, "Student1", "Test", "ST1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "Student2", "Test", "ST2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		// Create exception for student2
		exceptionDate := timezone.NewDate(2026, 5, 15) // Use noon to avoid day boundary issues
		testReason := "Test reason"
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student2.ID, // Belongs to student2
			ExceptionDate: exceptionDate,
			Reason:        &testReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(1)
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

		body := map[string]any{
			"exception_date": "2026-05-15",
			"reason":         "Updated",
		}
		// Try to update student2's exception using student1's URL
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-exceptions/%d", student1.ID, exception.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdateInvalid", "Test", "EUI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-exceptions/abc", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdateNotFound", "Test", "EUNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Updated reason",
		}
		// Use a valid but nonexistent exception ID
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-exceptions/999999", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionUpdateForbidden", "Test", "EUF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "updateexcstaff-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, guest.ID)

		body := map[string]any{
			"exception_date": "2026-02-15",
			"pickup_time":    "12:00",
			"reason":         "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-exceptions/1", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

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
		exceptionDate := timezone.NewDate(2026, 6, 15) // Use noon to avoid day boundary issues
		deleteReason := "To be deleted"
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        &deleteReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-exceptions/%d", student.ID, exception.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("success_deletes_guardian_exception", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
		defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Delete", "GuardianPickupExc")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		exceptionDate := timezone.NewDate(2026, 6, 16)
		pickupTime, err := time.Parse("2006-01-02 15:04", "2000-01-01 15:30")
		require.NoError(t, err)
		originalReason := "Parent pickup delete reason"
		exception := &scheduleModel.StudentPickupException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			PickupTime:        &pickupTime,
			Reason:            &originalReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		exception.SetTenantID(chain.TenantID)
		_, err = tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-exceptions/%d", chain.StudentID, exception.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		var remaining int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("COUNT(*)").
			Where("id = ?", exception.ID).
			Scan(context.Background(), &remaining))
		assert.Zero(t, remaining)
	})

	t.Run("forbidden_delete_exception_belongs_to_different_student", func(t *testing.T) {
		// Create two students
		student1 := testpkg.CreateTestStudent(t, tc.db, "DeleteSt1", "Test", "DS1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "DeleteSt2", "Test", "DS2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		// Create exception for student2
		exceptionDate := timezone.NewDate(2026, 7, 15) // Use noon to avoid day boundary issues
		deleteTestReason := "Test reason"
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student2.ID, // Belongs to student2
			ExceptionDate: exceptionDate,
			Reason:        &deleteTestReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(1)
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

		// Try to delete student2's exception using student1's URL
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-exceptions/%d", student1.ID, exception.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteInvalid", "Test", "EDI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-exceptions/invalid", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteNotFound", "Test", "EDNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Use a valid but nonexistent exception ID
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-exceptions/999999", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ExceptionDeleteForbidden", "Test", "EDF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "deleteexcstaff-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, guest.ID)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-exceptions/1", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Get Bulk Pickup Times Tests
// =============================================================================

func TestGetBulkPickupTimes(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("bad_request_empty_student_ids", func(t *testing.T) {
		body := map[string]any{
			"student_ids": []int64{},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		body := map[string]any{
			"student_ids": []int64{1, 2, 3},
			"date":        "27-01-2026",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("success_with_valid_request", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, tc.db, "BulkTest1", "Student", "BT1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "BulkTest2", "Student", "BT2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		body := map[string]any{
			"student_ids": []int64{student1.ID, student2.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_specific_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkDateTest", "Student", "BDT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"student_ids": []int64{student.ID},
			"date":        "2026-01-27",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_returns_empty_for_accounts_without_staff_record", func(t *testing.T) {
		// #2329: the bulk filter answers "every requested child" for admins and
		// verified staff and "none" for everyone else — a guest/guardian account
		// holding users:read gets an empty result rather than a 403.
		guest := testpkg.CreateTestAccount(t, tc.db, "bulk-pickup-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, guest.ID)

		student := testpkg.CreateTestStudent(t, tc.db, "UnauthorizedTest", "Student", "UTS1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"student_ids": []int64{student.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		// Should return 200 OK with empty data (no authorized students)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "[]") // Empty array
	})

	t.Run("success_returns_data_for_staff_without_supervision", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoGroups", "Staff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

		student := testpkg.CreateTestStudent(t, tc.db, "BulkAllowed", "Student", "BAS1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"student_ids": []int64{student.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("success_filters_nonexistent_student_ids", func(t *testing.T) {
		// Admin requests non-existent students - should still succeed with empty results
		body := map[string]any{
			"student_ids": []int64{999998, 999999}, // Non-existent IDs
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
			CreatedBy:  createStudentsAPITestStaffID(t, tc),
		}
		schedule.SetTenantID(1)
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
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
			CreatedBy:  createStudentsAPITestStaffID(t, tc),
		}
		schedule.SetTenantID(1)
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
		exceptionDate := timezone.NewDate(2026, 1, 26) // Monday, noon to avoid day boundary issues
		exceptionTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
		earlyPickupReason := "Early pickup"
		exception := &scheduleModel.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &exceptionTime,
			Reason:        &earlyPickupReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(1)
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
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		// Exception should override base time
		assert.Contains(t, rr.Body.String(), "12:00", "Should contain exception pickup time")
		assert.Contains(t, rr.Body.String(), "is_exception", "Should indicate exception")
	})
}

func TestBulkUpsertPickupSchedules(t *testing.T) {
	tc := setupTestContext(t)
	student1 := testpkg.CreateTestStudent(t, tc.db, "BulkPickupAPI1", "Student", "BPA1")
	student2 := testpkg.CreateTestStudent(t, tc.db, "BulkPickupAPI2", "Student", "BPA2")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkPickupAPI", "Teacher")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

	body := map[string]any{
		"student_ids": []int64{student1.ID, student2.ID},
		"schedules":   []map[string]any{{"weekday": 3, "pickup_time": "16:20"}},
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-schedules/bulk", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"students_affected":2`)
}

// =============================================================================
// Create Pickup Note Tests
// =============================================================================

func TestCreateStudentPickupNote(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_creates_note_as_teacher", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteCreate", "Test", "NCT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Create", "NoteTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "Please call before pickup today",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-15", "Should contain note date")
		assert.Contains(t, rr.Body.String(), "Please call before pickup today", "Should contain content")

		// Cleanup created note
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
			ModelTableExpr("schedule.student_pickup_notes").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("bad_request_missing_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteNoDate", "Test", "NND1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"content": "Test content",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "note_date is required")
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteBadDate", "Test", "NBD1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"note_date": "15-03-2026",
			"content":   "Test content",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "invalid note_date format")
	})

	t.Run("bad_request_missing_content", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteNoContent", "Test", "NNC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content is required")
	})

	t.Run("bad_request_content_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteLongContent", "Test", "NLC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		longContent := make([]byte, 501)
		for i := range longContent {
			longContent[i] = 'a'
		}
		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   string(longContent),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content cannot exceed 500 characters")
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteForbidden", "Test", "NF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "notestaff-guest@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, guest.ID)

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "Test note",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Pickup Note Tests
// =============================================================================

func TestUpdateStudentPickupNote(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_updates_note_as_teacher", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteUpdate", "Test", "NUT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "NoteTeacher2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		// Create a note to update
		noteDate := timezone.NewDate(2026, 4, 15)
		originalContent := "Original content"
		note := &scheduleModel.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  noteDate,
			Content:   originalContent,
			CreatedBy: createStudentsAPITestStaffID(t, tc),
		}
		note.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_pickup_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
				ModelTableExpr("schedule.student_pickup_notes").
				Where("id = ?", note.ID).
				Exec(context.Background())
		}()

		body := map[string]any{
			"note_date": "2026-04-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes/%d", student.ID, note.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Updated content", "Should contain updated content")
	})

	t.Run("forbidden_note_belongs_to_different_student", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, tc.db, "Student1Note", "Test", "ST1N")
		student2 := testpkg.CreateTestStudent(t, tc.db, "Student2Note", "Test", "ST2N")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		// Create note for student2
		noteDate := timezone.NewDate(2026, 5, 15)
		testContent := "Test content"
		note := &scheduleModel.StudentPickupNote{
			StudentID: student2.ID, // Belongs to student2
			NoteDate:  noteDate,
			Content:   testContent,
			CreatedBy: createStudentsAPITestStaffID(t, tc),
		}
		note.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_pickup_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
				ModelTableExpr("schedule.student_pickup_notes").
				Where("id = ?", note.ID).
				Exec(context.Background())
		}()

		body := map[string]any{
			"note_date": "2026-05-15",
			"content":   "Updated",
		}
		// Try to update student2's note using student1's URL
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes/%d", student1.ID, note.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("bad_request_invalid_note_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteUpdateInvalid", "Test", "NUI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"note_date": "2026-02-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes/abc", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteUpdateNotFound", "Test", "NUNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		body := map[string]any{
			"note_date": "2026-02-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes/999999", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

// =============================================================================
// Delete Pickup Note Tests
// =============================================================================

func TestDeleteStudentPickupNote(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("success_deletes_note_as_teacher", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteDelete", "Test", "NDT1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Create note to delete
		noteDate := timezone.NewDate(2026, 6, 15)
		deleteContent := "To be deleted"
		note := &scheduleModel.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  noteDate,
			Content:   deleteContent,
			CreatedBy: createStudentsAPITestStaffID(t, tc),
		}
		note.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_pickup_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-notes/%d", student.ID, note.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("forbidden_delete_note_belongs_to_different_student", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, tc.db, "DeleteSt1Note", "Test", "DS1N")
		student2 := testpkg.CreateTestStudent(t, tc.db, "DeleteSt2Note", "Test", "DS2N")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

		// Create note for student2
		noteDate := timezone.NewDate(2026, 7, 15)
		deleteTestContent := "Test content"
		note := &scheduleModel.StudentPickupNote{
			StudentID: student2.ID, // Belongs to student2
			NoteDate:  noteDate,
			Content:   deleteTestContent,
			CreatedBy: createStudentsAPITestStaffID(t, tc),
		}
		note.SetTenantID(1)
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_pickup_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
				ModelTableExpr("schedule.student_pickup_notes").
				Where("id = ?", note.ID).
				Exec(context.Background())
		}()

		// Try to delete student2's note using student1's URL
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-notes/%d", student1.ID, note.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("bad_request_invalid_note_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteDeleteInvalid", "Test", "NDI1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-notes/invalid", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "NoteDeleteNotFound", "Test", "NDNF1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/pickup-notes/999999", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}
