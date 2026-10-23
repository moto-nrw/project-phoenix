package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentHTTPAbsencesRequireUsersRead(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Gast", "Konto")
	for _, status := range []string{absencerecords.StudentStatusDaySick, absencerecords.StudentStatusDayExcused, absencerecords.StudentStatusDayClassTrip} {
		child := testpkg.CreateTestStudent(t, tc.db, status, "Kind", status)
		for _, day := range []int{0, 1} {
			testpkg.CreateTestStudentStatusDay(t, tc.db, child.ID, studentsTestToday.AddDays(day), status)
		}
		for _, granted := range [][]string{{"students:read"}, {"users:read"}, {"users:*"}, {"admin:*"}} {
			for _, path := range []string{
				fmt.Sprintf("%d", child.ID),
				fmt.Sprintf("?school_class=%s", status),
				fmt.Sprintf("?school_class=%s&date=%s", status, studentsTestToday.AddDays(1)),
				fmt.Sprintf("?school_class=%s&date=%s&view=slim", status, studentsTestToday.AddDays(1)),
			} {
				rr := authExec(t, tc, testutil.NewRequest("GET", "/"+path, nil), testutil.TeacherTestClaims(int(account.ID)), granted)
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var body struct {
					Data json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
				var student map[string]any
				if path[0] != '?' {
					require.NoError(t, json.Unmarshal(body.Data, &student))
				} else {
					var rows []map[string]any
					require.NoError(t, json.Unmarshal(body.Data, &rows))
					require.Len(t, rows, 1)
					student = rows[0]
				}
				require.Equal(t, status, student["first_name"])
				for _, field := range []string{"sick", "excused", "class_trip"} {
					visible := granted[0] != "students:read" && field == status
					require.Equal(t, visible, student[field], "%s %s %v", path, field, granted)
					if visible {
						require.NotEmpty(t, student[field+"_since"])
					} else {
						require.NotContains(t, student, field+"_since")
					}
				}
			}
		}
	}
}
