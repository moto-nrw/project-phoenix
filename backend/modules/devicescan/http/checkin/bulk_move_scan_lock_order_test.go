package checkin_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #5278: a bulk move and a kiosk room change of the same child. The move
// locks the child first and its visit last; before the fix the scan closed
// the visit first and locked the child only for its check-in, so each held
// what the other waited for (SQLSTATE 40P01). Now the scan locks the child
// before it reads and closes the visit. Both tests drive the production scan
// route and the production bulk move; a third transaction holds one row one
// of them needs, so the order of the two writers is fixed by locks, not by
// sleeps.

type bulkMoveScanFixture struct {
	k          *kiosk
	active     studentpresence.Presence
	scanDevice testutil.RequestOption
	scanStaff  testutil.RequestOption
	tag        string
	student    int64
	attendance int64
	visit      int64
	moveTarget int64
	scanRoom   int64
	scanStay   int64
}

func newBulkMoveScanFixture(t *testing.T) bulkMoveScanFixture {
	t.Helper()
	k := reattachKiosk(t)
	f := bulkMoveScanFixture{k: k}
	f.active = api.NewActiveTestPresence(testpkg.NewDatabase(t, k.db), testpkg.TenantRuntime(t, k.db)).Active
	var scanDeviceID int64
	f.scanDevice, scanDeviceID = k.deviceRow(t, "bulk-move-scan-kiosk")
	f.scanStaff, _, _ = k.staff(t, "Scan", "Kiosk")

	// The child is present today and in an open session of room A; the move
	// targets a session in room B, the scan the kiosk's session in room C.
	_, _, source := k.roomWithSession(t, "Bulk Move Quelle")
	_, _, f.moveTarget = k.roomWithSession(t, "Bulk Move Ziel")
	f.scanRoom, _, f.scanStay = k.roomWithSession(t, "Bulk Move Kiosk")
	testpkg.LinkDeviceToActiveGroup(t, k.db, f.scanStay, scanDeviceID)
	f.tag, f.student, _ = k.studentCard(t, "Verschoben", "Kind", "3a")
	f.attendance = testpkg.CreateTestAttendance(t, k.db, f.student, 0, scanDeviceID, time.Now().Add(-2*time.Hour), nil).ID
	f.visit = testpkg.CreateTestVisit(t, k.db, f.student, source, time.Now().Add(-time.Hour), nil).ID
	return f
}

// scan posts the kiosk scan into room C in its own goroutine.
func (f bulkMoveScanFixture) scan(t *testing.T) <-chan *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewTenantRouter(f.k.db)
	router.Mount("/", f.k.resource.Router())
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody(f.tag, f.scanRoom), f.scanDevice, f.scanStaff)
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- testutil.ExecuteRequest(router, req) }()
	return result
}

type moveOutcome struct {
	result *studentpresence.StudentMoveResult
	err    error
}

// move runs the bulk move into room B's session in its own goroutine.
func (f bulkMoveScanFixture) move(t *testing.T) <-chan moveOutcome {
	t.Helper()
	ctx := testpkg.Ctx(t)
	outcome := make(chan moveOutcome, 1)
	go func() {
		result, err := f.active.MoveStudentsToActiveGroupAuthorized(ctx, []int64{f.student}, f.moveTarget,
			studentpresence.StudentMoveAuthorization{BypassResourceChecks: true})
		outcome <- moveOutcome{result: result, err: err}
	}()
	return outcome
}

func awaitMove(t *testing.T, outcome <-chan moveOutcome) moveOutcome {
	t.Helper()
	select {
	case done := <-outcome:
		return done
	case <-time.After(15 * time.Second):
		t.Fatal("bulk move did not finish")
		return moveOutcome{}
	}
}

// assertOnlyOpenVisitIn checks that the child has exactly one open visit,
// in the given session, and still exactly today's one open attendance row.
func (f bulkMoveScanFixture) assertOnlyOpenVisitIn(t *testing.T, sessionID int64) {
	t.Helper()
	open, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{f.student}, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, open, 1, "the child has exactly one open visit")
	assert.Equal(t, sessionID, open[0].ActiveGroupID)
	var openAttendance int
	require.NoError(t, f.k.db.NewRaw("SELECT count(*) FROM active.attendance WHERE tenant_id = ? AND student_id = ? AND check_out_time IS NULL",
		testpkg.Tenant(t), f.student).Scan(testpkg.Ctx(t), &openAttendance))
	assert.Equal(t, 1, openAttendance, "the child keeps today's open attendance")
	closed, err := f.k.presence(t).FindVisit(testpkg.Ctx(t), f.visit)
	require.NoError(t, err)
	assert.NotNil(t, closed.ExitTime, "the visit in room A ended")
}

func assertMoved(t *testing.T, done moveOutcome, studentID int64) {
	t.Helper()
	require.NoError(t, done.err, "the bulk move must not lose against the scan")
	require.NotNil(t, done.result)
	assert.Equal(t, []int64{studentID}, done.result.Moved)
}

// TestBulkMoveAndScanDoNotDeadlock replays the cycle of #5278: the move
// holds the child and waits at its attendance row, which the holder keeps;
// the scan of the same child into room C starts and waits for the move.
// Before the fix the scan had closed the child's visit by then, the move's
// locked re-read of the visits waited for it and PostgreSQL aborted one of
// them with 40P01. Now the scan waits for the child before it touches the
// visit: the move commits first, and the scan changes the child from room B
// to room C.
func TestBulkMoveAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newBulkMoveScanFixture(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM active.attendance WHERE id = ? FOR UPDATE", f.attendance)

	move := f.move(t)
	movePID := awaitWaiterOf(t, f.k.db, holder.pid)
	scan := f.scan(t)
	awaitWaiterOf(t, f.k.db, movePID)
	holder.commit(t)

	done := awaitMove(t, move)
	rr := awaitScan(t, scan)
	assertMoved(t, done, f.student)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "transferred", responseData(t, rr.Body.Bytes())["action"])
	f.assertOnlyOpenVisitIn(t, f.scanStay)
}

// TestScanAndBulkMoveDoNotDeadlock is the opposite order: the scan holds the
// child, has closed its visit and waits at room C's row, which its capacity
// check locks and the holder keeps; the move must wait for the scan and then
// move the child from room C to room B.
func TestScanAndBulkMoveDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newBulkMoveScanFixture(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM facilities.rooms WHERE id = ? FOR UPDATE", f.scanRoom)

	scan := f.scan(t)
	scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
	move := f.move(t)
	awaitWaiterOf(t, f.k.db, scanPID)
	holder.commit(t)

	rr := awaitScan(t, scan)
	done := awaitMove(t, move)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "transferred", responseData(t, rr.Body.Bytes())["action"])
	assertMoved(t, done, f.student)
	f.assertOnlyOpenVisitIn(t, f.moveTarget)
}
