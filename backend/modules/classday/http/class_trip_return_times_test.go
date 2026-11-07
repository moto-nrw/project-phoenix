package classdayhttp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
// (#5824): the student and day locks, the locked membership check, the read
// of the child's day and the write. The single command's second read of the
// day under the same locks is gone.
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
	assert.Equal(t, 5*5, n8-n3, "each further child costs its locks, the locked membership check, its day read and the write")
	assert.Equal(t, n3, reclaim, "a reclaimed arrival costs what a new one costs")
}

// staffArrival stores an arrival the OGS entered, which a class trip may not
// overwrite.
func (f *classTripFixture) staffArrival(t *testing.T, studentID int64) {
	t.Helper()
	_, err := f.db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO schedule.student_arrival_exceptions (tenant_id, student_id, exception_date, expected_arrival, reason, created_by, source)
		 VALUES (?, ?, ?, '09:00', 'OGS', ?, 'staff')`,
		testpkg.Tenant(t), studentID, classTripMonday, f.staffID)
	require.NoError(t, err)
}

// holdStudentRow locks a child's student row in a transaction of its own,
// the way a concurrent care-day writer holds it, and returns that
// connection's backend PID.
func (f *classTripFixture) holdStudentRow(t *testing.T, studentID int64) int {
	t.Helper()
	ctx := testpkg.Ctx(t)
	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tx.Rollback()) })
	_, err = tx.ExecContext(ctx, `SELECT id FROM users.student_profiles WHERE id = ? FOR UPDATE`, studentID)
	require.NoError(t, err)
	var pid int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	return pid
}

// postAsync posts the trip in the background and returns its result.
func (f *classTripFixture) postAsync(t *testing.T, studentIDs []int64) <-chan [2]string {
	t.Helper()
	counter := testpkg.NewQueryCounter()
	done := make(chan [2]string, 1)
	go func() {
		code, resp, _ := f.returnTimeTrip(t, counter, studentIDs)
		done <- [2]string{fmt.Sprint(code), resp}
	}()
	return done
}

// A day conflict of an earlier child refuses the report before any later
// child is locked: a later child whose row another writer holds neither
// delays the refusal nor turns it into a server error (#5824, as one command
// per child).
func TestClassTripReturnTimeRefusesBeforeLockingLaterChildren(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)
	children := f.classChildren(t, 2)
	f.staffArrival(t, children[0])
	f.holdStudentRow(t, children[1])

	select {
	case result := <-f.postAsync(t, children):
		require.Equal(t, fmt.Sprint(http.StatusConflict), result[0], result[1])
		assert.Contains(t, result[1], "classday.class_trip_arrival_conflict")
	case <-time.After(10 * time.Second):
		t.Fatal("the refusal waited for the later child's lock")
	}
}

// A failure while waiting for a later child's lock ends the report with a
// server error and keeps no child (#5824): the earlier child's write rolls
// back with the request.
func TestClassTripReturnTimeLockFailureKeepsNoChild(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)
	children := f.classChildren(t, 2)
	blocker := f.holdStudentRow(t, children[1])
	done := f.postAsync(t, children)

	// Cancel the request's statement once it waits for the held row.
	ctx := testpkg.Ctx(t)
	var waiting int
	require.Eventually(t, func() bool {
		err := f.db.QueryRowContext(ctx, `SELECT pid FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)) LIMIT 1`, blocker).Scan(&waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond, "the report never waited for the held row")
	_, err := f.db.ExecContext(ctx, `SELECT pg_cancel_backend(?)`, waiting)
	require.NoError(t, err)

	select {
	case result := <-done:
		require.Equal(t, fmt.Sprint(http.StatusInternalServerError), result[0], result[1])
	case <-time.After(10 * time.Second):
		t.Fatal("the cancelled report did not end")
	}
	for _, id := range children {
		_, _, ok := f.arrival(t, id, classTripMonday)
		assert.False(t, ok, "child %d", id)
	}
}
