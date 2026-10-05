package students_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type liftedStatusResponse struct {
	Data struct {
		Sick    bool `json:"sick"`
		Excused bool `json:"excused"`
	} `json:"data"`
}

func decodeLiftedStatus(t *testing.T, body []byte) liftedStatusResponse {
	t.Helper()
	var resp liftedStatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp
}

// A parent's Abmeldung for today is an active status day without the live
// flag (#1735). Lifting the status in the profile must clear that row, free
// the blocks it owns and leave future days alone (#3854).
func TestUpdateStudent_LiftClearsStatusDayWithoutLiveFlag(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 9, 9)
	tc := setupStudentsRoute(t, func() time.Time { return today.BerlinMidnight().Add(10 * time.Hour) })

	for _, status := range []string{absencerecords.StudentStatusDayExcused, absencerecords.StudentStatusDaySick} {
		t.Run(status, func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, tc.db, "Lift", "Parent"+status, "LP1")
			todayRow := testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, today, status)
			_, err := tc.db.NewUpdate().Model((*testpkg.StudentStatusDayRow)(nil)).
				ModelTableExpr(`active.student_status_days`).
				Set("source = ?", absencerecords.StudentStatusSourceParent).
				Where("id = ?", todayRow.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			tomorrowRow := testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, today.AddDays(1), status)

			room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Lift-%s-%d", status, student.ID))
			instance := testpkg.CreateTestActivityInstance(t, tc.db, today, room.ID, testpkg.ActivityInstanceOpts{})
			participant := testpkg.CreateTestInstanceStudent(t, tc.db, instance.ID, student.ID, "absent",
				testpkg.InstanceStudentOpts{StudentStatusDayID: &todayRow.ID})

			detail := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/%d", student.ID), nil),
				testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
			before := decodeLiftedStatus(t, detail.Body.Bytes())
			require.True(t, before.Data.Excused || before.Data.Sick, "the parent row shows as today's status")

			lift := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
				map[string]any{status: false}), testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusOK, lift.Code, lift.Body.String())
			lifted := decodeLiftedStatus(t, lift.Body.Bytes())
			assert.False(t, lifted.Data.Excused, "the PUT response reports the effective status")
			assert.False(t, lifted.Data.Sick, "the PUT response reports the effective status")

			detail = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/%d", student.ID), nil),
				testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
			after := decodeLiftedStatus(t, detail.Body.Bytes())
			assert.False(t, after.Data.Excused, "today's parent row no longer shows")
			assert.False(t, after.Data.Sick, "today's parent row no longer shows")

			var clearedToday testpkg.StudentStatusDayRow
			require.NoError(t, tc.db.NewSelect().Model(&clearedToday).
				ModelTableExpr(`active.student_status_days AS "student_status_day"`).
				Where(`"student_status_day".id = ?`, todayRow.ID).Scan(testpkg.Ctx(t)))
			assert.NotNil(t, clearedToday.ClearedAt, "today's parent row is cleared")

			var future testpkg.StudentStatusDayRow
			require.NoError(t, tc.db.NewSelect().Model(&future).
				ModelTableExpr(`active.student_status_days AS "student_status_day"`).
				Where(`"student_status_day".id = ?`, tomorrowRow.ID).Scan(testpkg.Ctx(t)))
			assert.Nil(t, future.ClearedAt, "a planned future day stays")

			var block struct {
				Status             string `bun:"status"`
				StudentStatusDayID *int64 `bun:"student_status_day_id"`
			}
			require.NoError(t, tc.db.NewSelect().TableExpr("active.activity_session_attendance").
				Column("status", "student_status_day_id").
				Where("instance_student_id = ?", participant.ID).Scan(testpkg.Ctx(t), &block))
			assert.Equal(t, "expected", block.Status, "the block the parent row owned is released")
			assert.Nil(t, block.StudentStatusDayID)
		})
	}
}

func TestUpdateStudent_LiftUsesSameClockForResponse(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 9, 9)
	beforeMidnight := date.BerlinMidnight().Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	afterMidnight := beforeMidnight.Add(time.Second)
	tc := setupStudentsRoute(t, func() time.Time { return beforeMidnight })
	student := testpkg.CreateTestStudent(t, tc.db, "Lift", "Midnight", "LM1")
	todayRow := testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, date, absencerecords.StudentStatusDayExcused)
	tomorrowRow := testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, date.AddDays(1), absencerecords.StudentStatusDayExcused)

	clockCalls := 0
	tc.resource.Now = func() time.Time {
		clockCalls++
		if clockCalls == 1 {
			return beforeMidnight
		}
		return afterMidnight
	}

	lift := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
		map[string]any{"excused": false}), testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, lift.Code, lift.Body.String())
	assert.False(t, decodeLiftedStatus(t, lift.Body.Bytes()).Data.Excused, "the response verifies the day that was lifted")
	assert.Equal(t, 1, clockCalls, "the update captures its clock once")

	rows, err := tc.resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, date, date.AddDays(1))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, tomorrowRow.ID, rows[0].ID, "tomorrow's planned status remains active")

	var clearedToday testpkg.StudentStatusDayRow
	require.NoError(t, tc.db.NewSelect().Model(&clearedToday).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Where(`"student_status_day".id = ?`, todayRow.ID).Scan(testpkg.Ctx(t)))
	assert.NotNil(t, clearedToday.ClearedAt, "the status active when the request began is cleared")
}

// A response that cannot verify the effective status must fail the update and
// roll its status-day clear back. Otherwise the UI could claim a lift worked
// from a response which still only reflects the legacy live flag (#3854).
func TestUpdateStudent_LiftFailsWhenStatusDayResponseCannotBeVerified(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 9, 9)
	tc := setupStudentsRoute(t, func() time.Time { return today.BerlinMidnight().Add(10 * time.Hour) })
	student := testpkg.CreateTestStudent(t, tc.db, "Lift", "Verification", "LV1")
	statusDay := testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, today, absencerecords.StudentStatusDayExcused)

	statusDays := tc.resource.StudentStatusDayService
	tc.resource.StudentStatusDayService = statusDayResponseFailure{
		StatusDays: statusDays,
		err:        errors.New("status day response lookup failed"),
	}

	lift := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
		map[string]any{"excused": false}), testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, lift.Code, lift.Body.String())

	tc.resource.StudentStatusDayService = statusDays
	rows, err := statusDays.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, today, today)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, statusDay.ID, rows[0].ID, "the failed response verification rolls back the lift")
}

type statusDayResponseFailure struct {
	studentpresence.StatusDays
	err error
}

func (s statusDayResponseFailure) GetActiveByStudentAndDateRange(
	_ context.Context,
	_ int64,
	_, _ timezone.Date,
) ([]*absencerecords.StudentStatusDay, error) {
	return nil, s.err
}
