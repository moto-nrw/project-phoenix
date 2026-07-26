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
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListStudents_DayPlanningForDate covers #1939: the `date` query parameter
// evaluates the day-planning pipeline for the requested calendar day instead
// of today. Fixed now is Monday 2026-06-01; the requested day is Tuesday
// 2026-06-02.
func TestListStudents_DayPlanningForDate(t *testing.T) {
	tc := setupTestContext(t)
	fixedNow := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	tc.resource.Now = func() time.Time { return fixedNow }

	schoolClass := fmt.Sprintf("DD-%d", time.Now().UnixNano())
	plannedMondayOnly := testpkg.CreateTestStudent(t, tc.db, "DatePlan", "MondayOnly", schoolClass)
	plannedTomorrow := testpkg.CreateTestStudent(t, tc.db, "DatePlan", "Tuesday", schoolClass)
	sickTomorrow := testpkg.CreateTestStudent(t, tc.db, "DatePlan", "SickTomorrow", schoolClass)
	walkInToday := testpkg.CreateTestStudent(t, tc.db, "DatePlan", "WalkIn", schoolClass)
	sickTodayPlannedTomorrow := testpkg.CreateTestStudent(t, tc.db, "DatePlan", "SickToday", schoolClass)
	staff := testpkg.CreateTestStaff(t, tc.db, "DatePlan", "Creator")
	device := testpkg.CreateTestDevice(t, tc.db, "date-planning-device")
	defer testpkg.CleanupActivityFixtures(t, tc.db, plannedMondayOnly.ID, plannedTomorrow.ID, sickTomorrow.ID, walkInToday.ID, sickTodayPlannedTomorrow.ID, staff.ID, device.ID)

	tomorrow := timezone.NewDate(2026, time.June, 2)

	// Planning signals: one child only planned for Mondays, two planned for the
	// requested Tuesday.
	mondaySchedule := testpkg.CreateTestPickupSchedule(t, tc.db, plannedMondayOnly.ID, scheduleModel.WeekdayMonday, staff.ID, "15:30")
	tuesdaySchedule := testpkg.CreateTestPickupSchedule(t, tc.db, plannedTomorrow.ID, scheduleModel.WeekdayTuesday, staff.ID, "15:30")
	sickTodaySchedule := testpkg.CreateTestPickupSchedule(t, tc.db, sickTodayPlannedTomorrow.ID, scheduleModel.WeekdayTuesday, staff.ID, "14:00")
	defer testpkg.CleanupScheduleFixturesB11(t, tc.db, nil, nil, []int64{mondaySchedule.ID, tuesdaySchedule.ID, sickTodaySchedule.ID}, nil, nil, nil)

	// Currently checked in — must not count as a plan for tomorrow.
	testpkg.CreateTestAttendance(t, tc.db, walkInToday.ID, staff.ID, device.ID, time.Now().Add(-30*time.Minute), nil)

	// Sick TODAY (legacy student-row flag) — must not leak into tomorrow.
	_, err := tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("sick = ?", true).
		Where("id = ?", sickTodayPlannedTomorrow.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Explicit absence recorded for the requested day.
	statusDay := &activeModel.StudentStatusDay{
		StudentID:  sickTomorrow.ID,
		Date:       tomorrow,
		Status:     activeModel.StudentStatusDaySick,
		ReportedAt: fixedNow,
		Source:     activeModel.StudentStatusSourcePlanned,
	}
	statusDay.SetTenantID(1)
	_, err = tc.db.NewInsert().Model(statusDay).
		ModelTableExpr("active.student_status_days").
		Returning("id").
		Exec(context.Background())
	require.NoError(t, err)
	defer func() {
		_, _ = tc.db.NewDelete().Model((*activeModel.StudentStatusDay)(nil)).
			ModelTableExpr("active.student_status_days").
			Where("id = ?", statusDay.ID).
			Exec(context.Background())
	}()

	t.Run("evaluates_day_planning_for_requested_date", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-02&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())

		// Monday-only plan does not apply to Tuesday.
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[plannedMondayOnly.ID].DayPlanningStatus)
		assert.Equal(t, "no_plan", byID[plannedMondayOnly.ID].DayPlanningReason)

		// Tuesday plan applies.
		assert.Equal(t, students.DayPlanningStatusComesToday, byID[plannedTomorrow.ID].DayPlanningStatus)
		assert.Equal(t, "pickup_schedule", byID[plannedTomorrow.ID].DayPlanningReason)

		// Explicit absence for the day wins.
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[sickTomorrow.ID].DayPlanningStatus)
		assert.Equal(t, "sick", byID[sickTomorrow.ID].DayPlanningReason)

		// Being present right now is not a plan for tomorrow.
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[walkInToday.ID].DayPlanningStatus)
		assert.Equal(t, "no_plan", byID[walkInToday.ID].DayPlanningReason)

		// Today's sick flag does not leak into tomorrow; the Tuesday plan counts.
		assert.Equal(t, students.DayPlanningStatusComesToday, byID[sickTodayPlannedTomorrow.ID].DayPlanningStatus)
		assert.Equal(t, "pickup_schedule", byID[sickTodayPlannedTomorrow.ID].DayPlanningReason)
	})

	t.Run("day_status_filter_applies_to_requested_date", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-02&day_status=comes_today&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())
		require.Len(t, byID, 2)
		_, hasTuesday := byID[plannedTomorrow.ID]
		_, hasSickToday := byID[sickTodayPlannedTomorrow.ID]
		assert.True(t, hasTuesday)
		assert.True(t, hasSickToday)
	})

	t.Run("same_computation_when_date_names_today", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-01&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentsByID(t, rr.Body.Bytes())
		// Monday plan applies today, walk-in counts today, today's sick flag counts.
		assert.Equal(t, students.DayPlanningStatusComesToday, byID[plannedMondayOnly.ID].DayPlanningStatus)
		assert.Equal(t, students.DayPlanningStatusComesToday, byID[walkInToday.ID].DayPlanningStatus)
		assert.Equal(t, "unplanned_attendance", byID[walkInToday.ID].DayPlanningReason)
		assert.Equal(t, students.DayPlanningStatusNotComingToday, byID[sickTodayPlannedTomorrow.ID].DayPlanningStatus)
		assert.Equal(t, "sick", byID[sickTodayPlannedTomorrow.ID].DayPlanningReason)
	})

	t.Run("invalid_date_is_rejected", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=02.06.2026", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
	})

	t.Run("date_beyond_planning_horizon_is_rejected", func(t *testing.T) {
		// Fixed now is Monday 2026-06-01, so the horizon ends with the Sunday
		// closing the current calendar week (2026-06-07); the Monday after is
		// out — that week is not materialized yet under the default cadence.
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-08", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())

		req = testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-07&page_size=50", schoolClass), nil)
		rr = authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	})
}

// studentLiveLocationTestResponse decodes just the live-location snapshot the
// list ships, so the assertions below read the exact fields #1939 strips for a
// non-today planning date.
type studentLiveLocationTestResponse struct {
	ID            int64      `json:"id"`
	Location      string     `json:"current_location"`
	LocationSince *time.Time `json:"location_since"`
	RoomColor     *string    `json:"current_room_color"`
}

func decodeStudentLocationsByID(t *testing.T, body []byte) map[int64]studentLiveLocationTestResponse {
	t.Helper()
	var resp struct {
		Data []studentLiveLocationTestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	byID := make(map[int64]studentLiveLocationTestResponse, len(resp.Data))
	for _, student := range resp.Data {
		byID[student.ID] = student
	}
	return byID
}

// TestListStudents_LiveStateForPlanningDate covers the two ways today's live
// presence state could still reach a non-today planning list: through the
// live-only filters (room_id / location_state / location), which resolve via
// active.visits, and through the current-location snapshot on the response.
// Fixed now is Monday 2026-06-01; the planning day is Tuesday 2026-06-02.
func TestListStudents_LiveStateForPlanningDate(t *testing.T) {
	tc := setupTestContext(t)
	fixedNow := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	tc.resource.Now = func() time.Time { return fixedNow }

	schoolClass := fmt.Sprintf("LS-%d", time.Now().UnixNano())
	checkedIn := testpkg.CreateTestStudent(t, tc.db, "LiveState", "CheckedIn", schoolClass)
	staff := testpkg.CreateTestStaff(t, tc.db, "LiveState", "Creator")
	device := testpkg.CreateTestDevice(t, tc.db, "live-state-device")
	defer testpkg.CleanupActivityFixtures(t, tc.db, checkedIn.ID, staff.ID, device.ID)

	testpkg.CreateTestAttendance(t, tc.db, checkedIn.ID, staff.ID, device.ID, time.Now().Add(-30*time.Minute), nil)

	t.Run("today_still_reports_the_live_location", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentLocationsByID(t, rr.Body.Bytes())
		require.Contains(t, byID, checkedIn.ID)
		assert.NotEmpty(t, byID[checkedIn.ID].Location, "today must keep the live location")
	})

	t.Run("planning_date_strips_the_live_location", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-02&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		byID := decodeStudentLocationsByID(t, rr.Body.Bytes())
		require.Contains(t, byID, checkedIn.ID)
		assert.Empty(t, byID[checkedIn.ID].Location, "a planning date must not ship today's whereabouts")
		assert.Nil(t, byID[checkedIn.ID].LocationSince)
		assert.Nil(t, byID[checkedIn.ID].RoomColor)
	})

	t.Run("planning_date_rejects_live_only_filters", func(t *testing.T) {
		for _, query := range []string{
			"room_id=1",
			"location_state=present",
			"location=Anwesend",
		} {
			req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-02&%s", schoolClass, query), nil)
			rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
			assert.Equal(t, http.StatusBadRequest, rr.Code, "query %q, body: %s", query, rr.Body.String())
		}
	})

	t.Run("today_accepts_live_only_filters", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/?school_class=%s&date=2026-06-01&location_state=present&page_size=50", schoolClass), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	})
}
