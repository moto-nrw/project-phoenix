package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOfferingChangeNotFoundHTTP(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Missing", "Reviewer")
	var missingID int64
	require.NoError(t, tc.db.NewRaw("SELECT nextval(pg_get_serial_sequence('enrollment.offering_change_requests', 'id'))").Scan(t.Context(), &missingID))
	var foreignID int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign := setupStudentsRoute(t, fixedCalendarClock)
		student := testpkg.CreateTestStudent(t, foreign.db, "Foreign", "Request", "3a")
		_, guardian := testpkg.CreateTestTeacherWithAccount(t, foreign.db, "Foreign", "Submitter")
		fixture := setupCorrectionFixture(t, foreign, student.ID, testpkg.Tenant(t), "Foreign")
		foreignID = insertPendingOfferingChangeRequest(t, foreign, fixture, student.ID, guardian.ID).ID
	})
	for name, id := range map[string]int64{"missing": missingID, "foreign": foreignID} {
		t.Run(name, func(t *testing.T) {
			for _, operation := range []struct{ name, path, body string }{
				{"approve", "/offering-change-requests/%d/decide", `{"approve":true,"effective_from":"2026-09-01","reason":"Approved"}`},
				{"reject", "/offering-change-requests/%d/decide", `{"approve":false,"reason":"Rejected"}`},
				{"mark done", "/change-requests/offering/%d/mark-done", `{}`},
				{"preview", "/offering-change-requests/%d/preview", `{}`},
			} {
				t.Run(operation.name, func(t *testing.T) {
					rr := authExec(t, tc, testutil.NewRequest(http.MethodPost, fmt.Sprintf(operation.path, id), strings.NewReader(operation.body)),
						testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
					require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
					var problem struct {
						Code string `json:"code"`
					}
					require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
					require.Equal(t, "general.input", problem.Code)
				})
			}
		})
	}
}
