// Router-level test for GET /api/staff/{id}/time-tracking/month-summary
// (#1842): pins auth wiring, query validation, and the response envelope.
package timetracking

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffMonthSummaryAPI(t *testing.T) {
	t.Parallel()

	tc := setupStaffRoute(t)
	suffix := time.Now().UnixNano()

	editorPerson, editorAccount := testpkg.CreateTestPersonWithAccount(t, tc.db, "Summary", fmt.Sprintf("Editor-%d", suffix))
	testpkg.CreateTestStaffForPerson(t, tc.db, editorPerson.ID)
	subject := testpkg.CreateTestStaff(t, tc.db, "Summary", fmt.Sprintf("Subject-%d", suffix))

	claims := testutil.DefaultTestClaims()
	claims.ID = int(editorAccount.ID)
	claims.Permissions = []string{"time_tracking:manage"}
	token := testutil.MintTestJWT(t, claims)

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		tc.router.ServeHTTP(rec, req)
		return rec
	}

	rec := get(fmt.Sprintf("/staff/%d/time-tracking/month-summary?year=2026&month=6", subject.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"carry_in_minutes"`)
	assert.Contains(t, rec.Body.String(), `"closing_balance_minutes"`)

	rec = get(fmt.Sprintf("/staff/%d/time-tracking/month-summary?year=2026&month=13", subject.ID))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = get(fmt.Sprintf("/staff/%d/time-tracking/month-summary", subject.ID))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
