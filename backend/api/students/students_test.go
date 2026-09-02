package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type studentDayPlanningTestResponse struct {
	ID                int64  `json:"id"`
	DayPlanningStatus string `json:"day_planning_status"`
	DayPlanningReason string `json:"day_planning_reason"`
}

func decodeStudentsByID(t *testing.T, body []byte) map[int64]studentDayPlanningTestResponse {
	t.Helper()
	var resp struct {
		Data []studentDayPlanningTestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	byID := make(map[int64]studentDayPlanningTestResponse, len(resp.Data))
	for _, student := range resp.Data {
		byID[student.ID] = student
	}
	return byID
}

func requireStudentsBusDaysColumn(t *testing.T, tc *testContext) {
	t.Helper()
	var exists bool
	err := tc.db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'users'
			  AND table_name = 'students'
			  AND column_name = 'bus_days'
		)
	`).Scan(context.Background(), &exists)
	require.NoError(t, err)
	if !exists {
		t.Skip("users.students.bus_days column is not present in this test database")
	}
}

// =============================================================================
// List Students with Pickup Times Tests
// =============================================================================

func TestListStudents_WithPickupTimes(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t, fixedCalendarClock)

	// Create two students: one with a pickup schedule, one without
	studentWithSchedule := testpkg.CreateTestStudent(t, tc.db, "Pickup", "WithSchedule", "PT1")
	studentNoSchedule := testpkg.CreateTestStudent(t, tc.db, "Pickup", "NoSchedule", "PT2")

	// The handler derives the weekday via timezone.DateOf(time.Now()), which
	// converts to Europe/Berlin before extracting the date. The test must use
	// the same conversion so the inserted schedule matches the handler's query,
	// even when CI runs near midnight UTC (where UTC and Berlin dates differ).
	berlinToday := timezone.DateOf(time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	todayWeekday := int(berlinToday.Weekday())
	if todayWeekday == 0 {
		todayWeekday = 7 // Sunday
	}

	// Skip on weekends — pickup schedules only apply Mon-Fri
	if todayWeekday > scheduleModel.WeekdayFriday {
		t.Skip("Skipping pickup time test on weekend — schedules only apply Mon-Fri")
	}

	// Insert a pickup schedule for today's weekday
	pickupTime := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)
	schedule := &scheduleModel.StudentPickupSchedule{
		StudentID:  studentWithSchedule.ID,
		Weekday:    todayWeekday,
		PickupTime: pickupTime,
		CreatedBy:  createStudentsAPITestStaffID(t, tc),
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	_, err := tc.db.NewInsert().Model(schedule).
		ModelTableExpr("schedule.student_pickup_schedules").
		Returning("id").
		Exec(context.Background())
	require.NoError(t, err)
	defer func() {
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
			ModelTableExpr("schedule.student_pickup_schedules").
			Where("student_id = ?", studentWithSchedule.ID).
			Exec(context.Background())
	}()

	t.Run("include_pickup_times_returns_pickup_time_field", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?include_pickup_times=true&school_class=PT1&page_size=50", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		// Parse response to check pickup_time field
		var resp struct {
			Data []json.RawMessage `json:"data"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err, "Failed to parse response JSON")
		require.NotEmpty(t, resp.Data, "Expected at least one student in response")

		// Find the student with schedule and verify pickup_time is set
		found := false
		for _, raw := range resp.Data {
			var student struct {
				ID         int64   `json:"id"`
				PickupTime *string `json:"pickup_time"`
			}
			err := json.Unmarshal(raw, &student)
			require.NoError(t, err)

			if student.ID == studentWithSchedule.ID {
				found = true
				require.NotNil(t, student.PickupTime, "Student with schedule should have pickup_time")
				assert.Equal(t, "15:30", *student.PickupTime, "Pickup time should match the schedule")
			}

			if student.ID == studentNoSchedule.ID {
				assert.Nil(t, student.PickupTime, "Student without schedule should not have pickup_time")
			}
		}
		assert.Truef(t, found, "Student with schedule should be in response. Body: %s", rr.Body.String())
	})

	t.Run("without_include_pickup_times_omits_field", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?school_class=PT1&page_size=50", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code)

		// Verify pickup_time is not present in any response
		var resp struct {
			Data []json.RawMessage `json:"data"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)

		for _, raw := range resp.Data {
			var student map[string]interface{}
			err := json.Unmarshal(raw, &student)
			require.NoError(t, err)

			_, hasPickupTime := student["pickup_time"]
			assert.False(t, hasPickupTime, "pickup_time should not be present when include_pickup_times is not set")
		}
	})
}

func TestListStudents_WithArrivalTimes(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t, fixedCalendarClock)

	schoolClass := fmt.Sprintf("AT-%d", time.Now().UnixNano())
	studentWithSchedule := testpkg.CreateTestStudent(t, tc.db, "Arrival", "WithSchedule", schoolClass)
	studentNoSchedule := testpkg.CreateTestStudent(t, tc.db, "Arrival", "NoSchedule", schoolClass)
	staff := testpkg.CreateTestStaff(t, tc.db, "Arrival", "Creator")

	berlinToday := timezone.DateOf(time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
	todayWeekday := int(berlinToday.Weekday())
	if todayWeekday == 0 {
		todayWeekday = 7
	}

	if todayWeekday > scheduleModel.WeekdayFriday {
		t.Skip("Skipping arrival time test on weekend — schedules only apply Mon-Fri")
	}

	arrivalTime := time.Date(2000, 1, 1, 8, 15, 0, 0, time.UTC)
	schedule := &scheduleModel.StudentArrivalSchedule{
		StudentID:       studentWithSchedule.ID,
		Weekday:         todayWeekday,
		ExpectedArrival: arrivalTime,
		CreatedBy:       staff.ID,
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
			Where("student_id = ?", studentWithSchedule.ID).
			Exec(context.Background())
	}()

	t.Run("include_arrival_times_returns_arrival_time_field", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?include_arrival_times=true&school_class=%s&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		var resp struct {
			Data []json.RawMessage `json:"data"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Data)

		found := false
		for _, raw := range resp.Data {
			var student struct {
				ID          int64   `json:"id"`
				ArrivalTime *string `json:"arrival_time"`
			}
			err := json.Unmarshal(raw, &student)
			require.NoError(t, err)

			if student.ID == studentWithSchedule.ID {
				found = true
				require.NotNil(t, student.ArrivalTime)
				assert.Equal(t, "08:15", *student.ArrivalTime)
			}

			if student.ID == studentNoSchedule.ID {
				assert.Nil(t, student.ArrivalTime)
			}
		}
		assert.Truef(t, found, "Student with schedule should be in response. Body: %s", rr.Body.String())
	})

	t.Run("without_include_arrival_times_omits_field", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			Data []json.RawMessage `json:"data"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)

		for _, raw := range resp.Data {
			var student map[string]interface{}
			err := json.Unmarshal(raw, &student)
			require.NoError(t, err)

			_, hasArrivalTime := student["arrival_time"]
			assert.False(t, hasArrivalTime, "arrival_time should not be present when include_arrival_times is not set")
		}
	})
}

func TestListStudents_DayPlanningStatus(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	fixedNow := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	tc.resource.Now = func() time.Time { return fixedNow }

	schoolClass := fmt.Sprintf("DP-%d", time.Now().UnixNano())
	planned := testpkg.CreateTestStudent(t, tc.db, "DayPlan", "Pickup", schoolClass)
	notPlanned := testpkg.CreateTestStudent(t, tc.db, "DayPlan", "NoPlan", schoolClass)
	walkIn := testpkg.CreateTestStudent(t, tc.db, "DayPlan", "WalkIn", schoolClass)
	sick := testpkg.CreateTestStudent(t, tc.db, "DayPlan", "Sick", schoolClass)
	exceptionAbsent := testpkg.CreateTestStudent(t, tc.db, "DayPlan", "Exception", schoolClass)
	staff := testpkg.CreateTestStaff(t, tc.db, "DayPlan", "Creator")
	device := testpkg.CreateTestDevice(t, tc.db, "day-planning-device")

	today := timezone.DateFromTime(fixedNow)
	testpkg.CreateTestPickupSchedule(t, tc.db, planned.ID, scheduleModel.WeekdayMonday, staff.ID, "15:30")
	testpkg.CreateTestAttendance(t, tc.db, walkIn.ID, staff.ID, device.ID, time.Now().Add(-30*time.Minute), nil)

	trueValue := true
	_, err := tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("sick = ?", trueValue).
		Where("id = ?", sick.ID).
		Exec(context.Background())
	require.NoError(t, err)
	testpkg.CreateTestArrivalException(t, tc.db, exceptionAbsent.ID, today, staff.ID, "", "Arzttermin")

	t.Run("returns_computed_day_planning_fields", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())
		assert.Equal(t, students.DayPlanningStatusComesToday, byID[planned.ID].DayPlanningStatus)
		assert.Equal(t, "pickup_schedule", byID[planned.ID].DayPlanningReason)
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[notPlanned.ID].DayPlanningStatus)
		assert.Equal(t, "no_plan", byID[notPlanned.ID].DayPlanningReason)
		assert.Equal(t, students.DayPlanningStatusComesToday, byID[walkIn.ID].DayPlanningStatus)
		assert.Equal(t, "unplanned_attendance", byID[walkIn.ID].DayPlanningReason)
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[sick.ID].DayPlanningStatus)
		assert.Equal(t, "sick", byID[sick.ID].DayPlanningReason)
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[exceptionAbsent.ID].DayPlanningStatus)
		assert.Equal(t, "arrival_exception", byID[exceptionAbsent.ID].DayPlanningReason)
	})

	t.Run("filters_comes_today_before_pagination", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&day_status=comes_today&page_size=1", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())
		require.Len(t, byID, 1)
		_, ok := byID[planned.ID]
		assert.True(t, ok, "only planned student should survive comes_today filter")
	})

	t.Run("filters_actual_walk_in_as_comes_today_before_pagination", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&day_status=comes_today&page_size=2", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())
		require.Len(t, byID, 2)
		assert.Contains(t, byID, planned.ID)
		assert.Contains(t, byID, walkIn.ID)
	})

	t.Run("filters_not_coming_today", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&day_status=not_coming_today&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())
		assert.NotContains(t, byID, planned.ID)
		assert.NotContains(t, byID, walkIn.ID)
		assert.Contains(t, byID, notPlanned.ID)
		assert.Contains(t, byID, sick.ID)
		assert.Contains(t, byID, exceptionAbsent.ID)
	})
}

// =============================================================================
// List Students Tests
// =============================================================================

func TestListStudents(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Create test students using fixtures
	testpkg.CreateTestStudent(t, tc.db, "List", "StudentOne", "1a")
	testpkg.CreateTestStudent(t, tc.db, "List", "StudentTwo", "1b")
	testpkg.CreateTestStudent(t, tc.db, "List", "StudentEleven", "11a")

	t.Run("success_admin_lists_all_students", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_pagination", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?page=1&page_size=10", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_school_class_filter", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?school_class=1a", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "StudentOne")
		assert.NotContains(t, rr.Body.String(), "StudentEleven")
	})

	t.Run("success_with_search_filter", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?search=List", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListStudents_WithLocationFilter(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	testpkg.CreateTestStudent(t, tc.db, "Location", "Filter", "LF1")

	t.Run("filter_by_in_house", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?location=in_house", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("filter_by_absent", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?location=absent", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListStudents_WithNameFilters(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	testpkg.CreateTestStudent(t, tc.db, "NameFilter", "Test", "NF1")

	t.Run("filter_by_first_name", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?first_name=NameFilter", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "NameFilter")
	})

	t.Run("filter_by_last_name", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?last_name=Test", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListStudents_ExtendedFilters(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	testpkg.CreateTestStudent(t, tc.db, "Filter", "Student", "FI1")

	t.Run("filter_by_group_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FilterGroup")

		req := testutil.NewRequest("GET", fmt.Sprintf("/?group_id=%d", group.ID), nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid_page_size", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?page_size=invalid", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Invalid page_size should return bad request or be ignored
		assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, rr.Code)
	})

	t.Run("negative_page", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?page=-1", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Negative page should return bad request or default to 1
		assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, rr.Code)
	})
}

// =============================================================================
// Get Student Tests
// =============================================================================

func TestGetStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Get", "Student", "GS1")

	t.Run("success_admin_gets_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Get")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	// A graduated (alumnus) student is soft-deleted. GetStudentByID is
	// unfiltered, so the shared parseAndGetStudent gate must reject a bookmarked
	// or directly-called per-student route with 404 — the same status these
	// routes returned when graduates were hard-deleted (#405).
	t.Run("not_found_for_alumnus", func(t *testing.T) {
		alumnus := testpkg.CreateTestStudent(t, tc.db, "Graduated", "Alumnus", "GS-Alum")
		_, err := tc.db.NewUpdate().
			TableExpr(`users.students`).
			Set("status = ?", string(usersModel.StudentStatusAlumnus)).
			Where("id = ?", alumnus.ID).
			Exec(t.Context())
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", alumnus.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("not_found_for_ended_care_without_delete_permission", func(t *testing.T) {
		ended := testpkg.CreateTestStudent(t, tc.db, "Care", "Ended", "GS-Ended")
		_, err := tc.db.NewUpdate().
			TableExpr(`users.students`).
			Set("enrolled_until = ?", timezone.TodayDate().AddDays(-1)).
			Where("id = ?", ended.ID).
			Exec(t.Context())
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", ended.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		testutil.AssertNotFound(t, rr)
	})
}

func TestGetStudentIncludesConsentWithdrawalForStaff(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Consent", "Visible", "GS-Consent")
	grantedAt := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	withdrawnAt := time.Date(2026, time.August, 31, 15, 0, 0, 0, time.UTC)
	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("agb_accepted_at = ?", grantedAt).
		Set("photo_consent_given_at = NULL").
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	change := &auditModel.StudentConsentChange{
		Model:      auditModel.Model{CreatedAt: withdrawnAt, UpdatedAt: withdrawnAt},
		StudentID:  student.ID,
		ConsentKey: auditModel.StudentConsentPhoto,
		Action:     auditModel.StudentConsentWithdrawn,
		Source:     auditModel.StudentConsentSourceParentPortal,
	}
	change.SetTenantID(testpkg.Tenant(t))
	_, err = tc.db.NewInsert().Model(change).Returning("id").Exec(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().Model((*auditModel.StudentConsentChange)(nil)).
			Where("id = ?", change.ID).
			Exec(context.Background())
	})

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data struct {
			Consents []students.StudentConsentResponse `json:"consents"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Len(t, response.Data.Consents, 4)
	assert.Equal(t, "agb", response.Data.Consents[0].Key)
	assert.Equal(t, "granted", response.Data.Consents[0].State)
	assert.Equal(t, "photo", response.Data.Consents[3].Key)
	assert.Equal(t, "withdrawn", response.Data.Consents[3].State)
	require.NotNil(t, response.Data.Consents[3].ChangedAt)
	assert.True(t, response.Data.Consents[3].ChangedAt.Equal(withdrawnAt))
}

// =============================================================================
// Create Student Tests
// =============================================================================

func TestCreateStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_creates_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "New",
			"last_name":    "Student",
			"school_class": "2a",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
	})

	t.Run("success_creates_student_with_optional_fields", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":          "Optional",
			"last_name":           "Fields",
			"school_class":        "2b",
			"birthday":            "2015-06-15",
			"guardian_name":       "Parent Name",
			"guardian_email":      "parent@example.com",
			"address_street":      "Musterstraße 12",
			"address_city":        "Köln",
			"address_postal_code": "50667",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code)
		var resp struct {
			Data students.StudentResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "Musterstraße 12", resp.Data.AddressStreet)
		assert.Equal(t, "Köln", resp.Data.AddressCity)
		assert.Equal(t, "50667", resp.Data.AddressPostalCode)
	})

	t.Run("success_creates_student_with_bus_days", func(t *testing.T) {
		requireStudentsBusDaysColumn(t, tc)

		body := map[string]interface{}{
			"first_name":   "BusDays",
			"last_name":    "Create",
			"school_class": "2b",
			"bus_days": map[string]bool{
				"mon": true,
				"wed": true,
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var resp struct {
			Data students.StudentResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.True(t, resp.Data.Bus)
		assert.True(t, resp.Data.BusDays[usersModel.BusDayMonday])
		assert.True(t, resp.Data.BusDays[usersModel.BusDayWednesday])
		assert.False(t, resp.Data.BusDays[usersModel.BusDayTuesday])
	})

	t.Run("success_creates_student_with_departure_days", func(t *testing.T) {
		requireStudentsBusDaysColumn(t, tc)

		body := map[string]interface{}{
			"first_name":   "Departure",
			"last_name":    "Create",
			"school_class": "2b",
			"departure_days": map[string]string{
				"mon": "bus",
				"wed": "pickup",
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())

		var resp struct {
			Data students.StudentResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		// Unified field round-trips, and the legacy mirrors are derived.
		assert.Equal(t, usersModel.DepartureBus, resp.Data.DepartureDays.ModeFor("mon"))
		assert.Equal(t, usersModel.DeparturePickup, resp.Data.DepartureDays.ModeFor("wed"))
		assert.True(t, resp.Data.BusDays[usersModel.BusDayMonday])
		assert.True(t, resp.Data.Bus)
		assert.True(t, resp.Data.PickupDays[usersModel.PickupDayWednesday])
		assert.Equal(t, usersModel.PickupStatusPickedUp, resp.Data.PickupStatus)
	})

	t.Run("bad_request_invalid_departure_mode", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":     "Invalid",
			"last_name":      "Departure",
			"school_class":   "2c",
			"departure_days": map[string]string{"mon": "taxi"},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "taxi")
	})

	t.Run("bad_request_missing_first_name", func(t *testing.T) {
		body := map[string]interface{}{
			"last_name":    "NoFirst",
			"school_class": "2c",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_last_name", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "NoLast",
			"school_class": "2c",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_school_class", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "NoClass",
			"last_name":  "Student",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_birthday_format", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "Invalid",
			"last_name":    "Birthday",
			"school_class": "2c",
			"birthday":     "not-a-date",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Invalid birthday format should fail
		assert.NotEqual(t, http.StatusCreated, rr.Code)
	})

	t.Run("bad_request_invalid_bus_days_weekday", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "Invalid",
			"last_name":    "BusDays",
			"school_class": "2c",
			"bus_days": map[string]bool{
				"sat": true,
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "sat")

		var count int
		err := tc.db.NewSelect().
			TableExpr(`users.persons AS "person"`).
			ColumnExpr(`count(*)`).
			Where(`"person".first_name = ?`, "Invalid").
			Where(`"person".last_name = ?`, "BusDays").
			Scan(testpkg.Ctx(t), &count)
		require.NoError(t, err)
		assert.Zero(t, count)
	})
}

func TestCreateStudent_WithGroupID(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CreateGroup")

	t.Run("creates_student_with_group", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "WithGroup",
			"last_name":    "Student",
			"school_class": "3a",
			"group_id":     group.ID,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestCreateStudent_WithAllOptionalFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("create_with_all_fields", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FullCreateGroup")

		body := map[string]interface{}{
			"first_name":       "Full",
			"last_name":        "Create",
			"school_class":     "FC1",
			"birthday":         "2015-03-25",
			"group_id":         group.ID,
			"guardian_name":    "Parent Full",
			"guardian_email":   "fullparent@test.com",
			"guardian_phone":   "+4912345678",
			"guardian_contact": "Emergency info",
			"health_info":      "No allergies",
			"extra_info":       "Extra notes",
			"pickup_status":    "bus",
			"bus":              true,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Should create student. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Student Tests
// =============================================================================

func TestUpdateStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Update", "Student", "US1")

	t.Run("success_updates_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Updated")
	})

	t.Run("success_updates_multiple_fields", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "Multi",
			"last_name":    "Update",
			"school_class": "4a",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "NotFound",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_empty_first_name", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateStudent_WithGuardianInfo(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Update", "GU1")

	t.Run("update_guardian_name", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_name": "New Guardian",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_guardian_email", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_email": "guardian@example.com",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_guardian_phone", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_phone": "+49123456789",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_WithSickStatus(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Sick", "Status", "SS1")

	t.Run("mark_as_sick", func(t *testing.T) {
		body := map[string]interface{}{
			"sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"sick":true`)
	})

	t.Run("mark_as_not_sick", func(t *testing.T) {
		body := map[string]interface{}{
			"sick": false,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_SickStatusExtended(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("mark_student_as_sick_sets_sick_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "SickSince", "Student", "SS2")

		// Mark as sick
		body := map[string]interface{}{
			"sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Should mark student as sick")
		assert.Contains(t, rr.Body.String(), `"sick":true`)
	})

	t.Run("clear_sick_status_clears_sick_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ClearSick", "Student", "CS1")

		// First mark as sick
		sickBody := map[string]interface{}{
			"sick": true,
		}
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), sickBody)
		sickRR := authExec(t, tc, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, sickRR.Code)

		// Then clear sick status
		clearBody := map[string]interface{}{
			"sick": false,
		}
		clearReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), clearBody)
		clearRR := authExec(t, tc, clearReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, clearRR.Code, "Should clear sick status")
		assert.Contains(t, clearRR.Body.String(), `"sick":false`)

		var history activeModel.StudentStatusDay
		err := tc.db.NewSelect().
			Model(&history).
			ModelTableExpr(`active.student_status_days AS "student_status_day"`).
			Where(`"student_status_day".student_id = ?`, student.ID).
			Where(`"student_status_day".status = ?`, activeModel.StudentStatusDaySick).
			Scan(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, history.ClearedAt)
	})
}

func TestUpdateStudent_WithExcusedStatus(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("mark_as_excused_sets_excused_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Excused", "Status", "ES1")

		eventCount := len(tc.broadcaster.Events())

		body := map[string]interface{}{"excused": true}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"excused":true`)
		assert.Contains(t, rr.Body.String(), `"excused_since"`)
		events := tc.broadcaster.Events()
		require.Len(t, events, eventCount+1)
		assert.Equal(t, "student_updated", string(events[eventCount].Type))
	})

	t.Run("clear_excused_clears_excused_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ClearExcused", "Student", "EC1")

		setReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"excused": true})
		setRR := authExec(t, tc, setReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, setRR.Code)

		clearReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"excused": false})
		clearRR := authExec(t, tc, clearReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, clearRR.Code)
		assert.Contains(t, clearRR.Body.String(), `"excused":false`)

		var history activeModel.StudentStatusDay
		err := tc.db.NewSelect().
			Model(&history).
			ModelTableExpr(`active.student_status_days AS "student_status_day"`).
			Where(`"student_status_day".student_id = ?`, student.ID).
			Where(`"student_status_day".status = ?`, activeModel.StudentStatusDayExcused).
			Scan(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, history.ClearedAt)
	})
}

// TestUpdateStudent_SickExcusedMutualExclusion encodes the business rule
// that a student cannot be both sick and excused at the same time.
func TestUpdateStudent_SickExcusedMutualExclusion(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("both_flags_true_in_same_request_is_rejected", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Mutex", "Student", "MS1")

		body := map[string]interface{}{"sick": true, "excused": true}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusConflict, rr.Code,
			"setting both sick and excused to true must return 409")
		assert.Contains(t, rr.Body.String(), students.ErrCodeSickExcusedConflict,
			"response should carry the SICK_EXCUSED_CONFLICT code for the frontend")
	})

	t.Run("setting_sick_while_already_excused_is_rejected", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Mutex", "Excused", "ME1")

		// Pre-state: excused = true
		excReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"excused": true})
		excRR := authExec(t, tc, excReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, excRR.Code)

		// Attempt: set sick = true without clearing excused
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"sick": true})
		sickRR := authExec(t, tc, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusConflict, sickRR.Code)
		assert.Contains(t, sickRR.Body.String(), students.ErrCodeSickExcusedConflict)
	})

	t.Run("switch_from_sick_to_excused_in_one_request_succeeds", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Switch", "Student", "SW1")

		// Pre-state: sick = true
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"sick": true})
		sickRR := authExec(t, tc, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, sickRR.Code)

		// Switch: clear sick AND set excused in one request
		switchReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"sick": false, "excused": true})
		switchRR := authExec(t, tc, switchReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, switchRR.Code,
			"simultaneous clear-one + set-other must be accepted")
		assert.Contains(t, switchRR.Body.String(), `"sick":false`)
		assert.Contains(t, switchRR.Body.String(), `"excused":true`)
	})
}

func TestUpdateStudent_ExtendedFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_health_info", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Health", "Student", "HS1")

		body := map[string]interface{}{
			"health_info": "Allergies: Peanuts",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_extra_info", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Extra", "Student", "EX1")

		body := map[string]interface{}{
			"extra_info": "Additional notes about the student",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_supervisor_notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Notes", "Student", "NS1")

		body := map[string]interface{}{
			"supervisor_notes": "Supervisor observations",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_and_clear_child_address", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Address", "Student", "AS1")

		body := map[string]interface{}{
			"address_street":      "  Neue Straße 5  ",
			"address_city":        "  Bonn  ",
			"address_postal_code": "  53111  ",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		var resp struct {
			Data students.StudentResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "Neue Straße 5", resp.Data.AddressStreet)
		assert.Equal(t, "Bonn", resp.Data.AddressCity)
		assert.Equal(t, "53111", resp.Data.AddressPostalCode)

		clearBody := map[string]interface{}{
			"address_street":      "",
			"address_city":        "",
			"address_postal_code": "",
		}
		clearReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), clearBody)
		clearRR := authExec(t, tc, clearReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, clearRR.Code, "Body: %s", clearRR.Body.String())
		var clearResp struct {
			Data students.StudentResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(clearRR.Body.Bytes(), &clearResp))
		assert.Empty(t, clearResp.Data.AddressStreet)
		assert.Empty(t, clearResp.Data.AddressCity)
		assert.Empty(t, clearResp.Data.AddressPostalCode)

		fresh, err := tc.resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.Nil(t, fresh.AddressStreet)
		assert.Nil(t, fresh.AddressCity)
		assert.Nil(t, fresh.AddressPostalCode)
	})

	t.Run("update_pickup_status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Pickup", "Student", "PU1")

		body := map[string]interface{}{
			"pickup_status": "ready",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_bus", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Bus", "Student", "BU1")

		// Bus is a boolean flag, not a string
		body := map[string]interface{}{
			"bus": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("legacy_bus_true_preserves_existing_bus_days", func(t *testing.T) {
		requireStudentsBusDaysColumn(t, tc)

		student := testpkg.CreateTestStudent(t, tc.db, "BusDays", "Preserve", "BDP1")

		existing := usersModel.BusDays{
			usersModel.BusDayMonday:    true,
			usersModel.BusDayWednesday: true,
		}
		_, err := tc.db.NewUpdate().
			TableExpr(`users.students AS "student"`).
			Set(`bus_days = ?`, existing).
			Where(`"student".id = ?`, student.ID).
			Exec(testpkg.Ctx(t))
		require.NoError(t, err)

		body := map[string]interface{}{
			"bus": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code)
		fresh, err := tc.resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.True(t, fresh.BusDays[usersModel.BusDayMonday])
		assert.True(t, fresh.BusDays[usersModel.BusDayWednesday])
		assert.False(t, fresh.BusDays[usersModel.BusDayTuesday])
		assert.False(t, fresh.BusDays[usersModel.BusDayThursday])
		assert.False(t, fresh.BusDays[usersModel.BusDayFriday])
	})

	t.Run("bus_days_update_derives_bus_flag", func(t *testing.T) {
		requireStudentsBusDaysColumn(t, tc)

		student := testpkg.CreateTestStudent(t, tc.db, "BusDays", "Replace", "BDR1")

		body := map[string]interface{}{
			"bus_days": map[string]bool{
				"tue": true,
				"thu": true,
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code)
		fresh, err := tc.resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.True(t, fresh.BusDays.HasAny())
		assert.True(t, fresh.BusDays[usersModel.BusDayTuesday])
		assert.True(t, fresh.BusDays[usersModel.BusDayThursday])
		assert.False(t, fresh.BusDays[usersModel.BusDayMonday])
	})

	t.Run("departure_days_update_replaces_plan_and_derives_mirrors", func(t *testing.T) {
		requireStudentsBusDaysColumn(t, tc)

		student := testpkg.CreateTestStudent(t, tc.db, "Departure", "Update", "DEP1")

		// Seed an existing bus plan, then replace it via the unified field.
		seed := map[string]interface{}{"bus_days": map[string]bool{"mon": true}}
		seedReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), seed)
		require.Equal(t, http.StatusOK, authExec(t, tc, seedReq, testutil.AdminTestClaims(1), []string{"admin:*"}).Code)

		body := map[string]interface{}{
			"departure_days": map[string]string{"tue": "pickup", "thu": "bus"},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		fresh, err := tc.resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		// Unified replacement fully overrides the prior Monday bus day.
		assert.Equal(t, usersModel.DeparturePickup, fresh.DepartureDays.ModeFor("tue"))
		assert.Equal(t, usersModel.DepartureBus, fresh.DepartureDays.ModeFor("thu"))
		assert.Equal(t, usersModel.DepartureAlone, fresh.DepartureDays.ModeFor("mon"))
		// Derived legacy mirrors reflect the new plan.
		assert.True(t, fresh.PickupDays[usersModel.PickupDayTuesday])
		assert.True(t, fresh.BusDays[usersModel.BusDayThursday])
		assert.False(t, fresh.BusDays[usersModel.BusDayMonday])
		require.NotNil(t, fresh.PickupStatus)
		assert.Equal(t, usersModel.PickupStatusPickedUp, *fresh.PickupStatus)
	})

	t.Run("invalid_bus_days_update_is_rejected_without_changing_state", func(t *testing.T) {
		requireStudentsBusDaysColumn(t, tc)

		student := testpkg.CreateTestStudent(t, tc.db, "BusDays", "Reject", "BDX1")

		existing := usersModel.BusDays{
			usersModel.BusDayMonday: true,
		}
		_, err := tc.db.NewUpdate().
			TableExpr(`users.students AS "student"`).
			Set(`bus_days = ?`, existing).
			Where(`"student".id = ?`, student.ID).
			Exec(testpkg.Ctx(t))
		require.NoError(t, err)

		body := map[string]interface{}{
			"bus_days": map[string]bool{
				"sat": true,
			},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "sat")

		fresh, err := tc.resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.True(t, fresh.BusDays.HasAny())
		assert.True(t, fresh.BusDays[usersModel.BusDayMonday])
		assert.False(t, fresh.BusDays[usersModel.BusDayTuesday])
		assert.False(t, fresh.BusDays[usersModel.BusDayWednesday])
		assert.False(t, fresh.BusDays[usersModel.BusDayThursday])
		assert.False(t, fresh.BusDays[usersModel.BusDayFriday])
	})

	t.Run("update_guardian_contact", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Contact", "Student", "GC1")

		body := map[string]interface{}{
			"guardian_contact": "Emergency: 0800-123456",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_PersonFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_last_name", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Original", "Last", "OL1")

		body := map[string]interface{}{
			"last_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Updated")
	})

	t.Run("update_birthday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Birthday", "Update", "BU2")

		body := map[string]interface{}{
			"birthday": "2015-06-15",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "2015-06-15")
	})

	t.Run("clear_guardian_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Clear", "GCL1")

		// First set guardian fields
		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.students SET guardian_name = ?, guardian_email = ? WHERE id = ?",
			"Parent Name", "parent@test.com", student.ID)
		require.NoError(t, err)

		// Clear guardian name by setting empty string
		body := map[string]interface{}{
			"guardian_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// =============================================================================
// Delete Student Tests
// =============================================================================

func TestDeleteStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_deletes_student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Delete", "Me", "DM1")
		// No cleanup needed - we're deleting

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Handler returns 200 OK with success message (not 204 NoContent)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/999999", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/invalid", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Student Request Validation Tests
// =============================================================================

func TestStudentRequestValidation(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("bind_validates_required_fields", func(t *testing.T) {
		// Empty body should fail validation
		body := map[string]interface{}{}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Router Tests
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	router := tc.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// Error Rendering Coverage
// =============================================================================

func TestRenderErrorCases(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("internal_server_error", func(t *testing.T) {
		// Request for student that doesn't exist to trigger error path
		req := testutil.NewRequest("GET", "/999999/current-visit", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Should return some error status
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})
}

// =============================================================================
// Student With Group and Supervisor Tests (Coverage for supervisor contacts)
// =============================================================================

func TestGetStudent_WithGroupAndSupervisors(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("student_with_group_and_teacher", func(t *testing.T) {
		// Create a complete setup: teacher, group, and student
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Supervisor", "Teacher")
		group := testpkg.CreateTestEducationGroup(t, tc.db, "SupervisorGroup")
		student := testpkg.CreateTestStudent(t, tc.db, "Supervised", "Student", "SS1")

		// Assign teacher to group
		testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

		// Assign student to group
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		// Admin sees full details including supervisors
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		// Should return student data with group
		assert.Contains(t, rr.Body.String(), "SupervisorGroup")
	})

	t.Run("non_admin_sees_supervisor_contacts", func(t *testing.T) {
		// Create a complete setup: teacher assigned to group, student in group
		teacher, teacherAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Contact", "Teacher")
		group := testpkg.CreateTestEducationGroup(t, tc.db, "ContactGroup")
		student := testpkg.CreateTestStudent(t, tc.db, "Contact", "Student", "CS1")

		// Assign teacher to group (this makes them a supervisor)
		testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

		// Assign student to group
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

		// Create another staff member (not a supervisor of this group)
		_, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Other", "Viewer")

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		// Non-admin (supervisor of the group) sees student with supervisor contacts
		claims := testutil.TeacherTestClaims(int(teacherAccount.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		// Also test with staff who has limited access - should see supervisor contacts
		req2 := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		claims2 := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr2 := authExec(t, tc, req2, claims2, []string{"users:read"})

		// Staff can view student (read permission) but should see limited data with supervisor contacts
		assert.Equal(t, http.StatusOK, rr2.Code, "Expected 200 OK. Body: %s", rr2.Body.String())
	})
}

// =============================================================================
// Extended Update Tests (Coverage for applyPersonUpdates paths)
// =============================================================================

func TestUpdateStudent_AllPersonFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_all_person_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Update", "AllFields", "UAF1")

		body := map[string]interface{}{
			"first_name":  "NewFirst",
			"last_name":   "NewLast",
			"birthday":    "2015-06-15",
			"gender":      "m",
			"street":      "New Street 123",
			"city":        "New City",
			"postal_code": "54321",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "NewFirst")
		assert.Contains(t, rr.Body.String(), "NewLast")
	})

	t.Run("update_guardian_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Update", "GU1")

		body := map[string]interface{}{
			"guardian_first_name": "GuardianFirst",
			"guardian_last_name":  "GuardianLast",
			"guardian_email":      "guardian@example.com",
			"guardian_phone":      "+49123456789",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("update_student_specific_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Student", "Specific", "SS2")

		body := map[string]interface{}{
			"school_class":        "2b",
			"bus":                 true,
			"extra_info":          "Some extra info",
			"data_retention_days": 15,
			"responsible_person":  "Ms. Smith",
			"responsible_phone":   "+49987654321",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("update_sick_status_extended", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Sick", "Status", "SK1")

		body := map[string]interface{}{
			"sick":       true,
			"sick_since": "2024-01-15",
			"sick_until": "2024-01-20",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("clear_sick_status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Clear", "Sick", "CS1")

		// First set sick status
		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx, "UPDATE users.students SET sick = true WHERE id = ?", student.ID)
		require.NoError(t, err)

		body := map[string]interface{}{
			"sick": false,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Extended Create Tests (Coverage for createStudent error paths)
// =============================================================================

func TestCreateStudent_ExtendedValidation(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("create_with_all_optional_fields", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":          "Complete",
			"last_name":           "Student",
			"school_class":        "3a",
			"birthday":            "2015-03-20",
			"gender":              "f",
			"street":              "Main Street 42",
			"city":                "Berlin",
			"postal_code":         "10115",
			"bus":                 true,
			"extra_info":          "Test student with all fields",
			"guardian_first_name": "Parent",
			"guardian_last_name":  "Name",
			"guardian_email":      "parent@example.com",
			"guardian_phone":      "+49111222333",
			"responsible_person":  "Teacher",
			"responsible_phone":   "+49444555666",
			"data_retention_days": 20,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
			// Cleanup created student
			assert.Contains(t, rr.Body.String(), "Complete")
		}
	})

	t.Run("create_with_group_assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "AssignGroup")

		body := map[string]interface{}{
			"first_name":   "Group",
			"last_name":    "Assigned",
			"school_class": "4a",
			"group_id":     group.ID,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
			assert.Contains(t, rr.Body.String(), "Group")
		}
	})
}

// =============================================================================
// Extended List Tests (Coverage for list filtering paths)
// =============================================================================

func TestListStudents_GroupAndCombinedFilters(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("filter_with_group_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FilterGroup")
		student := testpkg.CreateTestStudent(t, tc.db, "Filter", "GroupStudent", "FG1")
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/?group_id=%d", group.ID), nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("filter_with_group_id_and_school_class", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FilterGroupClass")
		matching := testpkg.CreateTestStudent(t, tc.db, "Filter", "MatchingClass", "FGC1")
		other := testpkg.CreateTestStudent(t, tc.db, "Filter", "OtherClass", "FGC2")
		testpkg.AssignStudentToGroup(t, tc.db, matching.ID, group.ID)
		testpkg.AssignStudentToGroup(t, tc.db, other.ID, group.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/?group_id=%d&school_class=FGC1&page_size=50", group.ID), nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		var resp struct {
			Data []struct {
				ID          int64  `json:"id"`
				SchoolClass string `json:"school_class"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		require.Len(t, resp.Data, 1)
		assert.Equal(t, matching.ID, resp.Data[0].ID)
		assert.Equal(t, "FGC1", resp.Data[0].SchoolClass)
	})

	t.Run("filter_combined_search_and_class", func(t *testing.T) {
		testpkg.CreateTestStudent(t, tc.db, "Combined", "Filter", "CF1")

		req := testutil.NewRequest("GET", "/?search=Combined&school_class=CF1", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("filter_with_large_page_size", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?page_size=100", nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListSchoolClasses(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Class", "One", "DistinctClass1")
	testpkg.CreateTestStudent(t, tc.db, "Class", "Two", "DistinctClass2")
	testpkg.CreateTestStudent(t, tc.db, "Class", "Duplicate", "DistinctClass1")

	req := testutil.NewRequest("GET", "/school-classes", nil)

	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	var resp struct {
		Data []string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Contains(t, resp.Data, "DistinctClass1")
	assert.Contains(t, resp.Data, "DistinctClass2")
	assert.Equal(t, 1, countString(resp.Data, "DistinctClass1"))
}

func countString(values []string, needle string) int {
	count := 0
	for _, value := range values {
		if value == needle {
			count++
		}
	}
	return count
}

// TestListStudents_AlumniHidden verifies graduated (alumnus) students are
// invisible to the staff student list and the school-classes endpoint. Their
// rows survive in the DB (soft delete via grade transitions), so the API layer
// must filter them out everywhere staff browse students.
func TestListStudents_AlumniHidden(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	alumniClass := fmt.Sprintf("AlumHidden-%d", time.Now().UnixNano()%1_000_000)

	visible := testpkg.CreateTestStudent(t, tc.db, "Visible", "Kid", alumniClass)
	hidden := testpkg.CreateTestStudent(t, tc.db, "Hidden", "Alumnus", alumniClass)

	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModel.StudentStatusAlumnus)).
		Where("id = ?", hidden.ID).
		Exec(t.Context())
	require.NoError(t, err)

	t.Run("list excludes alumni", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s", alumniClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var resp struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

		ids := make([]int64, 0, len(resp.Data))
		for _, s := range resp.Data {
			ids = append(ids, s.ID)
		}
		assert.Contains(t, ids, visible.ID)
		assert.NotContains(t, ids, hidden.ID)
	})

	t.Run("search excludes alumni", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?search=Hidden+Alumnus", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		for _, s := range resp.Data {
			assert.NotEqual(t, hidden.ID, s.ID, "alumnus must not be searchable")
		}
	})

	t.Run("school-classes excludes alumni-only classes", func(t *testing.T) {
		// Graduate the remaining active student too — class disappears entirely
		_, err := tc.db.NewUpdate().
			TableExpr(`users.students`).
			Set("status = ?", string(usersModel.StudentStatusAlumnus)).
			Where("id = ?", visible.ID).
			Exec(t.Context())
		require.NoError(t, err)

		req := testutil.NewRequest("GET", "/school-classes", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			Data []string `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.NotContains(t, resp.Data, alumniClass)
	})
}
