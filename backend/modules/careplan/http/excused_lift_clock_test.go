package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
func TestUpdateStudent_LiftUsesSameClockForResponse(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, 9, 9)
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

	rows, err := tc.statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, date, date.AddDays(1))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, tomorrowRow.ID, rows[0].ID, "tomorrow's planned status remains active")

	var clearedToday testpkg.StudentStatusDayRow
	require.NoError(t, tc.db.NewSelect().Model(&clearedToday).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Where(`"student_status_day".id = ?`, todayRow.ID).Scan(testpkg.Ctx(t)))
	assert.NotNil(t, clearedToday.ClearedAt, "the status active when the request began is cleared")
}
