package grouplivehttp_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOGSGroupLiveGuestCannotReadPlanningNotes(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	setOGSLiveOverviewScope(t, tc, settings.OverviewScopeAllStaff)
	testpkg.SetTestTenantSetting(t, tc.db, testpkg.Tenant(t), "operations.student_photos_enabled", "true")
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Gast", "Konto")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Gastgruppe")
	child := testpkg.CreateTestStudent(t, tc.db, "Mila", "Kind", "2a")
	testpkg.AssignStudentToGroup(t, tc.db, child.ID, group.ID)
	testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, 1, teacher.StaffID, "13:00")
	testpkg.CreateTestArrivalException(t, tc.db, child.ID, calendar.Date("2026-08-24"), teacher.StaffID, "13:30", "Ankunft vertraulich")
	testpkg.CreateTestPickupScheduleWithNote(t, tc.db, child.ID, 1, teacher.StaffID, "15:30", "Abholung vertraulich")
	_, err := tc.db.NewUpdate().TableExpr("users.student_profiles").
		Set("photo_path = ?", "/uploads/student-photos/guest-test.jpg").Where("id = ?", child.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	for _, granted := range [][]string{
		{"students:read", "groups:read"}, {"users:read", "groups:read"}, {"users:*", "groups:read"}, {"admin:*"}, {"*:*"},
	} {
		req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), granted)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		data := decodeOGSLive(t, rr.Body.Bytes()).Data
		require.Len(t, data.Students, 1)
		student := data.Students[0]
		require.Equal(t, "Mila", student["first_name"])
		require.Equal(t, "2a", student["school_class"])
		require.NotEmpty(t, student["photo_url"], "photos remain available independently of planning permissions")
		if granted[0] == "students:read" {
			require.NotContains(t, rr.Body.String(), "vertraulich")
			require.NotContains(t, student, "arrival_notes")
			require.NotContains(t, student, "day_planning_label")
			require.Empty(t, data.PickupTimes)
		} else {
			require.Equal(t, "Ankunft vertraulich", student["arrival_notes"])
			require.Len(t, data.PickupTimes, 1)
			require.Equal(t, "Abholung vertraulich", data.PickupTimes[0]["notes"])
		}
	}
}
