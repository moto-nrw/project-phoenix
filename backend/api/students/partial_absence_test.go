package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCreatePartialAbsenceAppliesOnlyToBlocksStartingAfterTheChosenTime(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Partial", "Absence", "PA1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Partial", "Teacher")
	room := testpkg.CreateTestRoom(t, tc.db, "Partial absence room")

	date := timezone.NewDate(2026, 9, 15)
	morning := testpkg.CreateTestActivityInstance(t, tc.db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "08:00",
		EndHHMM:   "09:00",
		Title:     "Randstunde",
	})
	overlap := testpkg.CreateTestActivityInstance(t, tc.db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:30",
		EndHHMM:   "14:00",
		Title:     "Lernzeit",
	})
	afternoon := testpkg.CreateTestActivityInstance(t, tc.db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00",
		EndHHMM:   "15:00",
		Title:     "Ganztag",
	})
	actual := testpkg.CreateTestActivityInstance(t, tc.db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00",
		EndHHMM:   "16:00",
		Title:     "AG",
	})

	morningRow := testpkg.CreateTestInstanceStudent(t, tc.db, morning.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	overlapRow := testpkg.CreateTestInstanceStudent(t, tc.db, overlap.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	afternoonRow := testpkg.CreateTestInstanceStudent(t, tc.db, afternoon.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	checkedInAt := time.Date(2026, 9, 15, 15, 5, 0, 0, timezone.Berlin)
	actualRow := testpkg.CreateTestInstanceStudent(
		t, tc.db, actual.ID, student.ID, scheduleModel.AttendanceStatusPresent,
		testpkg.InstanceStudentOpts{CheckedInAt: &checkedInAt},
	)

	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/partial-absences", student.ID), map[string]any{
		"date":      date.String(),
		"from_time": "13:30",
		"reason":    "Arzttermin",
	})
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"from_time":"13:30"`)
	assert.Contains(t, rr.Body.String(), `"reason":"Arzttermin"`)

	rows := loadAttendanceRows(t, tc, morningRow.ID, overlapRow.ID, afternoonRow.ID, actualRow.ID)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, rows[morningRow.ID].Status)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, rows[overlapRow.ID].Status)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, rows[afternoonRow.ID].Status)
	require.NotNil(t, rows[afternoonRow.ID].Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, *rows[afternoonRow.ID].Substatus)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, rows[actualRow.ID].Status)
	assert.True(t, checkedInAt.Equal(*rows[actualRow.ID].CheckedInAt), "actual check-in instant must be preserved")

	partialAbsenceID := partialAbsenceResponseID(t, rr.Body.Bytes())
	updateReq := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/%d/partial-absences/%d", student.ID, partialAbsenceID), map[string]any{
		"date":      date.String(),
		"from_time": "14:30",
		"reason":    "Späterer Arzttermin",
	})
	updateRR := authExec(t, tc, updateReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, updateRR.Code, updateRR.Body.String())

	rows = loadAttendanceRows(t, tc, morningRow.ID, overlapRow.ID, afternoonRow.ID, actualRow.ID)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, rows[afternoonRow.ID].Status, "update must restore the slot no longer covered")
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, rows[actualRow.ID].Status, "update must preserve actual attendance")

	deleteReq := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d/partial-absences/%d", student.ID, partialAbsenceID), nil)
	deleteRR := authExec(t, tc, deleteReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code, deleteRR.Body.String())

	rows = loadAttendanceRows(t, tc, morningRow.ID, overlapRow.ID, afternoonRow.ID, actualRow.ID)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, rows[afternoonRow.ID].Status)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, rows[actualRow.ID].Status, "delete must preserve actual attendance")
}

func TestPartialAbsencePreservesAnExplicitPickupOverride(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Pickup", "Override", "PA2")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Pickup", "Teacher")
	date := timezone.NewDate(2026, 9, 16)
	pickupReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/pickup-exceptions", student.ID), map[string]any{
		"exception_date": date.String(), "pickup_time": "16:00",
	})
	pickupRR := authExec(t, tc, pickupReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, pickupRR.Code, pickupRR.Body.String())
	exceptionID := partialAbsenceResponseID(t, pickupRR.Body.Bytes())

	createReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/partial-absences", student.ID), map[string]any{
		"date": date.String(), "from_time": "13:30", "reason": "Termin",
	})
	createRR := authExec(t, tc, createReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())
	assert.Contains(t, createRR.Body.String(), `"pickup_time":"16:00"`)

	deleteReq := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d/partial-absences/%d", student.ID, exceptionID), nil)
	deleteRR := authExec(t, tc, deleteReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code, deleteRR.Body.String())

	fresh, err := tc.resource.PickupScheduleService.GetStudentPickupExceptionByID(testpkg.Ctx(t), exceptionID)
	require.NoError(t, err)
	require.NotNil(t, fresh)
	require.NotNil(t, fresh.PickupTime)
	assert.Equal(t, "16:00", fresh.PickupTime.Format("15:04"))
	assert.Nil(t, fresh.ExcusedFrom)
}

func TestFullDayStatusDoesNotOverwritePartialAbsence(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Conflict", "Partial", "PA3")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Conflict", "Teacher")
	date := timezone.NewDate(2026, 9, 17)

	partialReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/partial-absences", student.ID), map[string]any{
		"date": date.String(), "from_time": "13:30",
	})
	partialRR := authExec(t, tc, partialReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, partialRR.Code, partialRR.Body.String())

	statusReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "excused", "dates": []string{date.String()},
	})
	statusRR := authExec(t, tc, statusReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, statusRR.Code, statusRR.Body.String())
	assert.Contains(t, statusRR.Body.String(), "partial absence")

	rows, err := tc.resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, date, date)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestPartialAbsenceIsAPlannedPickupTimeWithoutAFullDayStatus(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	// Pin the clock to a fixed Monday: the effective-time resolver skips
	// Saturday/Sunday entirely (weekday > WeekdayFriday), so a real-today
	// date makes this test fail on every weekend CI run.
	tc.resource.Now = func() time.Time { return time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC) }
	student := testpkg.CreateTestStudent(t, tc.db, "Planning", "Partial", "PA4")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Planning", "Teacher")
	date := timezone.NewDate(2026, 6, 1)

	partialReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/partial-absences", student.ID), map[string]any{
		"date": date.String(), "from_time": "13:30", "reason": "Arzttermin",
	})
	partialRR := authExec(t, tc, partialReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, partialRR.Code, partialRR.Body.String())

	listReq := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?date="+date.String()+"&include_pickup_times=true", nil)
	listRR := authExec(t, tc, listReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, listRR.Code, listRR.Body.String())
	var response struct {
		Data []struct {
			ID                int64   `json:"id"`
			PickupTime        *string `json:"pickup_time"`
			PickupIsException bool    `json:"pickup_is_exception"`
			Excused           bool    `json:"excused"`
			DayPlanningStatus string  `json:"day_planning_status"`
			DayPlanningReason string  `json:"day_planning_reason"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &response))
	var found bool
	for _, row := range response.Data {
		if row.ID != student.ID {
			continue
		}
		found = true
		require.NotNil(t, row.PickupTime, listRR.Body.String())
		assert.Equal(t, "13:30", *row.PickupTime)
		assert.True(t, row.PickupIsException)
		assert.False(t, row.Excused, "partial absence must not count as a full-day status")
		assert.Equal(t, "comes_today", row.DayPlanningStatus)
		assert.Equal(t, "pickup_exception", row.DayPlanningReason)
	}
	assert.True(t, found)
}

func partialAbsenceResponseID(t *testing.T, body []byte) int64 {
	t.Helper()
	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	require.Positive(t, response.Data.ID)
	return response.Data.ID
}

func loadAttendanceRows(t *testing.T, tc *testContext, ids ...int64) map[int64]*scheduleModel.InstanceStudent {
	t.Helper()
	rows := make([]*scheduleModel.InstanceStudent, 0, len(ids))
	require.NoError(t, tc.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".id IN (?)`, bun.List(ids)).
		Scan(context.Background()))

	byID := make(map[int64]*scheduleModel.InstanceStudent, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	return byID
}
