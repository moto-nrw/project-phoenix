package presence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #5206: a kiosk start that joins a device-less room stay moves the open
// visits of the device's recently ended session before it locks the stay.
// A web move of one of those visits into the stay locks the stay and then
// the visit, so it takes the stay's room lock first, as a push move does;
// without it the two would wait on each other. A third transaction holds the
// stay's row, so both writers queue on locks, not on sleeps.
func TestReattachAndWebVisitMoveIntoTheStayDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	room := testpkg.CreateTestRoom(t, db, "Reattach Web Hof")
	activity := testpkg.CreateTestActivityGroup(t, db, "Reattach Web Freispiel")
	markActivitySystem(t, db, activity.ID)
	stay := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Reattach", "Web")
	device := testpkg.CreateTestDevice(t, db, "reattach-web-kiosk")
	ended := testpkg.CreateTestActiveGroup(t, db, testpkg.CreateTestActivityGroup(t, db, "Reattach Web Mensa").ID,
		testpkg.CreateTestRoom(t, db, "Reattach Web Mensa").ID)
	testpkg.LinkDeviceToActiveGroup(t, db, ended.ID, device.ID)
	moved := testpkg.CreateTestStudent(t, db, "Verschoben", "Web", "2a")
	takenOver := testpkg.CreateTestStudent(t, db, "Uebernommen", "Web", "2a")
	movedVisit := testpkg.CreateTestVisit(t, db, moved.ID, ended.ID, time.Now().Add(-time.Hour), nil)
	testpkg.CreateTestVisit(t, db, takenOver.ID, ended.ID, time.Now().Add(-time.Hour), nil)
	_, err := db.NewUpdate().TableExpr("active.groups").Set("end_time = NOW() - INTERVAL '5 minutes'").
		Where("id = ?", ended.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)

	holder := holdStayRow(t, db, stay.ID)
	webMove := make(chan error, 1)
	go func() {
		webMove <- svc.UpdateVisit(ctx, &studentpresence.Visit{
			ID: movedVisit.ID, TenantID: movedVisit.TenantID, StudentID: moved.ID,
			ActiveGroupID: stay.ID, EntryTime: movedVisit.EntryTime,
		})
	}()
	webPID := awaitBlockedOn(t, db, holder.pid, 0)
	reattach := make(chan error, 1)
	go func() {
		_, err := svc.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff.ID}, &room.ID)
		reattach <- err
	}()
	awaitBlockedOn(t, db, holder.pid, webPID)
	holder.commit(t)

	require.NoError(t, awaitResult(t, webMove), "the web move must not lose against the reattach")
	require.NoError(t, awaitResult(t, reattach), "the reattach must not lose against the web move")
	visits, err := testSchoolPresence(t, db).ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{moved.ID, takenOver.ID}, OpenOnly: true})
	require.NoError(t, err)
	sessions := map[int64]int64{}
	for _, visit := range visits {
		sessions[visit.StudentID] = visit.ActiveGroupID
	}
	assert.Equal(t, map[int64]int64{moved.ID: stay.ID, takenOver.ID: stay.ID}, sessions, "each child has one open visit, in the stay")
}

type stayRowHolder struct {
	pid         int
	release     chan struct{}
	releaseOnce sync.Once
	done        chan struct{}
	err         error
}

func holdStayRow(t *testing.T, db *bun.DB, stayID int64) *stayRowHolder {
	t.Helper()
	holder := &stayRowHolder{release: make(chan struct{}), done: make(chan struct{})}
	t.Cleanup(func() { holder.commit(t) })
	ready := make(chan int, 1)
	go func() {
		defer close(holder.done)
		holder.err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var pid int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
				return err
			}
			var locked int64
			if err := tx.NewRaw("SELECT id FROM active.groups WHERE id = ? FOR UPDATE", stayID).Scan(txCtx, &locked); err != nil {
				return err
			}
			ready <- pid
			<-holder.release
			return nil
		})
	}()
	select {
	case holder.pid = <-ready:
	case <-holder.done:
		t.Fatalf("stay holder did not start: %v", holder.err)
	}
	return holder
}

func (h *stayRowHolder) commit(t *testing.T) {
	t.Helper()
	h.releaseOnce.Do(func() { close(h.release) })
	select {
	case <-h.done:
		require.NoError(t, h.err)
	case <-time.After(10 * time.Second):
		t.Fatal("stay holder did not commit")
	}
}

// awaitBlockedOn returns a backend other than except that waits on a lock the
// holder pid or except holds.
func awaitBlockedOn(t *testing.T, db *bun.DB, holderPID, except int) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiters []int
		require.NoError(t, db.NewRaw(`SELECT pid FROM pg_stat_activity
 WHERE pid <> ?::int AND (?::int = ANY (pg_blocking_pids(pid)) OR ?::int = ANY (pg_blocking_pids(pid)))`,
			except, holderPID, except).Scan(context.Background(), &waiters))
		if len(waiters) > 0 {
			return waiters[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no backend waited on the holder")
	return 0
}

func awaitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("writer did not finish")
		return nil
	}
}

func TestStayRowHolderCleanupReleasesTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	room := testpkg.CreateTestRoom(t, db, "Stay holder cleanup")
	activity := testpkg.CreateTestActivityGroup(t, db, "Stay holder cleanup")
	stay := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	var holder *stayRowHolder
	t.Run("without explicit commit", func(t *testing.T) {
		holder = holdStayRow(t, db, stay.ID)
	})
	defer holder.commit(t)
	select {
	case <-holder.done:
	default:
		t.Fatal("cleanup did not finish the holder transaction")
	}
	holder.commit(t)
}
