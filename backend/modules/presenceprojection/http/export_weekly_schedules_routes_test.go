package presenceprojectionhttp_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentExportKeepsSelectedScheduleCells(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Schedule", "Teacher")
	child := testpkg.CreateTestStudent(t, tc.db, "Weekly", "Export", "S29")
	testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, 1, staff.ID, "08:15")
	testpkg.CreateTestPickupSchedule(t, tc.db, child.ID, 1, staff.ID, "16:15")
	testpkg.CreateTestPickupSchedule(t, tc.db, child.ID, 5, staff.ID, "15:00")
	for _, format := range []string{"csv", "xlsx", "pdf"} {
		for _, selection := range []struct {
			preset, column, value string
		}{
			{"pickup_list", "planned_pickup", "16:15"},
			{"ogs_weekly", "care_days", "Mo, Fr"},
			{"ogs_weekly", "weekly_monday", "Ankunft: 08:15, Abholung: 16:15"},
			{"ogs_weekly", "weekly_friday", "Abholung: 15:00"},
			{"class_roster", "weekly_monday", "16:15 Uhr, geht alleine"},
		} {
			t.Run(format+"/"+selection.preset+"/"+selection.column, func(t *testing.T) {
				req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/export", map[string]any{
					"format": format, "preset": selection.preset, "columns": []string{"name", selection.column},
					"filters": map[string]string{"school_class": "S29", "date": studentsTestToday.String()},
				})
				rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"users:read"})
				if format == "csv" {
					// This route rejects CSV even though the shared renderer supports it.
					require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
					require.Contains(t, rr.Body.String(), `"code":"general.input"`)
					require.Contains(t, rr.Body.String(), `unsupported export format \"csv\"`)
					return
				}
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				require.Contains(t, rr.Header().Get("Content-Disposition"), "."+format)
				if format == "pdf" {
					require.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
					require.True(t, strings.HasPrefix(rr.Body.String(), "%PDF-"))
					return
				}
				rows := students20WorkbookRows(t, rr.Body.Bytes())
				require.Contains(t, rows, []string{"Weekly Export", selection.value})
			})
		}
	}
}
