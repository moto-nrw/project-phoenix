package presenceprojectionhttp_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// A holiday guest of the hosting school stands in "Alle Kinder" only from the
// first day of its week (#5630, ADR 0120 Nachtrag zu Punkt 2), on the list
// the page really reads: GET /api/students. The school's own child scheduled
// for a later start waits for it as well (#6041), and the administrative view
// that asks for every child still sees both.
func TestStudentListShowsAHolidayGuestOnlyFromTheFirstDayOfItsWeek(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	today := studentsTestToday // Monday 24.08.2026, the day the handler sees

	waiting := testpkg.CreateTestStudent(t, tc.db, "Wartet", "Feriengast", peopledirectory.HolidayGuestSchoolClass)
	nextMonday, nextFriday := today.AddDays(7), today.AddDays(11)
	testpkg.SetStudentLifecycle(t, tc.db, waiting.ID, testpkg.StudentStatusPending, &nextMonday, &nextFriday)
	arrived := testpkg.CreateTestStudent(t, tc.db, "Da", "Feriengast", peopledirectory.HolidayGuestSchoolClass)
	friday := today.AddDays(4)
	testpkg.SetStudentLifecycle(t, tc.db, arrived.ID, testpkg.StudentStatusActive, &today, &friday)
	ownLater := testpkg.CreateTestStudent(t, tc.db, "Kommt", "Spaeter", "1a")
	testpkg.SetStudentLifecycle(t, tc.db, ownLater.ID, testpkg.StudentStatusPending, &nextMonday, nil)
	own := testpkg.CreateTestStudent(t, tc.db, "Ist", "Da", "1a")
	// A guest someone checked in before its week stays, like any child
	// actually present: the list never hides a child in the building.
	checkedIn := testpkg.CreateTestStudent(t, tc.db, "Schon", "Feriengast", peopledirectory.HolidayGuestSchoolClass)
	testpkg.SetStudentLifecycle(t, tc.db, checkedIn.ID, testpkg.StudentStatusPending, &nextMonday, &nextFriday)
	staff := testpkg.CreateTestStaff(t, tc.db, "Betreuung", "Feriengast")
	device := testpkg.CreateTestDevice(t, tc.db, "feriengast-liste-geraet")
	testpkg.CreateTestAttendanceForDate(t, tc.db, checkedIn.ID, staff.ID, device.ID, today, fixedCalendarClock(), nil)

	actor := testpkg.CreateTestAccount(t, tc.db, "feriengast-liste@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	listed := func(query string) map[int64]bool {
		t.Helper()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
		var body struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		ids := map[int64]bool{}
		for _, row := range body.Data {
			ids[row.ID] = true
		}
		return ids
	}

	rows := listed("?page_size=500")
	assert.False(t, rows[waiting.ID], "before its week the holiday guest is not in the child list")
	assert.True(t, rows[arrived.ID], "from the first day of its week the holiday guest is in the child list")
	// Since #6041 a scheduled own child also waits for its care start.
	assert.False(t, rows[ownLater.ID], "an own child scheduled for a later start is not in the running list yet")
	assert.True(t, rows[own.ID])
	assert.True(t, rows[checkedIn.ID], "a holiday guest actually present stays in the child list")

	all := listed("?page_size=500&care_status=all")
	assert.True(t, all[waiting.ID], "the administrative view of every child keeps the waiting guest")
	assert.True(t, all[arrived.ID])
	assert.True(t, all[ownLater.ID], "the administrative view keeps the scheduled own child")
}
