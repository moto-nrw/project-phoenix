package checkin_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #5206: a kiosk start that reattaches to a device-less room stay takes over
// the open visits of the device's recently ended session, while another
// kiosk scans one of those children into that very room stay. Before the
// fix the scan held the child's visit and waited for the stay's row, and the
// reattach held the stay's row and waited for the visit (SQLSTATE 40P01).
// Now both lock the visit before the stay. Both tests drive the production scan route and the production session
// start; a third transaction holds one row the scan needs, so the order of
// the two writers is fixed by locks, not by sleeps.

type reattachScanFixture struct {
	k          *kiosk
	active     studentpresence.Presence
	scanDevice testutil.RequestOption
	scanStaff  testutil.RequestOption
	device     int64
	supervisor int64
	activity   int64
	room       int64
	stay       int64
	ended      int64
	tag        string
	scanned    int64
	scannedV   int64
	takenOver  int64
}

func newReattachScanFixture(t *testing.T) reattachScanFixture {
	t.Helper()
	k := setupCheckinRoute(t)
	f := reattachScanFixture{k: k}
	f.active = api.NewActiveTestPresence(testpkg.NewDatabase(t, k.db), testpkg.TenantRuntime(t, k.db)).Active
	f.scanDevice = k.device(t, "reattach-scan-kiosk")
	f.scanStaff, _, _ = k.staff(t, "Scan", "Kiosk")
	_, f.supervisor, _ = k.staff(t, "Start", "Kiosk")

	// The room stay the kiosk joins: a system activity's device-less session.
	room := testpkg.CreateTestRoom(t, k.db, "Reattach Hof")
	activity := testpkg.CreateTestActivityGroup(t, k.db, "Reattach Freispiel")
	_, err := k.db.NewUpdate().TableExpr("activities.groups").Set("is_system = TRUE").
		Where("id = ?", activity.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	f.room, f.activity = room.ID, activity.ID
	f.stay = testpkg.CreateTestActiveGroup(t, k.db, activity.ID, room.ID).ID

	// The kiosk's own session in another room ended five minutes ago and
	// still holds two children.
	device := testpkg.CreateTestDevice(t, k.db, "reattach-start-kiosk")
	f.device = device.ID
	_, _, ended := k.roomWithSession(t, "Reattach Mensa")
	f.ended = ended
	testpkg.LinkDeviceToActiveGroup(t, k.db, ended, device.ID)
	var scannedStudent int64
	f.tag, scannedStudent, _ = k.studentCard(t, "Gescannt", "Kind", "2a")
	f.scanned = scannedStudent
	f.scannedV = testpkg.CreateTestVisit(t, k.db, scannedStudent, ended, time.Now().Add(-time.Hour), nil).ID
	takenOver := testpkg.CreateTestStudent(t, k.db, "Uebernommen", "Kind", "2a")
	f.takenOver = takenOver.ID
	testpkg.CreateTestVisit(t, k.db, takenOver.ID, ended, time.Now().Add(-time.Hour), nil)
	_, err = k.db.NewUpdate().TableExpr("active.groups").Set("end_time = NOW() - INTERVAL '5 minutes'").
		Where("id = ?", ended).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return f
}

// scan posts the kiosk scan in its own goroutine.
func (f reattachScanFixture) scan(t *testing.T) <-chan *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewTenantRouter(f.k.db)
	router.Mount("/", f.k.resource.Router())
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody(f.tag, f.room), f.scanDevice, f.scanStaff)
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- testutil.ExecuteRequest(router, req) }()
	return result
}

// reattach starts the kiosk session in its own goroutine.
func (f reattachScanFixture) reattach(t *testing.T) <-chan error {
	t.Helper()
	ctx := testpkg.Ctx(t)
	result := make(chan error, 1)
	go func() {
		_, err := f.active.StartActivitySessionWithSupervisors(ctx, f.activity, f.device, []int64{f.supervisor}, &f.room)
		result <- err
	}()
	return result
}

// rowHolder is a tenant transaction that holds one row lock until commit.
type rowHolder struct {
	pid     int
	release chan struct{}
	done    chan error
}

// holdRow runs the locking query, which returns one id, in its own tenant
// transaction and keeps that transaction open until commit.
func holdRow(t *testing.T, db *bun.DB, query string, args ...any) *rowHolder {
	t.Helper()
	holder := &rowHolder{release: make(chan struct{}), done: make(chan error, 1)}
	ready := make(chan int, 1)
	go func() {
		holder.done <- testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var pid int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
				return err
			}
			var locked int64
			if err := tx.NewRaw(query, args...).Scan(txCtx, &locked); err != nil {
				return err
			}
			ready <- pid
			<-holder.release
			return nil
		})
	}()
	select {
	case holder.pid = <-ready:
	case err := <-holder.done:
		t.Fatalf("row holder did not start: %v", err)
	}
	return holder
}

func (h *rowHolder) commit(t *testing.T) {
	t.Helper()
	close(h.release)
	select {
	case err := <-h.done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("row holder did not commit")
	}
}

// awaitWaiterOf returns the backend that waits on a lock the holder pid holds.
func awaitWaiterOf(t *testing.T, db *bun.DB, holderPID int) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiters []int
		require.NoError(t, db.NewRaw("SELECT pid FROM pg_stat_activity WHERE ?::int = ANY (pg_blocking_pids(pid))", holderPID).
			Scan(context.Background(), &waiters))
		if len(waiters) > 0 {
			return waiters[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no backend waited on the holder")
	return 0
}

// awaitDoneOrWaiting waits until the step finished or another backend waits
// on the holder pid. It returns the step's error when it finished.
func awaitDoneOrWaiting(t *testing.T, db *bun.DB, step <-chan error, holderPID int) (finished bool, err error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-step:
			return true, err
		default:
		}
		var waiting bool
		require.NoError(t, db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ?::int = ANY (pg_blocking_pids(pid)))", holderPID).
			Scan(context.Background(), &waiting))
		if waiting {
			return false, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the step neither finished nor waited on the holder")
	return false, nil
}

func awaitScan(t *testing.T, result <-chan *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()
	select {
	case rr := <-result:
		return rr
	case <-time.After(15 * time.Second):
		t.Fatal("scan did not finish")
		return nil
	}
}

func awaitReattach(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("reattach did not finish")
		return nil
	}
}

func (f reattachScanFixture) openVisitSessions(t *testing.T) map[int64]int64 {
	t.Helper()
	visits, err := f.k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{
		StudentIDs: []int64{f.scanned, f.takenOver}, OpenOnly: true,
	})
	require.NoError(t, err)
	sessions := make(map[int64]int64, len(visits))
	for _, visit := range visits {
		_, duplicate := sessions[visit.StudentID]
		require.False(t, duplicate, "child %d has more than one open visit", visit.StudentID)
		sessions[visit.StudentID] = visit.ActiveGroupID
	}
	return sessions
}

// assertScanThenTakeover checks the outcome both orders share: the scan closed
// the child's visit in the ended session and checked it into the stay, the
// reattach attached the kiosk and took over the other child, and nothing was
// booked twice.
func (f reattachScanFixture) assertScanThenTakeover(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "transferred", responseData(t, rr.Body.Bytes())["action"])
	var deviceID *int64
	require.NoError(t, f.k.db.NewRaw("SELECT device_id FROM active.groups WHERE id = ?", f.stay).Scan(testpkg.Ctx(t), &deviceID))
	require.NotNil(t, deviceID, "the kiosk attached to the room stay")
	assert.Equal(t, f.device, *deviceID)
	assert.Equal(t, map[int64]int64{f.scanned: f.stay, f.takenOver: f.stay}, f.openVisitSessions(t),
		"the scan checked the child into the stay and the reattach took over the other child")
	closed, err := f.k.presence(t).FindVisit(testpkg.Ctx(t), f.scannedV)
	require.NoError(t, err)
	assert.Equal(t, f.ended, closed.ActiveGroupID, "the scan closed the visit in the ended session")
	assert.NotNil(t, closed.ExitTime)
}

// runReattachAgainstScan starts the scan, waits until the holder stops it,
// starts the reattach and requires it to wait for the scan, which holds the
// child's visit, then releases the holder and returns both results.
func (f reattachScanFixture) runReattachAgainstScan(t *testing.T, holder *rowHolder, release func(*testing.T)) (*httptest.ResponseRecorder, error) {
	t.Helper()
	scan := f.scan(t)
	scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
	reattach := f.reattach(t)
	finished, reattachErr := awaitDoneOrWaiting(t, f.k.db, reattach, scanPID)
	release(t)
	if !finished {
		reattachErr = awaitReattach(t, reattach)
	}
	rr := awaitScan(t, scan)
	require.False(t, finished, "the reattach must wait for the scan that holds the child's visit (reattach error: %v)", reattachErr)
	return rr, reattachErr
}

// TestReattachWaitsForTheScanHoldingTheVisit replays the cycle of #5206: the
// scan closed the child's visit and waits before its check-in; the reattach
// starts and takes over the ended session's visits. Before the fix the
// reattach had locked the stay first, the scan's check-in waited for it and
// PostgreSQL aborted one of them with 40P01.
func TestReattachWaitsForTheScanHoldingTheVisit(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM users.student_profiles WHERE id = ? FOR UPDATE", f.scanned)

	rr, reattachErr := f.runReattachAgainstScan(t, holder, holder.commit)

	require.NoError(t, reattachErr, "the reattach must not lose against the scan")
	f.assertScanThenTakeover(t, rr)
}

// TestReattachWaitsForTheScanHoldingTheStay holds the scan after it closed the
// child's visit and locked the stay, at the room row its capacity check
// locks; the reattach must wait for the scan and then take over the other
// child. This order already worked before the fix and must keep working.
func TestReattachWaitsForTheScanHoldingTheStay(t *testing.T) {
	t.Parallel()
	f := newReattachScanFixture(t)
	holder := holdRow(t, f.k.db, "SELECT id FROM facilities.rooms WHERE id = ? FOR UPDATE", f.room)

	rr, reattachErr := f.runReattachAgainstScan(t, holder, holder.commit)

	require.NoError(t, reattachErr, "the reattach must not lose against the scan")
	f.assertScanThenTakeover(t, rr)
}
