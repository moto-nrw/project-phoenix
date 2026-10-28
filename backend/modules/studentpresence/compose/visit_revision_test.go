package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The visit revision of a session orders the kiosk counter (#5128). These
// tests pin its invariants: every committed change of the open visit count
// raises it, nothing else does, one revision names one count, writers of one
// session commit in revision order, and a read that started before a write
// committed never carries that write's revision.

type countSnapshot struct {
	count    int
	revision int64
}

func readSnapshot(t *testing.T, ctx context.Context, module studentpresence.Owner, groupID int64) countSnapshot {
	t.Helper()
	count, revision, err := module.CountOpenVisitsInGroupWithRevision(ctx, groupID)
	require.NoError(t, err)
	return countSnapshot{count: count, revision: revision}
}

func newRevisionModule(t *testing.T, db *bun.DB) studentpresence.Owner {
	t.Helper()
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	return module
}

func openVisit(t *testing.T, ctx context.Context, module studentpresence.Owner, db *bun.DB, groupID int64, name string) studentpresence.Visit {
	t.Helper()
	student := testpkg.CreateTestStudent(t, db, name, "Revision", "3a")
	visit, err := module.RecordVisit(ctx, studentpresence.Visit{StudentID: student.ID, ActiveGroupID: groupID, EntryTime: time.Now().Add(-time.Hour)})
	require.NoError(t, err)
	return visit
}

func TestVisitRevisionFollowsEveryOpenCountChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Revision activity")
	room := testpkg.CreateTestRoom(t, db, "Revision room")
	other := testpkg.CreateTestRoom(t, db, "Revision other room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	target := testpkg.CreateTestActiveGroup(t, db, activity.ID, other.ID)

	start := readSnapshot(t, ctx, module, group.ID)
	assert.Equal(t, countSnapshot{count: 0, revision: 0}, start, "a new session starts at revision 0")

	first := openVisit(t, ctx, module, db, group.ID, "First")
	afterOpen := readSnapshot(t, ctx, module, group.ID)
	assert.Equal(t, countSnapshot{count: 1, revision: 1}, afterOpen, "opening a visit raises the revision")

	closedStudent := testpkg.CreateTestStudent(t, db, "Closed", "Revision", "3a")
	exit := time.Now().Add(-time.Minute)
	_, err := module.RecordVisit(ctx, studentpresence.Visit{StudentID: closedStudent.ID, ActiveGroupID: group.ID, EntryTime: time.Now().Add(-2 * time.Hour), ExitTime: &exit})
	require.NoError(t, err)
	assert.Equal(t, afterOpen, readSnapshot(t, ctx, module, group.ID), "a closed visit leaves the count and its revision")

	_, err = db.NewUpdate().Table("active.visits").Set("entry_time = entry_time - INTERVAL '1 minute'").
		Where("id = ?", first.ID).Exec(ctx)
	require.NoError(t, err)
	assert.Equal(t, afterOpen, readSnapshot(t, ctx, module, group.ID), "an update that keeps the count keeps the revision")

	var updatedBefore, updatedAfter time.Time
	require.NoError(t, db.NewRaw("SELECT updated_at FROM active.groups WHERE id = ?", group.ID).Scan(ctx, &updatedBefore))
	moved, err := module.TransferOpenVisits(ctx, group.ID, target.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, moved)
	assert.Equal(t, countSnapshot{count: 0, revision: 2}, readSnapshot(t, ctx, module, group.ID), "a move raises the source session")
	assert.Equal(t, countSnapshot{count: 1, revision: 1}, readSnapshot(t, ctx, module, target.ID), "a move raises the target session")
	require.NoError(t, db.NewRaw("SELECT updated_at FROM active.groups WHERE id = ?", group.ID).Scan(ctx, &updatedAfter))
	assert.True(t, updatedBefore.Equal(updatedAfter), "the revision change leaves the session's updated_at")

	second := openVisit(t, ctx, module, db, target.ID, "Second")
	third := openVisit(t, ctx, module, db, target.ID, "Third")
	assert.Equal(t, countSnapshot{count: 3, revision: 3}, readSnapshot(t, ctx, module, target.ID))
	closed, err := module.CloseGroupVisits(ctx, []int64{target.ID})
	require.NoError(t, err)
	require.EqualValues(t, 3, closed)
	assert.Equal(t, countSnapshot{count: 0, revision: 4}, readSnapshot(t, ctx, module, target.ID),
		"one statement closing several visits raises the revision once")

	require.NoError(t, module.DeleteVisit(ctx, second.ID))
	assert.Equal(t, countSnapshot{count: 0, revision: 4}, readSnapshot(t, ctx, module, target.ID), "deleting a closed visit keeps the revision")
	fourth := openVisit(t, ctx, module, db, target.ID, "Fourth")
	require.NoError(t, module.DeleteVisit(ctx, fourth.ID))
	assert.Equal(t, countSnapshot{count: 0, revision: 6}, readSnapshot(t, ctx, module, target.ID), "opening and deleting an open visit raise it twice")
	assert.NotZero(t, third.ID)
}

func TestVisitRevisionStaysInsideTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	var foreignGroup int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx := testpkg.Ctx(t)
		activity := testpkg.CreateTestActivityGroup(t, db, "Foreign revision")
		room := testpkg.CreateTestRoom(t, db, "Foreign revision")
		foreignGroup = testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID).ID
		openVisit(t, foreignCtx, module, db, foreignGroup, "Foreign")
	})
	_, _, err := module.CountOpenVisitsInGroupWithRevision(ctx, foreignGroup)
	require.Error(t, err, "another school's session has no count and no revision here")
	_, _, err = module.CountOpenVisitsInGroupWithRevision(ctx, 0)
	require.Error(t, err)
}

// txActor runs one tenant transaction in its own goroutine, so a test can
// hold it open, let it block on a lock, and decide when it commits.
type txActor struct {
	pid   int
	steps chan func(context.Context) error
	done  chan error
}

func startTx(t *testing.T, ctx context.Context, db *bun.DB) *txActor {
	t.Helper()
	actor := &txActor{steps: make(chan func(context.Context) error), done: make(chan error, 1)}
	ready := make(chan int, 1)
	go func() {
		actor.done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var pid int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
				return err
			}
			ready <- pid
			for step := range actor.steps {
				if step == nil {
					return nil
				}
				if err := step(txCtx); err != nil {
					return err
				}
			}
			return nil
		})
	}()
	select {
	case actor.pid = <-ready:
	case err := <-actor.done:
		t.Fatalf("transaction did not start: %v", err)
	}
	return actor
}

// run hands a step to the transaction and returns its result channel
// without waiting, so the step may block on a lock.
func (a *txActor) run(step func(context.Context) error) <-chan error {
	result := make(chan error, 1)
	a.steps <- func(ctx context.Context) error {
		err := step(ctx)
		result <- err
		return err
	}
	return result
}

func (a *txActor) commit(t *testing.T) {
	t.Helper()
	a.steps <- nil
	select {
	case err := <-a.done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("transaction did not commit")
	}
}

func awaitStep(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("step did not finish")
	}
}

// awaitBlockedBy waits until the waiter's backend waits on a lock the holder
// holds, so the test orders both transactions without timing guesses.
func awaitBlockedBy(t *testing.T, db *bun.DB, waiter, holder *txActor) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		require.NoError(t, db.NewRaw("SELECT ?::int = ANY (pg_blocking_pids(?::int))", holder.pid, waiter.pid).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("waiter never blocked on the holder")
}

// TestVisitRevisionOrdersAPollBeforeTheScanCommitted drives the race of
// #5128: the scan books the eleventh child and answers 11 before it commits;
// a later poll still reads 10. The poll's revision is lower, so the kiosk
// keeps 11 whichever answer arrives last, and the next poll returns exactly
// the scan's count and revision.
func TestVisitRevisionOrdersAPollBeforeTheScanCommitted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Race activity")
	room := testpkg.CreateTestRoom(t, db, "Race room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	for range 10 {
		openVisit(t, ctx, module, db, group.ID, "Present")
	}
	eleventh := testpkg.CreateTestStudent(t, db, "Eleventh", "Revision", "3a")

	scan := startTx(t, ctx, db)
	var scanAnswer countSnapshot
	awaitStep(t, scan.run(func(txCtx context.Context) error {
		if _, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: eleventh.ID, ActiveGroupID: group.ID, EntryTime: time.Now()}); err != nil {
			return err
		}
		scanAnswer = readSnapshot(t, txCtx, module, group.ID)
		return nil
	}))
	poll := readSnapshot(t, ctx, module, group.ID)
	scan.commit(t)

	assert.Equal(t, 11, scanAnswer.count)
	assert.Equal(t, 10, poll.count, "the poll read before the scan committed")
	assert.Less(t, poll.revision, scanAnswer.revision, "the older count carries the lower revision")
	assert.Equal(t, scanAnswer, readSnapshot(t, ctx, module, group.ID), "the same revision names the same count")
}

// TestVisitRevisionSerializesWritersOfOneSession runs two scans into one
// session at once, as two kiosks would. The second waits for the first to
// commit, so commit order, revision order and count agree.
func TestVisitRevisionSerializesWritersOfOneSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Writers activity")
	room := testpkg.CreateTestRoom(t, db, "Writers room")
	otherRoom := testpkg.CreateTestRoom(t, db, "Writers other room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	otherGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, otherRoom.ID)
	first := testpkg.CreateTestStudent(t, db, "First", "Writer", "3a")
	second := testpkg.CreateTestStudent(t, db, "Second", "Writer", "3a")
	elsewhere := testpkg.CreateTestStudent(t, db, "Elsewhere", "Writer", "3a")
	book := func(studentID, groupID int64, answer *countSnapshot) func(context.Context) error {
		return func(txCtx context.Context) error {
			if _, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: studentID, ActiveGroupID: groupID, EntryTime: time.Now()}); err != nil {
				return err
			}
			*answer = readSnapshot(t, txCtx, module, groupID)
			return nil
		}
	}

	w1 := startTx(t, ctx, db)
	var a1, a2, other countSnapshot
	awaitStep(t, w1.run(book(first.ID, group.ID, &a1)))

	independent := startTx(t, ctx, db)
	awaitStep(t, independent.run(book(elsewhere.ID, otherGroup.ID, &other)))
	independent.commit(t)
	assert.Equal(t, countSnapshot{count: 1, revision: 1}, other, "another session never waits for this one")

	w2 := startTx(t, ctx, db)
	second2 := w2.run(book(second.ID, group.ID, &a2))
	awaitBlockedBy(t, db, w2, w1)
	w1.commit(t)
	awaitStep(t, second2)
	w2.commit(t)

	assert.Equal(t, countSnapshot{count: 1, revision: 1}, a1)
	assert.Equal(t, countSnapshot{count: 2, revision: 2}, a2, "the later commit carries the next revision and both children")
	assert.Equal(t, a2, readSnapshot(t, ctx, module, group.ID))
}

// TestCloseVisitsWaitsForASessionEndWithoutDeadlock pins the lock order the
// revision needs: a checkout locks the session before its visit, as a
// session end does. A session end holding the session lets the checkout
// wait instead of both transactions deadlocking.
func TestCloseVisitsWaitsForASessionEndWithoutDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Deadlock activity")
	room := testpkg.CreateTestRoom(t, db, "Deadlock room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	visit := openVisit(t, ctx, module, db, group.ID, "Leaving")
	before := readSnapshot(t, ctx, module, group.ID)

	sessionEnd := startTx(t, ctx, db)
	awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		return err
	}))
	checkout := startTx(t, ctx, db)
	var closed []studentpresence.Visit
	closing := checkout.run(func(txCtx context.Context) error {
		var err error
		closed, err = module.CloseVisits(txCtx, []int64{visit.ID}, time.Now())
		return err
	})
	awaitBlockedBy(t, db, checkout, sessionEnd)
	awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, time.Now())
		return err
	}))
	sessionEnd.commit(t)
	awaitStep(t, closing)
	checkout.commit(t)

	assert.Empty(t, closed, "the session end closed the visit first")
	assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 1}, readSnapshot(t, ctx, module, group.ID))
}

// Deletion and revision must lock the session before the visit, including
// both sessions when a revision moves an open visit.
func TestVisitWritersWaitForASessionEndWithoutDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, operation := range []string{"delete", "close", "move from ending session", "move to ending session"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			module := newRevisionModule(t, db)
			activity := testpkg.CreateTestActivityGroup(t, db, "Writer activity")
			room := testpkg.CreateTestRoom(t, db, "Writer room")
			otherRoom := testpkg.CreateTestRoom(t, db, "Writer target room")
			group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			target := testpkg.CreateTestActiveGroup(t, db, activity.ID, otherRoom.ID)
			visit := openVisit(t, ctx, module, db, group.ID, "Leaving")
			endingID := group.ID
			if operation == "move to ending session" {
				endingID = target.ID
			}

			sessionEnd := startTx(t, ctx, db)
			awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
				_, err := module.LockGroup(txCtx, endingID)
				return err
			}))
			writer := startTx(t, ctx, db)
			writing := writer.run(func(txCtx context.Context) error {
				if operation == "delete" {
					return module.DeleteVisit(txCtx, visit.ID)
				}
				if operation == "close" {
					at := time.Now()
					visit.ExitTime = &at
				} else {
					visit.ActiveGroupID = target.ID
				}
				_, err := module.ReviseVisit(txCtx, visit)
				return err
			})
			awaitBlockedBy(t, db, writer, sessionEnd)
			awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
				_, err := module.EndGroupSession(txCtx, endingID, time.Now())
				return err
			}))
			sessionEnd.commit(t)
			awaitStep(t, writing)
			writer.commit(t)

			assert.Equal(t, countSnapshot{count: 0, revision: 2}, readSnapshot(t, ctx, module, group.ID))
			if operation == "delete" || operation == "close" {
				assert.Equal(t, countSnapshot{}, readSnapshot(t, ctx, module, target.ID))
			} else {
				assert.Equal(t, countSnapshot{count: 1, revision: 1}, readSnapshot(t, ctx, module, target.ID))
			}
		})
	}
}

// awaitBlockedOrDone waits until the waiter blocks on the holder (true) or
// its step finishes first (false, with the step's error). The test then
// knows which interleaving the lock order produced.
func awaitBlockedOrDone(t *testing.T, db *bun.DB, waiter, holder *txActor, step <-chan error) (bool, error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-step:
			return false, err
		default:
		}
		var blocked bool
		require.NoError(t, db.NewRaw("SELECT ?::int = ANY (pg_blocking_pids(?::int))", holder.pid, waiter.pid).Scan(context.Background(), &blocked))
		if blocked {
			return true, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("waiter neither blocked nor finished")
	return false, nil
}

// TestOppositeScansWaitInsteadOfDeadlocking replays two scans that move
// children in opposite directions between two sessions, each as the device
// scan runs it: lock the scan's sessions, close the current visit, lock the
// target (CreateVisit) and record the new visit. Locking every session up
// front in ascending order makes the second scan wait for the first;
// without it, each holds its source and waits for the other's.
func TestOppositeScansWaitInsteadOfDeadlocking(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Opposite activity")
	firstRoom := testpkg.CreateTestRoom(t, db, "Opposite first room")
	secondRoom := testpkg.CreateTestRoom(t, db, "Opposite second room")
	first := testpkg.CreateTestActiveGroup(t, db, activity.ID, firstRoom.ID)
	second := testpkg.CreateTestActiveGroup(t, db, activity.ID, secondRoom.ID)
	leavingFirst := openVisit(t, ctx, module, db, first.ID, "Forward")
	leavingSecond := openVisit(t, ctx, module, db, second.ID, "Backward")
	checkout := func(visit studentpresence.Visit, roomID int64) func(context.Context) error {
		return func(txCtx context.Context) error {
			if err := module.LockVisitSessions(txCtx, visit.StudentID, roomID); err != nil {
				return err
			}
			_, err := module.CloseVisits(txCtx, []int64{visit.ID}, time.Now())
			return err
		}
	}
	checkin := func(studentID, groupID int64) func(context.Context) error {
		return func(txCtx context.Context) error {
			if _, err := module.LockGroup(txCtx, groupID); err != nil {
				return err
			}
			_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: studentID, ActiveGroupID: groupID, EntryTime: time.Now()})
			return err
		}
	}

	forward := startTx(t, ctx, db)
	awaitStep(t, forward.run(checkout(leavingFirst, secondRoom.ID)))
	backward := startTx(t, ctx, db)
	backwardCheckout := backward.run(checkout(leavingSecond, firstRoom.ID))
	blocked, err := awaitBlockedOrDone(t, db, backward, forward, backwardCheckout)
	if !blocked {
		require.NoError(t, err)
		forwardCheckin := forward.run(checkin(leavingFirst.StudentID, second.ID))
		backwardCheckin := backward.run(checkin(leavingSecond.StudentID, first.ID))
		require.NoError(t, <-backwardCheckin, "the opposite scan must wait instead of deadlocking")
		require.NoError(t, <-forwardCheckin, "the opposite scan must wait instead of deadlocking")
		t.Fatal("the opposite scan did not wait for the sessions the first scan locked")
	}
	awaitStep(t, forward.run(checkin(leavingFirst.StudentID, second.ID)))
	forward.commit(t)
	awaitStep(t, backwardCheckout)
	awaitStep(t, backward.run(checkin(leavingSecond.StudentID, first.ID)))
	backward.commit(t)

	assert.Equal(t, countSnapshot{count: 1, revision: 3}, readSnapshot(t, ctx, module, first.ID))
	assert.Equal(t, countSnapshot{count: 1, revision: 3}, readSnapshot(t, ctx, module, second.ID))
}

// TestCareExitWaitsForASessionEndWithoutDeadlock replays a care exit (lock
// the child's open presence, then close it) next to a session end of the
// child's session (lock the session, then close its visits). The care exit
// locks the session before the visit, as the session end does.
func TestCareExitWaitsForASessionEndWithoutDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Care exit activity")
	room := testpkg.CreateTestRoom(t, db, "Care exit room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	leaving := openVisit(t, ctx, module, db, group.ID, "Leaving")
	staying := openVisit(t, ctx, module, db, group.ID, "Staying")

	careExit := startTx(t, ctx, db)
	awaitStep(t, careExit.run(func(txCtx context.Context) error {
		return module.LockOpenPresence(txCtx, []int64{leaving.StudentID})
	}))
	sessionEnd := startTx(t, ctx, db)
	sessionLock := sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		return err
	})
	endSession := func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, time.Now())
		return err
	}
	closePresence := func(txCtx context.Context) error {
		_, err := module.CloseOpenPresence(txCtx, []int64{leaving.StudentID}, time.Now())
		return err
	}
	blocked, err := awaitBlockedOrDone(t, db, sessionEnd, careExit, sessionLock)
	if !blocked {
		require.NoError(t, err)
		ending := sessionEnd.run(endSession)
		closing := careExit.run(closePresence)
		require.NoError(t, <-closing, "the care exit must not deadlock with the session end")
		require.NoError(t, <-ending, "the session end must not deadlock with the care exit")
		t.Fatal("the session end did not wait for the session the care exit locked")
	}
	awaitStep(t, careExit.run(closePresence))
	careExit.commit(t)
	awaitStep(t, sessionLock)
	awaitStep(t, sessionEnd.run(endSession))
	sessionEnd.commit(t)

	assert.Equal(t, countSnapshot{count: 0, revision: 4}, readSnapshot(t, ctx, module, group.ID),
		"two openings, the care exit and the session end each raised the revision")
	assert.NotZero(t, staying.ID)
}
