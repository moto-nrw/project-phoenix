package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentList_UsesBookingParticipationButKeepsAdministrationAndLivePresence(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, true, fixedCalendarClock)
	student := testpkg.CreateTestStudent(t, tc.db, "Sichtbar", "Grenze", "4c")
	endedWithoutTask := testpkg.CreateTestStudent(t, tc.db, "Ohne", "Aufgabe", "4d")
	studentID := student.ID
	today := calendar.NewDate(2026, 8, 24)
	// The gap must be a future day inside the planning horizon, which closes
	// on the Sunday of the current week (day_planning.go: maxPlanningDate).
	// fixedCalendarClock pins today to a Monday, so Tuesday is such a day.
	require.Equal(t, time.Monday, today.Weekday())
	setEnrolledUntil(t, tc, endedWithoutTask.ID, today.AddDays(-1))
	firstGap := today.AddDays(1)
	upsertNaturalCompletion(t, tc.db, studentID, firstGap)
	actor := testpkg.CreateTestAccount(t, tc.db, "participation-reader@example.test")
	claims := testutil.AdminTestClaims(int(actor.ID))

	listedIDs := func(query string) map[int64]bool { return listedCareStudentIDs(t, tc, claims, query) }

	assert.True(t, listedIDs("")[student.ID], "the child remains visible before the gap")
	assert.False(t, listedIDs("&include_pending_withdrawals=true")[endedWithoutTask.ID], "the administration exception must not restore every ended child")

	assert.False(t, listedIDs("&date=" + firstGap.String())[student.ID], "the operational list hides the child from the first bookingless day")
	assert.True(t, listedIDs("&date=" + firstGap.String() + "&include_pending_withdrawals=true")[student.ID], "master-data administration keeps the open task reachable")

	readOnlyRequest := testutil.NewAuthenticatedRequest(t, http.MethodGet,
		"/?page_size=500&date="+firstGap.String()+"&include_pending_withdrawals=true", nil)
	readOnlyResponse := authExec(t, tc, readOnlyRequest, claims, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, readOnlyResponse.Code,
		"the administrative exception must require users:delete")

	// Move the same pending task onto today to exercise the live-presence
	// exception without treating today's attendance as future planning data.
	upsertNaturalCompletion(t, tc.db, studentID, today)
	assert.False(t, listedIDs("")[student.ID])
	assert.True(t, listedIDs("&include_pending_withdrawals=true")[student.ID])

	staff := testpkg.CreateTestStaff(t, tc.db, "Live", "Aufsicht")
	device := testpkg.CreateTestDevice(t, tc.db, "participation-live-reader")
	setEnrolledUntil(t, tc, student.ID, today.AddDays(-1))
	testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, time.Now().Add(-time.Hour), nil)
	_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("status = ?", peopledirectory.StudentStatusAlumnus).Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(t.Context())
	require.NoError(t, err)
	assert.True(t, listedIDs("")[student.ID], "actual attendance overrides derived dates and technical status")
}

func upsertNaturalCompletion(
	t *testing.T, db bun.IDB, studentID int64, gap calendar.Date,
) {
	t.Helper()
	testpkg.CreateTestBookingExpiredCareWithdrawalOn(t, db, studentID, gap)
}

func listedCareStudentIDs(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, query string) map[int64]bool {
	t.Helper()
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?page_size=500"+query, nil)
	response := authExec(t, tc, request, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
	var body struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	ids := make(map[int64]bool, len(body.Data))
	for _, row := range body.Data {
		ids[row.ID] = true
	}
	return ids
}

func TestStudentList_CareStatusDecidesWhichSideIsShown(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)

	running := testpkg.CreateTestStudent(t, tc.db, "Listed", "Running", "3a")
	planned := testpkg.CreateTestStudent(t, tc.db, "Listed", "Planned", "3a")
	ended := testpkg.CreateTestStudent(t, tc.db, "Listed", "Ended", "3a")
	endsToday := testpkg.CreateTestStudent(t, tc.db, "Listed", "EndsToday", "3a")

	today := calendar.NewDate(2026, 8, 24)
	setEnrolledUntil(t, tc, planned.ID, today.AddDays(14))
	setEnrolledUntil(t, tc, ended.ID, today.AddDays(-1))
	setEnrolledUntil(t, tc, endsToday.ID, today)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CareStatusGroup")
	testpkg.AssignStudentToGroup(t, tc.db, running.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, planned.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, ended.ID, group.ID)

	actor := testpkg.CreateTestAccount(t, tc.db, "care-list@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))

	listed := func(query string) map[int64]bool {
		t.Helper()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
		var body struct {
			Data []struct {
				ID         int64  `json:"id"`
				CareEndsOn string `json:"care_ends_on"`
				CareEnded  bool   `json:"care_ended"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		out := map[int64]bool{}
		for _, row := range body.Data {
			out[row.ID] = row.CareEnded
		}
		return out
	}

	t.Run("the default list hides a child whose care has ended", func(t *testing.T) {
		rows := listed("?page_size=500")
		assert.Contains(t, rows, running.ID)
		assert.Contains(t, rows, planned.ID, "a PLANNED exit stays in the list")
		assert.NotContains(t, rows, ended.ID)
	})

	t.Run("care_status=ended shows exactly the other side", func(t *testing.T) {
		rows := listed("?page_size=500&care_status=ended")
		assert.Contains(t, rows, ended.ID)
		assert.True(t, rows[ended.ID], "the payload says the care has ended")
		assert.NotContains(t, rows, running.ID)
		assert.NotContains(t, rows, planned.ID)
	})

	t.Run("care_status=all shows both", func(t *testing.T) {
		rows := listed("?page_size=500&care_status=all")
		assert.Contains(t, rows, running.ID)
		assert.Contains(t, rows, planned.ID)
		assert.Contains(t, rows, ended.ID)
	})

	t.Run("a future planning day hides a child after its last care day", func(t *testing.T) {
		// Day planning stops at the Sunday closing the current week (#1939).
		// Today is the fixed Monday, so Tuesday lies inside that horizon.
		require.Equal(t, time.Monday, today.Weekday())
		planningDay := today.AddDays(1)
		rows := listed(fmt.Sprintf("?page_size=500&date=%s", planningDay))
		assert.Contains(t, rows, running.ID, "a child still in care stays listed on the planning day")
		assert.NotContains(t, rows, endsToday.ID)
	})

	t.Run("group list applies the care status before paging", func(t *testing.T) {
		endedRows := listed(fmt.Sprintf("?group_id=%d&page_size=500&care_status=ended", group.ID))
		assert.Contains(t, endedRows, ended.ID)
		assert.NotContains(t, endedRows, running.ID)
		assert.NotContains(t, endedRows, planned.ID)

		runningRows := listed(fmt.Sprintf("?group_id=%d&page_size=500", group.ID))
		assert.Contains(t, runningRows, running.ID)
		assert.Contains(t, runningRows, planned.ID)
		assert.NotContains(t, runningRows, ended.ID)
	})

	t.Run("default participation keeps SQL pagination and count aligned", func(t *testing.T) {
		query := fmt.Sprintf("group_id=%d&page_size=1&page=%%d", group.ID)
		first, firstTotal := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(query, 1))
		second, secondTotal := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(query, 2))
		require.Len(t, first, 1)
		require.Len(t, second, 1)
		assert.Equal(t, 2, firstTotal)
		assert.Equal(t, firstTotal, secondTotal)
		assert.NotEqual(t, first[0], second[0])
		assert.ElementsMatch(t, []int64{running.ID, planned.ID}, []int64{first[0], second[0]})
	})

	t.Run("an unknown care_status is rejected, not silently ignored", func(t *testing.T) {
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?care_status=vielleicht", nil)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	// The archive view sits behind users:delete. Without this the same set of
	// children is one query parameter away for anybody who may read the list
	// at all (#2487).
	t.Run("plain users:read cannot reach the departed side", func(t *testing.T) {
		readOnly := []string{"users:read"}
		for _, query := range []string{"?care_status=ended", "?care_status=all"} {
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil)
			response := authExec(t, tc, request, claims, readOnly)
			assert.Equal(t, http.StatusForbidden, response.Code, "query %s", query)
		}

		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?page_size=500", nil)
		response := authExec(t, tc, request, claims, readOnly)
		require.Equal(t, http.StatusOK, response.Code, "the ordinary list stays open")
	})

	t.Run("group-only archive view hides alumni", func(t *testing.T) {
		_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
			Set("status = ?", peopledirectory.StudentStatusAlumnus).
			Where("student_profile_id = ? AND deleted_at IS NULL", ended.ID).
			Exec(t.Context())
		require.NoError(t, err)

		rows := listed(fmt.Sprintf("?group_id=%d&page_size=500&care_status=all", group.ID))
		assert.NotContains(t, rows, ended.ID)
	})
}

// The list has to say WHY a child carries an end date: a recorded exit can be
// changed and cancelled here, the mere end of an enrolment phase cannot. Told
// apart by the exit row, never by how far the date lies ahead — a school that
// plans a departure for the end of the school year must still be able to take
// it back (#2487).
func TestStudentList_MarksRecordedExitsOnly(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRouteWithCareLifecycle(t, false)

	recorded := testpkg.CreateTestStudent(t, tc.db, "Flagged", "Recorded", "4a")
	phaseEnd := testpkg.CreateTestStudent(t, tc.db, "Flagged", "PhaseEnd", "4a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-flag@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	today := calendar.NewDate(2026, 8, 24)

	// Both carry the same end date, far ahead. Only one of them was entered
	// through "Betreuung beenden".
	farAhead := today.AddDays(200)
	setEnrolledUntil(t, tc, phaseEnd.ID, farAhead)
	confirmCareExitVia(t, tc, claims, recorded.ID, farAhead)

	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?page_size=500", nil)
	response := authExec(t, tc, request, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())

	var body struct {
		Data []struct {
			ID               int64  `json:"id"`
			CareEndsOn       string `json:"care_ends_on"`
			CareExitRecorded bool   `json:"care_exit_recorded"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))

	flags := map[int64]bool{}
	dates := map[int64]string{}
	for _, row := range body.Data {
		flags[row.ID] = row.CareExitRecorded
		dates[row.ID] = row.CareEndsOn
	}
	require.Contains(t, flags, recorded.ID, "a planned exit stays in the ordinary list")
	require.Contains(t, flags, phaseEnd.ID)
	assert.Equal(t, farAhead.String(), dates[recorded.ID])
	assert.Equal(t, farAhead.String(), dates[phaseEnd.ID])
	assert.True(t, flags[recorded.ID], "an entered exit can be changed and cancelled")
	assert.False(t, flags[phaseEnd.ID], "the end of an enrolment phase is not an exit")
}

// confirmCareExitVia ends a child's care through the real HTTP surface, so the
// exit row is written exactly the way the product writes it.
func confirmCareExitVia(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, studentID int64, lastCareDay calendar.Date) {
	t.Helper()
	body := map[string]any{
		"student_ids":   []string{fmt.Sprintf("%d", studentID)},
		"last_care_day": lastCareDay.String(),
		"reason":        careplan.CareExitReasonNoCareNeed,
	}
	previewRequest := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", body)
	previewResponse := authExec(t, tc, previewRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, previewResponse.Code, "Body: %s", previewResponse.Body.String())

	var preview struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewResponse.Body.Bytes(), &preview))
	body["token"] = preview.Data.Token

	confirmRequest := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end", body)
	confirmResponse := authExec(t, tc, confirmRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, confirmResponse.Code, "Body: %s", confirmResponse.Body.String())
}

func setEnrolledUntil(t *testing.T, tc *testContext, studentID int64, day calendar.Date) {
	t.Helper()
	_, err := tc.db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", day).
		Where("student_profile_id = ? AND deleted_at IS NULL", studentID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}
