package compose_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The visit revision of a session orders the kiosk counter (#5128, ADR
// 0131). It is the sum of the session's append-only change events a read
// sees. These tests pin its invariants: every committed change of the open
// visit count raises it, also deletes and moves, nothing else does, a read
// that started before a write committed never carries that write's events,
// visit writes take no lock they did not take before, the nightly
// compaction keeps every revision, and the deadlocks of the earlier
// lock-based design stay gone.

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

// lockScenarioDB gives a test that holds transactions open its own pool on
// the package clone (#5269). Its actors, the lock diagnostics and the reads
// in between take connections from this pool, so concurrent lock scenarios
// cannot exhaust the shared pool and wait for each other's connections.
func lockScenarioDB(t *testing.T) *bun.DB {
	t.Helper()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	return db
}

// txActor runs one tenant transaction in its own goroutine, so a test can
// hold it open, let it block on a lock, and decide when it commits.
type txActor struct {
	pid     int
	tx      bun.Tx
	steps   chan func(context.Context) error
	done    chan error
	release sync.Once
}

func startTx(t *testing.T, ctx context.Context, db *bun.DB) *txActor {
	t.Helper()
	txCtx, cancel := context.WithCancel(ctx)
	actor := &txActor{steps: make(chan func(context.Context) error), done: make(chan error, 1)}
	// A failed assertion leaves the actor open: end it, cancelling a step
	// that still waits on a lock, so the pool can close.
	t.Cleanup(func() {
		actor.finish()
		cancel()
		if actor.ended(time.Second) {
			return
		}
		cancelCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := db.ExecContext(cancelCtx, "SELECT pg_cancel_backend(?)", actor.pid)
		stop()
		assert.NoError(t, err)
		if !actor.ended(10 * time.Second) {
			t.Error("transaction did not end during cleanup")
		}
	})
	ready := make(chan int, 1)
	go func() {
		err := testpkg.WithTenantTx(t, txCtx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var pid int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
				return err
			}
			actor.tx = tx
			ready <- pid
			for step := range actor.steps {
				if err := step(txCtx); err != nil {
					return err
				}
			}
			return nil
		})
		actor.done <- err
		close(actor.done)
	}()
	select {
	case actor.pid = <-ready:
	case err := <-actor.done:
		t.Fatalf("transaction did not start: %v", err)
	}
	return actor
}

// finish lets the actor's transaction commit once its current step returns.
func (a *txActor) finish() {
	a.release.Do(func() { close(a.steps) })
}

// ended reports whether the actor's transaction finished within the wait.
func (a *txActor) ended(wait time.Duration) bool {
	select {
	case <-a.done:
		return true
	case <-time.After(wait):
		return false
	}
}

// TestTxActorCleanupEndsAnOpenTransaction pins the actor cleanup of #5269: an
// actor left open by a failed test, even one waiting on a lock, ends with
// its test, so the lock scenario's pool can close.
func TestTxActorCleanupEndsAnOpenTransaction(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	group := newSession(t, db, "Actor cleanup")
	var holder, waiter *txActor
	t.Run("without commit", func(t *testing.T) {
		holder = startTx(t, ctx, db)
		waiter = startTx(t, ctx, db)
		awaitStep(t, holder.run(func(txCtx context.Context) error {
			_, err := holder.tx.NewRaw("SELECT id FROM active.groups WHERE id = ? FOR UPDATE", group.ID).Exec(txCtx)
			return err
		}))
		waiter.run(func(txCtx context.Context) error {
			_, err := waiter.tx.NewRaw("SELECT id FROM active.groups WHERE id = ? FOR UPDATE", group.ID).Exec(txCtx)
			return err
		})
		awaitBlockedBy(t, db, waiter, holder)
	})
	for _, actor := range []*txActor{holder, waiter} {
		select {
		case <-actor.done:
		default:
			t.Fatal("cleanup left an actor's transaction open")
		}
	}
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
	a.finish()
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
		queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := db.NewRaw("SELECT ?::int = ANY (pg_blocking_pids(?::int))", holder.pid, waiter.pid).Scan(queryCtx, &blocked)
		cancel()
		require.NoError(t, err)
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
	db := lockScenarioDB(t)
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

func countEventRows(t *testing.T, db *bun.DB, groupID int64) int {
	t.Helper()
	var rows int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM active.visit_revision_events WHERE active_group_id = ?", groupID).Scan(context.Background(), &rows))
	return rows
}

func newSession(t *testing.T, db *bun.DB, name string) *testpkg.ActiveGroupRow {
	t.Helper()
	activity := testpkg.CreateTestActivityGroup(t, db, name+" activity")
	room := testpkg.CreateTestRoom(t, db, name+" room")
	return testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
}

// TestVisitWritesTakeNoSessionLock pins that the revision adds no lock edge:
// while another transaction holds a session FOR UPDATE (as a session end
// does), closing, moving and deleting its visits complete without waiting,
// and two check-ins into one session do not wait for each other.
func TestVisitWritesTakeNoSessionLock(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	group := newSession(t, db, "No lock")
	other := newSession(t, db, "No lock other")
	closing := openVisit(t, ctx, module, db, group.ID, "Closing")
	moving := openVisit(t, ctx, module, db, group.ID, "Moving")
	deleting := openVisit(t, ctx, module, db, group.ID, "Deleting")
	before := readSnapshot(t, ctx, module, group.ID)

	holder := startTx(t, ctx, db)
	awaitStep(t, holder.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		return err
	}))
	writer := startTx(t, ctx, db)
	awaitStep(t, writer.run(func(txCtx context.Context) error {
		if _, err := module.CloseVisits(txCtx, []int64{closing.ID}, time.Now()); err != nil {
			return err
		}
		moved := moving
		moved.ActiveGroupID = other.ID
		if _, err := module.ReviseVisit(txCtx, moved); err != nil {
			return err
		}
		return module.DeleteVisit(txCtx, deleting.ID)
	}))
	writer.commit(t)
	holder.commit(t)
	assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 3}, readSnapshot(t, ctx, module, group.ID))
	assert.Equal(t, countSnapshot{count: 1, revision: 1}, readSnapshot(t, ctx, module, other.ID))

	first := startTx(t, ctx, db)
	second := startTx(t, ctx, db)
	firstStudent := testpkg.CreateTestStudent(t, db, "First", "Parallel", "3a")
	secondStudent := testpkg.CreateTestStudent(t, db, "Second", "Parallel", "3a")
	record := func(studentID int64) func(context.Context) error {
		return func(txCtx context.Context) error {
			_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: studentID, ActiveGroupID: other.ID, EntryTime: time.Now()})
			return err
		}
	}
	awaitStep(t, first.run(record(firstStudent.ID)))
	awaitStep(t, second.run(record(secondStudent.ID)))
	second.commit(t)
	first.commit(t)
	assert.Equal(t, countSnapshot{count: 3, revision: 3}, readSnapshot(t, ctx, module, other.ID))
}

// TestConcurrentWritersOfOneSessionMeetOnTheNextRead pins the remaining
// blur ADR 0131 accepts: two writers of one session that overlap each read
// their own change and the other's absence, so both answers can carry the
// same revision. No answer is older than a committed state, and the next
// read after both commits carries both changes with a higher revision.
func TestConcurrentWritersOfOneSessionMeetOnTheNextRead(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	group := newSession(t, db, "Overlap")
	present := openVisit(t, ctx, module, db, group.ID, "Present")
	before := readSnapshot(t, ctx, module, group.ID)
	arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Overlap", "3a")

	checkin := startTx(t, ctx, db)
	checkout := startTx(t, ctx, db)
	var checkinAnswer, checkoutAnswer countSnapshot
	awaitStep(t, checkin.run(func(txCtx context.Context) error {
		if _, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: group.ID, EntryTime: time.Now()}); err != nil {
			return err
		}
		checkinAnswer = readSnapshot(t, txCtx, module, group.ID)
		return nil
	}))
	awaitStep(t, checkout.run(func(txCtx context.Context) error {
		if _, err := module.CloseVisits(txCtx, []int64{present.ID}, time.Now()); err != nil {
			return err
		}
		checkoutAnswer = readSnapshot(t, txCtx, module, group.ID)
		return nil
	}))
	checkin.commit(t)
	checkout.commit(t)

	assert.Equal(t, countSnapshot{count: 2, revision: before.revision + 1}, checkinAnswer)
	assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 1}, checkoutAnswer)
	assert.Equal(t, countSnapshot{count: 1, revision: before.revision + 2}, readSnapshot(t, ctx, module, group.ID),
		"the next read carries both changes above both answers")
}

// TestNightlyCloseCompactsEventsWithoutChangingRevisions pins the cleanup:
// the nightly close replaces the events of ended sessions by one row each
// and drops those of deleted sessions; every revision stays, also for a
// session reopened afterwards and for an event a concurrent writer commits
// while the compaction runs.
func TestNightlyCloseCompactsEventsWithoutChangingRevisions(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	ended := newSession(t, db, "Compact ended")
	deleted := newSession(t, db, "Compact deleted")
	running := newSession(t, db, "Compact running")
	stillOpen := openVisit(t, ctx, module, db, ended.ID, "Late")
	leaving := openVisit(t, ctx, module, db, ended.ID, "Leaving")
	_, err := module.CloseVisits(ctx, []int64{leaving.ID}, time.Now())
	require.NoError(t, err)
	gone := openVisit(t, ctx, module, db, deleted.ID, "Gone")
	require.NoError(t, module.DeleteVisit(ctx, gone.ID))
	openVisit(t, ctx, module, db, running.ID, "Running")
	_, err = db.NewRaw("UPDATE active.groups SET end_time = NOW() WHERE id = ?", ended.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("DELETE FROM active.groups WHERE id = ?", deleted.ID).Exec(ctx)
	require.NoError(t, err)
	endedBefore := readSnapshot(t, ctx, module, ended.ID)
	require.Equal(t, 3, countEventRows(t, db, ended.ID))

	writer := startTx(t, ctx, db)
	awaitStep(t, writer.run(func(txCtx context.Context) error {
		_, err := module.CloseVisits(txCtx, []int64{stillOpen.ID}, time.Now())
		return err
	}))
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, err := module.EndGroupSessions(txCtx, []int64{running.ID}, time.Now())
		return err
	}))
	writer.commit(t)

	assert.Equal(t, countSnapshot{count: 0, revision: endedBefore.revision + 1}, readSnapshot(t, ctx, module, ended.ID),
		"the compaction keeps the revision and the concurrent writer's event")
	assert.Equal(t, 2, countEventRows(t, db, ended.ID), "one compacted row and the concurrent writer's event")
	assert.Zero(t, countEventRows(t, db, deleted.ID), "a deleted session's events go")
	runningAfter := readSnapshot(t, ctx, module, running.ID)
	assert.Equal(t, countSnapshot{count: 0, revision: 2}, runningAfter, "the nightly close of the running session is its second change")
	assert.Equal(t, 1, countEventRows(t, db, running.ID))

	_, err = db.NewRaw("UPDATE active.groups SET end_time = NULL WHERE id = ?", running.ID).Exec(ctx)
	require.NoError(t, err)
	openVisit(t, ctx, module, db, running.ID, "Reopened")
	assert.Equal(t, countSnapshot{count: 1, revision: runningAfter.revision + 1}, readSnapshot(t, ctx, module, running.ID),
		"a reopened session continues above its compacted revision")
}

func TestNightlyCloseCompactsEventsWithNoOpenGroups(t *testing.T) {
	t.Parallel()
	for _, suppliedIDs := range []bool{false, true} {
		name := "empty IDs"
		if suppliedIDs {
			name = "already ended IDs"
		}
		t.Run(name, func(t *testing.T) {
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.OwnCtx(t)
			module := newRevisionModule(t, db)
			group := newSession(t, db, "Manual close")
			visit := openVisit(t, ctx, module, db, group.ID, "Manual")
			_, err := module.CloseVisits(ctx, []int64{visit.ID}, time.Now())
			require.NoError(t, err)
			_, err = db.NewRaw("UPDATE active.groups SET end_time = NOW() WHERE id = ?", group.ID).Exec(ctx)
			require.NoError(t, err)
			before := readSnapshot(t, ctx, module, group.ID)
			require.Equal(t, 2, countEventRows(t, db, group.ID))
			var ids []int64
			if suppliedIDs {
				ids = []int64{group.ID}
			}
			require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
				result, err := module.EndGroupSessions(txCtx, ids, time.Now())
				assert.Zero(t, result.SessionsEnded)
				return err
			}))
			assert.Equal(t, 1, countEventRows(t, db, group.ID))
			assert.Equal(t, before, readSnapshot(t, ctx, module, group.ID))
		})
	}
}

// The tests below replay the deadlocks both review rounds reproduced
// against the lock-based design. They interleave the writers the way the
// production paths do; with the event table no writer locks a session it
// did not lock before, and every pair finishes.

// TestOppositeScansDoNotDeadlock: two scans move children in opposite
// directions between two sessions (checkout, then the check-in's target
// lock and insert).
func TestOppositeScansDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	first := newSession(t, db, "Opposite first")
	second := newSession(t, db, "Opposite second")
	forwardVisit := openVisit(t, ctx, module, db, first.ID, "Forward")
	backwardVisit := openVisit(t, ctx, module, db, second.ID, "Backward")
	checkout := func(visit studentpresence.Visit) func(context.Context) error {
		return func(txCtx context.Context) error {
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
	backward := startTx(t, ctx, db)
	awaitStep(t, forward.run(checkout(forwardVisit)))
	awaitStep(t, backward.run(checkout(backwardVisit)))
	awaitStep(t, forward.run(checkin(forwardVisit.StudentID, second.ID)))
	awaitStep(t, backward.run(checkin(backwardVisit.StudentID, first.ID)))
	forward.commit(t)
	backward.commit(t)

	assert.Equal(t, countSnapshot{count: 1, revision: 3}, readSnapshot(t, ctx, module, first.ID))
	assert.Equal(t, countSnapshot{count: 1, revision: 3}, readSnapshot(t, ctx, module, second.ID))
}

// TestCheckoutAndSessionEndDoNotDeadlock: a checkout closes a visit while a
// session end holds the session and then closes its visits.
func TestCheckoutAndSessionEndDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	group := newSession(t, db, "Checkout end")
	visit := openVisit(t, ctx, module, db, group.ID, "Leaving")

	sessionEnd := startTx(t, ctx, db)
	checkout := startTx(t, ctx, db)
	awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		return err
	}))
	awaitStep(t, checkout.run(func(txCtx context.Context) error {
		_, err := module.CloseVisits(txCtx, []int64{visit.ID}, time.Now())
		return err
	}))
	ending := sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, time.Now())
		return err
	})
	awaitBlockedBy(t, db, sessionEnd, checkout)
	checkout.commit(t)
	awaitStep(t, ending)
	sessionEnd.commit(t)

	assert.Equal(t, countSnapshot{count: 0, revision: 2}, readSnapshot(t, ctx, module, group.ID))
}

// TestCareExitAndSessionEndDoNotDeadlock: a care exit locks the child's
// presence and closes it while a session end of the child's session holds
// the session and waits for the visit.
func TestCareExitAndSessionEndDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	group := newSession(t, db, "Care exit")
	leaving := openVisit(t, ctx, module, db, group.ID, "Leaving")
	openVisit(t, ctx, module, db, group.ID, "Staying")

	careExit := startTx(t, ctx, db)
	sessionEnd := startTx(t, ctx, db)
	awaitStep(t, careExit.run(func(txCtx context.Context) error {
		return module.LockOpenPresence(txCtx, []int64{leaving.StudentID})
	}))
	awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		return err
	}))
	ending := sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, time.Now())
		return err
	})
	awaitBlockedBy(t, db, sessionEnd, careExit)
	awaitStep(t, careExit.run(func(txCtx context.Context) error {
		_, err := module.CloseOpenPresence(txCtx, []int64{leaving.StudentID}, time.Now())
		return err
	}))
	careExit.commit(t)
	awaitStep(t, ending)
	sessionEnd.commit(t)

	assert.Equal(t, countSnapshot{count: 0, revision: 4}, readSnapshot(t, ctx, module, group.ID),
		"two openings, the care exit and the session end each raised the revision")
}

// TestAdminTransferWaitingCheckoutAndSessionEndDoNotDeadlock: an admin
// moves a visit to another session (target guard lock, then the update)
// while a checkout of that visit waits; the checkout then closes the moved
// visit, and a session end of the target waits for it.
func TestAdminTransferWaitingCheckoutAndSessionEndDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	source := newSession(t, db, "Transfer source")
	target := newSession(t, db, "Transfer target")
	visit := openVisit(t, ctx, module, db, source.ID, "Moved")

	admin := startTx(t, ctx, db)
	checkout := startTx(t, ctx, db)
	sessionEnd := startTx(t, ctx, db)
	awaitStep(t, admin.run(func(txCtx context.Context) error {
		if _, err := module.LockGroup(txCtx, target.ID); err != nil {
			return err
		}
		moved := visit
		moved.ActiveGroupID = target.ID
		_, err := module.ReviseVisit(txCtx, moved)
		return err
	}))
	closing := checkout.run(func(txCtx context.Context) error {
		_, err := module.CloseVisits(txCtx, []int64{visit.ID}, time.Now())
		return err
	})
	awaitBlockedBy(t, db, checkout, admin)
	admin.commit(t)
	awaitStep(t, closing)
	awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, target.ID)
		return err
	}))
	ending := sessionEnd.run(func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, target.ID, time.Now())
		return err
	})
	awaitBlockedBy(t, db, sessionEnd, checkout)
	checkout.commit(t)
	awaitStep(t, ending)
	sessionEnd.commit(t)

	assert.Equal(t, countSnapshot{count: 0, revision: 2}, readSnapshot(t, ctx, module, source.ID), "opened, then moved out")
	assert.Equal(t, countSnapshot{count: 0, revision: 2}, readSnapshot(t, ctx, module, target.ID), "moved in, then closed")
}

// TestReattachAndScanDoNotDeadlock: a device reattaches to its session (the
// session row update) and takes over the open visits of its recently ended
// session, while a scan checks a child out of that ended session into
// another room's session.
func TestReattachAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	device := testpkg.CreateTestDevice(t, db, "reattach-device")
	ended := newSession(t, db, "Reattach ended")
	reattached := newSession(t, db, "Reattach target")
	elsewhere := newSession(t, db, "Reattach elsewhere")
	testpkg.LinkDeviceToActiveGroup(t, db, ended.ID, device.ID)
	scanned := openVisit(t, ctx, module, db, ended.ID, "Scanned")
	openVisit(t, ctx, module, db, ended.ID, "TakenOver")
	_, err := db.NewRaw("UPDATE active.groups SET end_time = NOW() - INTERVAL '5 minutes' WHERE id = ?", ended.ID).Exec(ctx)
	require.NoError(t, err)

	reattach := startTx(t, ctx, db)
	scan := startTx(t, ctx, db)
	awaitStep(t, reattach.run(func(txCtx context.Context) error {
		_, err := reattach.tx.NewRaw("UPDATE active.groups SET device_id = ? WHERE id = ?", device.ID, reattached.ID).Exec(txCtx)
		return err
	}))
	awaitStep(t, scan.run(func(txCtx context.Context) error {
		_, err := module.CloseVisits(txCtx, []int64{scanned.ID}, time.Now())
		return err
	}))
	takeOver := reattach.run(func(txCtx context.Context) error {
		_, err := module.TransferRecentDeviceVisits(txCtx, reattached.ID, device.ID)
		return err
	})
	awaitBlockedBy(t, db, reattach, scan)
	awaitStep(t, scan.run(func(txCtx context.Context) error {
		if _, err := module.LockGroup(txCtx, elsewhere.ID); err != nil {
			return err
		}
		_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: scanned.StudentID, ActiveGroupID: elsewhere.ID, EntryTime: time.Now()})
		return err
	}))
	scan.commit(t)
	awaitStep(t, takeOver)
	reattach.commit(t)

	assert.Equal(t, 1, readSnapshot(t, ctx, module, reattached.ID).count, "the child still in the ended session moved over")
	assert.Equal(t, countSnapshot{count: 1, revision: 1}, readSnapshot(t, ctx, module, elsewhere.ID))
	assert.Equal(t, countSnapshot{count: 0, revision: 4}, readSnapshot(t, ctx, module, ended.ID), "two openings, the checkout and the takeover")
}
