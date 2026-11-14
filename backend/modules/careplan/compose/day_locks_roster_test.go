package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The roster bundle (#6981) and the single-day path must take the same
// care-day keys, in an order every writer shares, or they stop protecting
// each other. The dates cross a year boundary.
const (
	rosterDay1 = "2026-12-28"
	rosterDay2 = "2027-01-04"
	rosterDay3 = "2027-01-11"
)

const rosterLockWait = 10 * time.Second

type rosterLockRig struct {
	t        *testing.T
	db       *bun.DB
	tenantID int64
	student  int64
	locks    DayLocks
}

// newRosterLockRig binds the day locks to the student row lock of the People
// Directory table. Without studentRows the bundle's student step locks
// nothing, which isolates the order of its care-day locks.
func newRosterLockRig(t *testing.T, studentRows bool) rosterLockRig {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Bündel", "Sperre", "2a")
	lockRow := func(ctx context.Context, id int64) error {
		_, err := lockStudentProfiles(ctx, []int64{id})
		return err
	}
	lockRows := func(_ context.Context, ids []int64) (int, error) { return len(ids), nil }
	if studentRows {
		lockRows = lockStudentProfiles
	}
	locks, err := NewDayLocks(db, lockRow, errors.New("student not found"), lockRows)
	require.NoError(t, err)
	return rosterLockRig{t: t, db: db, tenantID: testpkg.Tenant(t), student: student.ID, locks: locks}
}

func lockStudentProfiles(ctx context.Context, ids []int64) (int, error) {
	db, tenantID, err := postgres.AmbientCarePlanDatabase(ctx)
	if err != nil {
		return 0, err
	}
	rows := []int64{}
	err = db.NewRaw(`SELECT id FROM users.student_profiles WHERE tenant_id = ? AND id IN (?) ORDER BY id FOR UPDATE`, tenantID, bun.List(ids)).Scan(ctx, &rows)
	return len(rows), err
}

// holder is one writer's transaction that keeps its locks until released.
type holder struct {
	pid     int
	locked  chan struct{}
	release chan struct{}
	done    chan error
}

func (r rosterLockRig) hold(lock func(context.Context) error) *holder {
	r.t.Helper()
	h := &holder{locked: make(chan struct{}), release: make(chan struct{}), done: make(chan error, 1)}
	pids := make(chan int, 1)
	go func() {
		h.done <- testpkg.WithTenantTx(r.t, testpkg.Ctx(r.t), r.db, r.tenantID, func(ctx context.Context, tx bun.Tx) error {
			var pid int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				pids <- 0
				return err
			}
			pids <- pid
			if err := lock(ctx); err != nil {
				return err
			}
			close(h.locked)
			<-h.release
			return nil
		})
	}()
	h.pid = <-pids
	require.NotZero(r.t, h.pid, "the writer's transaction did not start")
	r.t.Cleanup(func() {
		select {
		case <-h.release:
		default:
			close(h.release)
		}
	})
	return h
}

func (r rosterLockRig) bundle(days ...string) func(context.Context) error {
	return func(ctx context.Context) error {
		return r.locks.LockRosterDays(ctx, map[int64][]string{r.student: days})
	}
}

func (r rosterLockRig) singleDay(date string) func(context.Context) error {
	return func(ctx context.Context) error { return r.locks.LockExceptionDay(ctx, r.student, date) }
}

func (r rosterLockRig) studentAndDay(date string) func(context.Context) error {
	return func(ctx context.Context) error { return r.locks.LockStudentAndExceptionDay(ctx, r.student, date) }
}

// awaitBlocked waits until the writer's backend waits for a lock.
func (r rosterLockRig) awaitBlocked(h *holder) {
	r.t.Helper()
	require.Eventually(r.t, func() bool {
		var waiting bool
		err := r.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = ? AND NOT granted)", h.pid).Scan(testpkg.Ctx(r.t), &waiting)
		return err == nil && waiting
	}, rosterLockWait, 5*time.Millisecond, "the writer must wait for a held lock")
	select {
	case <-h.locked:
		r.t.Fatal("the writer took its locks although one of them is held")
	default:
	}
}

func (r rosterLockRig) finish(h *holder) {
	r.t.Helper()
	select {
	case <-h.locked:
	case err := <-h.done:
		require.NoError(r.t, err, "the writer failed instead of taking its locks")
		r.t.Fatal("the writer finished without taking its locks")
	case <-time.After(rosterLockWait):
		r.t.Fatal("the writer did not get its locks")
	}
	close(h.release)
	select {
	case err := <-h.done:
		require.NoError(r.t, err)
	case <-time.After(rosterLockWait):
		r.t.Fatal("the writer did not commit")
	}
}

// free reports whether the single-day path gets the student's care day now:
// a lock timeout means another transaction holds it.
func (r rosterLockRig) free(student int64, date string) bool {
	r.t.Helper()
	err := testpkg.WithTenantTx(r.t, testpkg.Ctx(r.t), r.db, r.tenantID, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '100ms'"); err != nil {
			return err
		}
		return r.locks.LockExceptionDay(ctx, student, date)
	})
	if err == nil {
		return true
	}
	var pgErr pgdriver.Error
	require.True(r.t, errors.As(err, &pgErr) && pgErr.Field('C') == "55P03", "unexpected lock error: %v", err)
	return false
}

func TestRosterDayLocksHoldTheSingleDayKeys(t *testing.T) {
	t.Parallel()
	r := newRosterLockRig(t, true)
	other := testpkg.CreateTestStudent(t, r.db, "Anderes", "Kind", "2a")

	bundle := r.hold(r.bundle(rosterDay3, rosterDay1))
	select {
	case <-bundle.locked:
	case err := <-bundle.done:
		t.Fatalf("bundle failed: %v", err)
	case <-time.After(rosterLockWait):
		t.Fatal("bundle did not take its locks")
	}

	assert.False(t, r.free(r.student, rosterDay1), "the bundle holds the first day under the single-day key")
	assert.False(t, r.free(r.student, rosterDay3), "the bundle holds the last day under the single-day key")
	assert.True(t, r.free(r.student, rosterDay2), "a day outside the bundle stays free")
	assert.True(t, r.free(other.ID, rosterDay1), "another child's day stays free")
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), r.db, r.tenantID, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '100ms'"); err != nil {
			return err
		}
		return r.locks.LockStudentAndExceptionDay(ctx, r.student, rosterDay2)
	})
	var pgErr pgdriver.Error
	require.True(t, errors.As(err, &pgErr) && pgErr.Field('C') == "55P03", "the bundle holds the student row first: %v", err)

	r.finish(bundle)
	assert.True(t, r.free(r.student, rosterDay1), "commit releases the bundle's days")
}

// One statement takes all days; its order is shown by stopping it on the
// middle day: the earlier day is held, the later one is not taken yet.
func TestRosterDayLocksTakeDaysInDateOrder(t *testing.T) {
	t.Parallel()
	r := newRosterLockRig(t, false)

	blocker := r.hold(r.singleDay(rosterDay2))
	<-blocker.locked
	bundle := r.hold(r.bundle(rosterDay3, rosterDay1, rosterDay2))
	r.awaitBlocked(bundle)

	assert.False(t, r.free(r.student, rosterDay1), "the bundle took the earliest day before it waited")
	assert.True(t, r.free(r.student, rosterDay3), "the bundle takes no later day while it waits")

	r.finish(blocker)
	r.finish(bundle)
}

// Two bundles over the same days, given in opposite order, both start at the
// earliest day: neither holds a day the other waits for.
func TestRosterDayLockBundlesInOppositeOrderDoNotDeadlock(t *testing.T) {
	t.Parallel()
	r := newRosterLockRig(t, false)

	blocker := r.hold(r.singleDay(rosterDay1))
	<-blocker.locked
	ascending := r.hold(r.bundle(rosterDay1, rosterDay2, rosterDay3))
	descending := r.hold(r.bundle(rosterDay3, rosterDay2, rosterDay1))
	r.awaitBlocked(ascending)
	r.awaitBlocked(descending)
	assert.True(t, r.free(r.student, rosterDay2), "no bundle passed the earliest day")
	assert.True(t, r.free(r.student, rosterDay3), "the descending input did not start with the latest day")

	r.finish(blocker)
	var first, second *holder
	select {
	case <-ascending.locked:
		first, second = ascending, descending
	case <-descending.locked:
		first, second = descending, ascending
	case err := <-ascending.done:
		t.Fatalf("ascending bundle failed: %v", err)
	case err := <-descending.done:
		t.Fatalf("descending bundle failed: %v", err)
	case <-time.After(rosterLockWait):
		t.Fatal("no bundle got its locks")
	}
	r.awaitBlocked(second)
	r.finish(first)
	r.finish(second)
}

// A bundle and a single-day writer on the same child and day exclude each
// other in both directions and finish one after the other.
func TestRosterDayLocksAndSingleDayWritersExcludeEachOther(t *testing.T) {
	t.Parallel()
	r := newRosterLockRig(t, true)

	single := r.hold(r.studentAndDay(rosterDay2))
	<-single.locked
	bundle := r.hold(r.bundle(rosterDay1, rosterDay2, rosterDay3))
	r.awaitBlocked(bundle)
	r.finish(single)
	<-bundle.locked

	dayOnly := r.hold(r.singleDay(rosterDay3))
	r.awaitBlocked(dayOnly)
	withStudent := r.hold(r.studentAndDay(rosterDay1))
	r.awaitBlocked(withStudent)
	r.finish(bundle)
	r.finish(dayOnly)
	r.finish(withStudent)
}
