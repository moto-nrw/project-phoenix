package classdayhttp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// returnTimeTrip posts a one-day outing with a return time for the given
// children and returns the statements the request ran.
func (f *classTripFixture) returnTimeTrip(t *testing.T, counter *testpkg.QueryCounter, studentIDs []int64) (int, string, int) {
	t.Helper()
	ids := make([]string, 0, len(studentIDs))
	for _, id := range studentIDs {
		ids = append(ids, fmt.Sprint(id))
	}
	body := fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%s],"return_time":"13:30"}`, classTripMonday, classTripMonday, strings.Join(ids, ","))
	req := httptest.NewRequest(http.MethodPost, "/class-trips/"+f.class, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(counter.Context(req.Context()))
	counter.Reset()
	rec := testutil.ExecuteWithAuthPermissions(t, f.router, req, f.claims, []string{classTripWritePermission})
	return rec.Code, rec.Body.String(), counter.Total()
}

func (f *classTripFixture) classChildren(t *testing.T, count int) []int64 {
	t.Helper()
	ids := make([]int64, 0, count)
	for i := range count {
		ids = append(ids, testpkg.CreateTestStudent(t, f.db, "Kind", fmt.Sprintf("Ausflug%d", i), f.class).ID)
	}
	return ids
}

// A return time for several children without an arrival yet sets each
// child's arrival. The store scans every created row back; the children of
// one report share the return time, which must survive the first child.
func TestClassTripReturnTimeSetsEveryChildsArrival(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)
	counter := testpkg.CaptureQueriesForContext(t, f.db)
	children := f.classChildren(t, 3)
	code, resp, _ := f.returnTimeTrip(t, counter, children)
	require.Equal(t, http.StatusCreated, code, resp)
	for _, id := range children {
		arrival, reason, ok := f.arrival(t, id, classTripMonday)
		require.True(t, ok, "child %d", id)
		assert.Equal(t, "13:30", arrival)
		assert.Equal(t, "Ausflug", reason)
	}
}
