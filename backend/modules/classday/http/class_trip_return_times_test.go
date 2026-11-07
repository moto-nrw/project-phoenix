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

// guardianArrival stores an arrival the parents entered, which a class trip
// may reclaim.
func (f *classTripFixture) guardianArrival(t *testing.T, studentID int64) {
	t.Helper()
	_, err := f.db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO schedule.student_arrival_exceptions (tenant_id, student_id, exception_date, expected_arrival, reason, created_by, source)
		 VALUES (?, ?, ?, '09:00', 'Eltern', ?, 'guardian')`,
		testpkg.Tenant(t), studentID, classTripMonday, f.staffID)
	require.NoError(t, err)
}

// The statements of a return-time trip grow by a fixed number per child
// (#5824): the student and day locks, the locked membership check and the
// write. The children's current arrivals are one read after the last lock.
func TestClassTripReturnTimeStatementsPerChild(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)
	counter := testpkg.CaptureQueriesForContext(t, f.db)

	small := f.classChildren(t, 3)
	code, resp, n3 := f.returnTimeTrip(t, counter, small)
	require.Equal(t, http.StatusCreated, code, resp)

	large := f.classChildren(t, 8)
	code, resp, n8 := f.returnTimeTrip(t, counter, large)
	require.Equal(t, http.StatusCreated, code, resp)

	mixed := f.classChildren(t, 3)
	f.guardianArrival(t, mixed[1])
	code, resp, reclaim := f.returnTimeTrip(t, counter, mixed)
	require.Equal(t, http.StatusCreated, code, resp)
	arrival, reason, ok := f.arrival(t, mixed[1], classTripMonday)
	require.True(t, ok)
	assert.Equal(t, "13:30", arrival)
	assert.Equal(t, "Ausflug", reason)

	t.Logf("class trip return time: N3=%d N8=%d N3-reclaim=%d per-child=%d", n3, n8, reclaim, (n8-n3)/5)
	assert.Equal(t, 4*5, n8-n3, "each further child costs its locks, the locked membership check and the write")
	assert.Equal(t, n3, reclaim, "a reclaimed arrival costs what a new one costs")
}
