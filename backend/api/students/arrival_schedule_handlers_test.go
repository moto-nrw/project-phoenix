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
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func createStudentsAPITestStaffID(t *testing.T, tc *testContext) int64 {
	t.Helper()

	staff := testpkg.CreateTestStaff(t, tc.db, "Arrival", fmt.Sprintf("Creator-%d", time.Now().UnixNano()))

	return staff.ID
}

// =============================================================================
// Get Arrival Schedules Tests
// =============================================================================

func TestGetStudentArrivalSchedules(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "ArrivalGet", "Test", "AG1")

	t.Run("success_returns_empty_schedules", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/arrival-schedules", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "schedules")
		assert.Contains(t, rr.Body.String(), "exceptions")
		assert.Contains(t, rr.Body.String(), "notes")
	})

	t.Run("success_returns_schedules_with_data", func(t *testing.T) {
		studentWithData := testpkg.CreateTestStudent(t, tc.db, "ArrivalData", "Test", "AD1")

		arrivalTime := time.Date(2000, 1, 1, 7, 45, 0, 0, time.UTC)
		notes := "Kommt mit Schwester"
		schedule := &scheduleModel.StudentArrivalSchedule{
			StudentID:       studentWithData.ID,
			Weekday:         1, // Monday
			ExpectedArrival: arrivalTime,
			Notes:           &notes,
			CreatedBy:       createStudentsAPITestStaffID(t, tc),
		}
		schedule.SetTenantID(testpkg.Tenant(t))
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
		exceptionDate := timezone.NewDate(2026, 2, 15)
		exceptionTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
		arztterminReason := "Arzttermin"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:       studentWithData.ID,
			ExceptionDate:   exceptionDate,
			ExpectedArrival: &exceptionTime,
			Reason:          &arztterminReason,
			CreatedBy:       createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(testpkg.Tenant(t))
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

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/arrival-schedules", studentWithData.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "07:45", "Should contain arrival time")
		assert.Contains(t, rr.Body.String(), "Montag", "Should contain weekday name")
		assert.Contains(t, rr.Body.String(), "Kommt mit Schwester", "Should contain notes")
		assert.Contains(t, rr.Body.String(), "2026-02-15", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "09:00", "Should contain exception arrival time")
		assert.Contains(t, rr.Body.String(), "Arzttermin", "Should contain reason")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999/arrival-schedules", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("rejects_date_ranges_longer_than_one_week", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/arrival-schedules?date=2026-08-17&to=2026-08-24", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "at most 7 days")
	})

	t.Run("forbidden_without_read_access", func(t *testing.T) {
		// #2329: every verified staff member reads the care plan, so the
		// remaining denial is an account without a staff record (guest,
		// guardian) that merely holds users:read.
		guest := testpkg.CreateTestAccount(t, tc.db, "arrival-schedule-guest@example.com")

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/arrival-schedules", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("staff_outside_the_group_may_read", func(t *testing.T) {
		_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoAccess", "ArrStaff")

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/arrival-schedules", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Arrival Schedules Tests
// =============================================================================

func TestUpdateStudentArrivalSchedules(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_updates_schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalSuccess", "Test", "AST1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ArrTeacher")

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "07:45"},
				{"weekday": 3, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "07:45", "Should contain first arrival time")
		assert.Contains(t, rr.Body.String(), "08:00", "Should contain second arrival time")

		// Cleanup created schedules
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
			ModelTableExpr("schedule.student_arrival_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("success_empty_schedules_clears_existing", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalEmpty", "Test", "AE1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Clear", "ArrTeacher")

		arrivalTime := time.Date(2000, 1, 1, 7, 45, 0, 0, time.UTC)
		schedule := &scheduleModel.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         1,
			ExpectedArrival: arrivalTime,
			CreatedBy:       createStudentsAPITestStaffID(t, tc),
		}
		schedule.SetTenantID(testpkg.Tenant(t))
		_, err := tc.db.NewInsert().Model(schedule).
			ModelTableExpr("schedule.student_arrival_schedules").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"schedules": []map[string]any{},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		count, err := tc.db.NewSelect().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
			ModelTableExpr("schedule.student_arrival_schedules").
			Where("student_id = ?", student.ID).
			Count(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalWeekday", "Test", "AW1")

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 7, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_empty_time_without_class_selector", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkEmptyTime", "Test", "BET1")
		body := map[string]any{
			"student_ids": []int64{student.ID},
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": ""},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_time_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalTime", "Test", "AT1")

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "invalid"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	// Business rule changed with #2414 / ADR 0005: an entry without a time is
	// not an incomplete request, it is a care day whose time comes from the
	// child's class timetable. What must not exist is a time on a day without
	// care, and that is expressed by the absence of an entry.
	t.Run("accepts_a_care_day_without_its_own_time", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalNoTime", "Test", "ANT1")
		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "NoTime", "ArrTeacher")

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		stored, err := tc.resource.ArrivalScheduleService.GetStudentArrivalSchedules(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		require.Len(t, stored, 1)
		assert.True(t, stored[0].InheritsClassTime(),
			"no class time is maintained, so the care day carries none either")
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrivalForbidden", "Test", "AF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "arrupdatestaff-guest@example.com")

		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Create Arrival Exception Tests
// =============================================================================

func TestCreateStudentArrivalException(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_creates_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcCreate", "Test", "AEC1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Create", "ArrExcTeacher")

		body := map[string]any{
			"exception_date":   "2026-03-15",
			"expected_arrival": "09:00",
			"reason":           "Doctor appointment",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

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

	t.Run("create_response_includes_staff_source_without_refetch", func(t *testing.T) {
		// Locks the response contract: the immediate 201 body must report
		// source:staff, so a client never sees source:"" before a refetch. The
		// handler stamps the source explicitly (and bun also backfills the
		// column default via RETURNING) — this guards both against regressing.
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcSrc", "Test", "AESRC1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Source", "ArrExcTeacher")

		body := map[string]any{
			"exception_date":   "2026-03-17",
			"expected_arrival": "08:15",
			"reason":           "Source check",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"source":"staff"`,
			"Immediate create response must carry source:staff, not the unset default. Body: %s", rr.Body.String())

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("success_creates_absent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcAbsent", "Test", "AEA1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Absent", "ArrExcTeacher2")

		body := map[string]any{
			"exception_date": "2026-03-16",
			"reason":         "Student is sick",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "2026-03-16", "Should contain exception date")
		assert.Contains(t, rr.Body.String(), "Student is sick", "Should contain reason")

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("create_over_guardian_row_reclaims_for_staff", func(t *testing.T) {
		// Same race as the pickup case: a guardian set the day from the portal,
		// then staff create over a stale view. The create must fold into a staff
		// override (reclaim) instead of colliding with the unique index.
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "CreateRace", "GuardianArrivalExc")

		exceptionDate := timezone.NewDate(2026, 4, 17)
		originalReason := "Parent arrival reason"
		guardian := &scheduleModel.StudentArrivalException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			Reason:            &originalReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		guardian.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(guardian).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"exception_date":   "2026-04-17",
			"expected_arrival": "09:45",
			"reason":           "Staff created over parent",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", chain.StudentID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created (not a 500 on the unique index). Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), scheduleModel.ExceptionSourceStaff)
		assert.Contains(t, rr.Body.String(), "09:45")

		var rowCount int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &rowCount))
		assert.Equal(t, 1, rowCount, "create-over-existing must not insert a duplicate row")

		var source string
		var createdByGuardian *int64
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
			ColumnExpr("source").
			ColumnExpr("created_by_guardian").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &source, &createdByGuardian))
		assert.Equal(t, scheduleModel.ExceptionSourceStaff, source)
		assert.Nil(t, createdByGuardian)
	})

	t.Run("create_over_staff_row_conflicts", func(t *testing.T) {
		// A different staff member set the arrival after this client loaded its
		// (empty) view. A STAFF-authored row must NOT be silently overwritten —
		// refuse with a 409 so the client reloads and edits through the update path.
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)

		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "CreateRace", "StaffArrivalExc")

		exceptionDate := timezone.NewDate(2026, 4, 18)
		originalReason := "Other staff arrival reason"
		originalTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
		staffRow := &scheduleModel.StudentArrivalException{
			StudentID:       chain.StudentID,
			ExceptionDate:   exceptionDate,
			ExpectedArrival: &originalTime,
			Reason:          &originalReason,
			Source:          scheduleModel.ExceptionSourceStaff,
			CreatedBy:       teacher.StaffID,
		}
		staffRow.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(staffRow).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"exception_date":   "2026-04-18",
			"expected_arrival": "09:45",
			"reason":           "Staff created over staff",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", chain.StudentID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusConflict, rr.Code, "staff-on-staff create must be a 409, not a silent overwrite. Body: %s", rr.Body.String())

		var source string
		var createdBy int64
		var arrival *time.Time
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
			ColumnExpr("source").
			ColumnExpr("created_by").
			ColumnExpr("expected_arrival").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &source, &createdBy, &arrival))
		assert.Equal(t, scheduleModel.ExceptionSourceStaff, source)
		assert.Equal(t, teacher.StaffID, createdBy, "existing staff row must keep its author")
		require.NotNil(t, arrival)
		assert.Equal(t, "08:00", arrival.Format("15:04"), "existing staff time must be unchanged")
	})

	t.Run("bad_request_missing_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcNoDate", "Test", "AEND1")

		body := map[string]any{
			"expected_arrival": "09:00",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcBadDate", "Test", "AEBD1")

		body := map[string]any{
			"exception_date":   "15-02-2026",
			"expected_arrival": "09:00",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_arrival_time_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcBadTime", "Test", "AEBT1")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "invalid",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_reason_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcLongReason", "Test", "AELR1")

		longReason := make([]byte, 256)
		for i := range longReason {
			longReason[i] = 'a'
		}
		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           string(longReason),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcForbidden", "Test", "AEF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "arrexcstaff-guest@example.com")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Test reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Arrival Exception Tests
// =============================================================================

func TestUpdateStudentArrivalException(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_updates_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdate", "Test", "AEU1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ArrExcTeacher3")

		exceptionDate := timezone.NewDate(2026, 4, 15)
		exceptionTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
		originalReason := "Original reason"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   exceptionDate,
			ExpectedArrival: &exceptionTime,
			Reason:          &originalReason,
			CreatedBy:       createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(testpkg.Tenant(t))
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

		body := map[string]any{
			"exception_date":   "2026-04-15",
			"expected_arrival": "09:30",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-exceptions/%d", student.ID, exception.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "09:30", "Should contain updated arrival time")
		assert.Contains(t, rr.Body.String(), "Updated reason", "Should contain updated reason")
	})

	t.Run("success_reclaims_guardian_exception_for_staff", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "GuardianArrivalExc")

		exceptionDate := timezone.NewDate(2026, 4, 16)
		originalReason := "Parent arrival reason"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			Reason:            &originalReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		exception.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"exception_date":   "2026-04-16",
			"expected_arrival": "09:45",
			"reason":           "Staff adjusted arrival",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-exceptions/%d", chain.StudentID, exception.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), scheduleModel.ExceptionSourceStaff)
		assert.Contains(t, rr.Body.String(), "09:45")

		// A staff edit reclaims the parent-authored day: source flips to staff,
		// the editing staff becomes the author, and the guardian link is dropped.
		var source string
		var createdBy *int64
		var createdByGuardian *int64
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
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

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdateInvalid", "Test", "AEUI1")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-exceptions/abc", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdateNF", "Test", "AEUNF1")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-exceptions/999999", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcUpdateForbid", "Test", "AEUF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "arrupdateexcstaff-guest@example.com")

		body := map[string]any{
			"exception_date":   "2026-02-15",
			"expected_arrival": "09:00",
			"reason":           "Updated reason",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-exceptions/1", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Delete Arrival Exception Tests
// =============================================================================

func TestDeleteStudentArrivalException(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_deletes_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDelete", "Test", "AED1")

		exceptionDate := timezone.NewDate(2026, 6, 15)
		deleteReason := "To be deleted"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        &deleteReason,
			CreatedBy:     createStudentsAPITestStaffID(t, tc),
		}
		exception.SetTenantID(testpkg.Tenant(t))
		_, err := tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-exceptions/%d", student.ID, exception.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("success_deletes_guardian_exception", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Delete", "GuardianArrivalExc")

		exceptionDate := timezone.NewDate(2026, 6, 16)
		arrivalTime, err := time.Parse("2006-01-02 15:04", "2000-01-01 08:15")
		require.NoError(t, err)
		originalReason := "Parent arrival delete reason"
		exception := &scheduleModel.StudentArrivalException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			ExpectedArrival:   &arrivalTime,
			Reason:            &originalReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		exception.SetTenantID(chain.TenantID)
		_, err = tc.db.NewInsert().Model(exception).
			ModelTableExpr("schedule.student_arrival_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-exceptions/%d", chain.StudentID, exception.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		var remaining int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
			ColumnExpr("COUNT(*)").
			Where("id = ?", exception.ID).
			Scan(context.Background(), &remaining))
		assert.Zero(t, remaining)
	})

	t.Run("bad_request_invalid_exception_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDeleteInvalid", "Test", "AEDI1")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-exceptions/invalid", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDeleteNF", "Test", "AEDNF1")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-exceptions/999999", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrExcDeleteForbid", "Test", "AEDF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "arrdeleteexcstaff-guest@example.com")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-exceptions/1", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Create Arrival Note Tests
// =============================================================================

func TestCreateStudentArrivalNote(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_creates_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteCreate", "Test", "ANC1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Create", "ArrNoteTeacher")

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "Arrives with school bus today",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

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

		body := map[string]any{
			"content": "Test content",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "note_date is required")
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteBadDate", "Test", "ANBD1")

		body := map[string]any{
			"note_date": "15-03-2026",
			"content":   "Test content",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "invalid note_date format")
	})

	t.Run("bad_request_missing_content", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteNoContent", "Test", "ANNC1")

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content is required")
	})

	t.Run("bad_request_content_too_long", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteLongContent", "Test", "ANLC1")

		longContent := make([]byte, 501)
		for i := range longContent {
			longContent[i] = 'a'
		}
		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   string(longContent),
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content cannot exceed 500 characters")
	})

	// #2329: every verified staff member may write the care plan; what
	// stays refused is an account without a staff record (guest, guardian).
	t.Run("forbidden_without_staff_record", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteForbidden", "Test", "ANF1")
		guest := testpkg.CreateTestAccount(t, tc.db, "arrnotestaff-guest@example.com")

		body := map[string]any{
			"note_date": "2026-03-15",
			"content":   "Test note",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
	})
}

// =============================================================================
// Update Arrival Note Tests
// =============================================================================

func TestUpdateStudentArrivalNote(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_updates_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteUpdate", "Test", "ANU1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Update", "ArrNoteTeacher2")

		noteDate := timezone.NewDate(2026, 4, 15)
		note := &scheduleModel.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  noteDate,
			Content:   "Original content",
			CreatedBy: createStudentsAPITestStaffID(t, tc),
		}
		note.SetTenantID(testpkg.Tenant(t))
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

		body := map[string]any{
			"note_date": "2026-04-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-notes/%d", student.ID, note.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Updated content", "Should contain updated content")
	})

	t.Run("bad_request_invalid_note_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteUpdateInvalid", "Test", "ANUI1")

		body := map[string]any{
			"note_date": "2026-02-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-notes/abc", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteUpdateNF", "Test", "ANUNF1")

		body := map[string]any{
			"note_date": "2026-02-15",
			"content":   "Updated content",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-notes/999999", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

// =============================================================================
// Delete Arrival Note Tests
// =============================================================================

func TestDeleteStudentArrivalNote(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_deletes_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteDelete", "Test", "AND1")

		noteDate := timezone.NewDate(2026, 6, 15)
		note := &scheduleModel.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  noteDate,
			Content:   "To be deleted",
			CreatedBy: createStudentsAPITestStaffID(t, tc),
		}
		note.SetTenantID(testpkg.Tenant(t))
		_, err := tc.db.NewInsert().Model(note).
			ModelTableExpr("schedule.student_arrival_notes").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-notes/%d", student.ID, note.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("bad_request_invalid_note_id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteDeleteInvalid", "Test", "ANDI1")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-notes/invalid", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("not_found_nonexistent_note", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrNoteDeleteNF", "Test", "ANDNF1")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/arrival-notes/999999", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})
}

// =============================================================================
// Bulk Upsert Arrival Schedules Tests
// =============================================================================

func TestBulkUpsertArrivalSchedules(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("bad_request_missing_filter", func(t *testing.T) {
		body := map[string]any{
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_with_class_and_group_filters", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "BulkArrivalBothFilters")

		body := map[string]any{
			"school_class": "1a",
			"group_id":     group.ID,
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_with_group_and_student_filters", func(t *testing.T) {
		body := map[string]any{
			"group_id":    42,
			"student_ids": []int64{1, 2},
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_empty_schedules", func(t *testing.T) {
		body := map[string]any{
			"school_class": "1a",
			"schedules":    []map[string]any{},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_weekday", func(t *testing.T) {
		body := map[string]any{
			"school_class": "1a",
			"schedules": []map[string]any{
				{"weekday": 7, "expected_arrival": "08:00"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("success_with_valid_request", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkArrival", "Test", "BAR1")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkArr", "Teacher")

		body := map[string]any{
			"school_class": "BAR1",
			"schedules": []map[string]any{
				{"weekday": 1, "expected_arrival": "07:45"},
				{"weekday": 3, "expected_arrival": "08:15"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		// Cleanup
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
			ModelTableExpr("schedule.student_arrival_schedules").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	t.Run("success_with_group_filter", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "BulkArrivalHandlerGroup")
		student := testpkg.CreateTestStudent(t, tc.db, "BulkGroupArrival", "Test", "BGAR1")
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkGroupArr", "Teacher")

		body := map[string]any{
			"group_id": group.ID,
			"schedules": []map[string]any{
				{"weekday": 2, "expected_arrival": "09:20"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"students_affected":1`)
	})

	t.Run("success_with_explicit_student_ids", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, tc.db, "BulkExplicit1", "Test", "BE1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "BulkExplicit2", "Test", "BE2")

		_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkExplicit", "Teacher")

		body := map[string]any{
			"student_ids": []int64{student1.ID, student2.ID},
			"schedules": []map[string]any{
				{"weekday": 5, "expected_arrival": "10:30"},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-schedules/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"students_affected":2`)
	})
}

// =============================================================================
// Get Bulk Arrival Times Tests
// =============================================================================

func TestGetBulkArrivalTimes(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("bad_request_empty_student_ids", func(t *testing.T) {
		body := map[string]any{
			"student_ids": []int64{},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-times/bulk", body)
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
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		body := map[string]any{
			"student_ids": []int64{1, 2, 3},
			"date":        "27-01-2026",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("success_with_valid_request", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, tc.db, "BulkArrTime1", "Student", "BAT1")
		student2 := testpkg.CreateTestStudent(t, tc.db, "BulkArrTime2", "Student", "BAT2")

		body := map[string]any{
			"student_ids": []int64{student1.ID, student2.ID},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_specific_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkArrDateTest", "Student", "BADT1")

		body := map[string]any{
			"student_ids": []int64{student.ID},
			"date":        "2026-01-27",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/arrival-times/bulk", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// TestStaffArrivalWrite_BroadcastsArrivalScheduleChanged pins that every staff
// arrival write reaches the staff SSE bus at all — the broadcasts now run from
// tenant.RegisterAfterCommit hooks, and a hook that is never registered or never
// drained produces no event here, so this fails loudly if the wiring is dropped.
//
// Scope, stated plainly: it does NOT prove the broadcast happens AFTER the
// commit — an inline call would satisfy it too. Ordering itself is covered by
// the tenant package (aftercommit_test.go / rollback_middleware_test.go); the
// contract for callers is documented on broadcastArrivalScheduleChanged.
//
// The note paths are covered explicitly: they carried no after-commit hook at
// all before, so they are the easiest place for the wiring to be dropped again.
func TestStaffArrivalWrite_BroadcastsArrivalScheduleChanged(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ArrCast", "Teacher")
	claims := testutil.AdminTestClaims(int(account.ID))

	sawArrivalBroadcast := func() bool {
		for _, c := range tc.broadcaster.CallsByMethod("all") {
			if c.Event.Type == realtime.EventArrivalScheduleChanged {
				return true
			}
		}
		return false
	}

	t.Run("weekly_schedule_update", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrCast", "Weekly", "AC1")
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalSchedule)(nil)).
				ModelTableExpr("schedule.student_arrival_schedules").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		tc.broadcaster.Reset()
		body := map[string]any{
			"schedules": []map[string]any{{"weekday": 1, "expected_arrival": "07:45"}},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/arrival-schedules", student.ID), body)
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		assert.True(t, sawArrivalBroadcast(),
			"an arrival schedule update must broadcast arrival_schedule_changed after commit")
	})

	t.Run("day_note_create", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ArrCast", "Note", "AC2")
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalNote)(nil)).
				ModelTableExpr("schedule.student_arrival_notes").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		tc.broadcaster.Reset()
		body := map[string]any{"note_date": "2026-03-15", "content": "Kommt mit dem Bus"}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-notes", student.ID), body)
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		assert.True(t, sawArrivalBroadcast(),
			"an arrival day note must broadcast arrival_schedule_changed after commit")
	})
}
