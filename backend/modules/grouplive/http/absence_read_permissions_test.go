package grouplivehttp_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGroupLiveHTTPAbsencesRequireUsersRead(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Gast", "Konto")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Gastgruppe")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	for _, status := range []string{"sick", "excused", "class_trip"} {
		child := testpkg.CreateTestStudent(t, tc.db, status, "Kind", "2a")
		testpkg.AssignStudentToGroup(t, tc.db, child.ID, group.ID)
		testpkg.CreateTestStudentStatusDay(t, tc.db, child.ID, calendar.Date("2026-08-24"), status)
	}
	for _, granted := range [][]string{{"students:read"}, {"users:read"}, {"users:*"}, {"admin:*"}} {
		rr := authExec(t, tc, testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil), testutil.TeacherTestClaims(int(account.ID)), granted)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		students := decodeOGSLive(t, rr.Body.Bytes()).Data.Students
		require.Len(t, students, 3)
		for _, student := range students {
			require.Equal(t, "2a", student["school_class"])
			for _, field := range []string{"sick", "excused", "class_trip"} {
				visible := granted[0] != "students:read" && field == student["first_name"]
				require.Equal(t, visible, student[field], "%s %v", field, granted)
				if visible {
					require.NotEmpty(t, student[field+"_since"])
				} else {
					require.NotContains(t, student, field+"_since")
				}
			}
		}
	}
}
