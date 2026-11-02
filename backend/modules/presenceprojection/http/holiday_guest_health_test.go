package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayGuestHealthFieldEndsWithItsWeek: the school sees the health field
// of a holiday guest only up to the last day of its week (#4722), even while
// the hourly reconciliation has not cleared it yet. The archive (users:delete)
// still reads the ended guest; the school's own child keeps its health field
// after its care ended.
func TestHolidayGuestHealthFieldEndsWithItsWeek(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	child := func(first, class string, until calendar.Date) int64 {
		student := testpkg.CreateTestStudent(t, tc.db, first, "Sonne", class)
		_, err := tc.db.NewRaw(`UPDATE users.student_care_profiles SET health_info = ?
			WHERE membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)`,
			"Asthmaspray-"+first, student.ID).Exec(testpkg.Ctx(t))
		require.NoError(t, err)
		setEnrolledUntil(t, tc, student.ID, until)
		return student.ID
	}
	weekOver := calendar.NewDate(2020, 7, 10)
	ended := child("Finn", peopledirectory.HolidayGuestSchoolClass, weekOver)
	running := child("Mila", peopledirectory.HolidayGuestSchoolClass, calendar.NewDate(2099, 7, 10))
	own := child("Lia", "3a", weekOver)
	actor := testpkg.CreateTestAccount(t, tc.db, "holiday-health-reader@example.test")

	health := func(student int64) string {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d", student), nil),
			testutil.AdminTestClaims(int(actor.ID)), []string{"users:read", "users:delete"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data struct {
				HealthInfo string `json:"health_info"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		return body.Data.HealthInfo
	}

	assert.Empty(t, health(ended), "after the last day of the week the guest's health field reads empty")
	assert.Equal(t, "Asthmaspray-Mila", health(running), "during the week the school sees it")
	assert.Equal(t, "Asthmaspray-Lia", health(own), "the school's own child keeps its health field after its care")
}
