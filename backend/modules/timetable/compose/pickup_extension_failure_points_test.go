package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Failure points of a weekday pickup extension resolution (#5816). The
// resolution locks a template's planned blocks in planned (date, id) order,
// one statement per block, and checks each under its lock. These tests pin
// that contract for any later attempt to batch the locks: every failure point
// keeps its status, its lock prefix and its persistence. They use only the
// public capability and plain SQL.

var pickupChainTuesdays = []string{"2099-03-10", "2099-03-17", "2099-03-24", "2099-03-31", "2099-04-07", "2099-04-14", "2099-04-21", "2099-04-28"}

type pickupChainOptions struct {
	// creation lists the template's block dates in creation (id) order;
	// the planned order is always by date.
	creation []string
	sessions *fixedSessions
	// second adds another template on the same weekday and dates.
	second bool
}

type pickupChainFixture struct {
	*pickupLockFixture
	staffID        int64
	secondTemplate int64
	second         []int64
}

func newPickupChainFixture(t *testing.T, db *bun.DB, opts pickupChainOptions) *pickupChainFixture {
	t.Helper()
	if opts.creation == nil {
		opts.creation = pickupChainTuesdays[:3]
	}
	sessions := opts.sessions
	if sessions == nil {
		sessions = &fixedSessions{}
	}
	module, ctx := buildPickupExtensionModuleWithSessions(t, db, sessions), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup chain %d", suffix))
	other := testpkg.CreateTestStudent(t, db, "Ida", fmt.Sprintf("Chain-%d", suffix), "1a")
	child := testpkg.CreateTestStudent(t, db, "Emil", fmt.Sprintf("Chain-%d", suffix), "1a")
	staff := testpkg.CreateTestStaff(t, db, "Rita", fmt.Sprintf("Chain-%d", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Pickup chain room %d", suffix))
	template, instances := pickupChainTemplate(t, module, ctx, category.ID, room.ID, other.ID, "Freies Spiel", "14:45:00", opts.creation)
	fixture := &pickupChainFixture{pickupLockFixture: &pickupLockFixture{
		db: db, module: module, ctx: ctx, childID: child.ID, templateID: template, instances: instances,
	}, staffID: staff.ID}
	if opts.second {
		fixture.secondTemplate, fixture.second = pickupChainTemplate(t, module, ctx, category.ID, room.ID, other.ID, "Werkstatt", "15:00:00", opts.creation)
	}
	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	fixture.taskID = pickupExtensionTask(t, module, ctx, child.ID).ID
	return fixture
}

// pickupChainTemplate creates a Tuesday template with another child enrolled
// and its blocks in the given creation order; it returns them in planned
// (date, id) order.
func pickupChainTemplate(t *testing.T, module timetable.Owner, ctx context.Context, categoryID, roomID, otherID int64, name, start string, creation []string) (int64, []int64) {
	t.Helper()
	template := pickupExtensionTemplate(t, module, ctx, categoryID, name, start, "16:00:00", timetable.WeekdayTuesday)
	_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: otherID, ActivityGroupID: template.ID, ValidFrom: "2020-01-01",
	})
	require.NoError(t, err)
	type block struct {
		id   int64
		date string
	}
	blocks := make([]block, 0, len(creation))
	for _, date := range creation {
		instance := pickupExtensionInstance(t, module, ctx, ownedActivityInstanceFixture{roomID: roomID, groupID: template.ID}, date, start, "16:00:00", name)
		blocks = append(blocks, block{id: instance.ID, date: date})
	}
	slices.SortFunc(blocks, func(a, b block) int {
		if c := strings.Compare(a.date, b.date); c != 0 {
			return c
		}
		return int(a.id - b.id)
	})
	ids := make([]int64, 0, len(blocks))
	for _, b := range blocks {
		ids = append(ids, b.id)
	}
	return template.ID, ids
}

func (f *pickupChainFixture) resolveBlocksAsync(blockIDs ...int64) <-chan pickupResolveOutcome {
	done := make(chan pickupResolveOutcome, 1)
	go func() {
		resolved, err := f.module.ResolvePickupExtension(f.ctx, f.taskID, blockIDs)
		done <- pickupResolveOutcome{resolved: resolved, err: err}
	}()
	return done
}

// holdWith opens a transaction, runs one statement in it and keeps it open.
func (f *pickupChainFixture) holdWith(t *testing.T, statement string, args ...any) bun.Tx {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), &sql.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(), statement, args...)
	require.NoError(t, err)
	return tx
}

// assertNothingWritten checks the refusal kept the child off every block, left
// its enrollments alone and kept the task open.
func (f *pickupChainFixture) assertNothingWritten(t *testing.T) {
	t.Helper()
	roster, err := f.module.ListInstanceStudents(f.ctx, timetable.InstanceStudentFilter{
		InstanceIDs: append(slices.Clone(f.instances), f.second...), StudentIDs: []int64{f.childID},
	})
	require.NoError(t, err)
	for _, row := range roster {
		assert.Fail(t, "participant written", "instance %d", row.InstanceID)
	}
	enrollments, err := f.module.ListStudentEnrollments(f.ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{f.childID}})
	require.NoError(t, err)
	for _, enrollment := range enrollments {
		assert.NotContains(t, enrollment.SelectedWeekdays, timetable.WeekdayTuesday, "no enrollment carries the weekday")
		assert.Nil(t, enrollment.Weekday, "no weekday enrollment was created")
	}
	assert.Equal(t, 1, countPickupExtensionRows(t, f.db, f.childID), "the task stays open")
}

// resolveInTransaction runs the resolve inside an outer tenant transaction,
// like the HTTP route does, after the given statements on that transaction.
// inspect runs on the still open transaction after the resolve returned.
func (f *pickupChainFixture) resolveInTransaction(t *testing.T, setup []string, inspect func(error)) error {
	t.Helper()
	var resolveErr error
	err := testpkg.TenantCommand0(t, f.ctx, f.db, func(txCtx context.Context) error {
		value, ok := testpkg.TransactionFromContext(txCtx)
		require.True(t, ok)
		idb, ok := value.(bun.IDB)
		require.True(t, ok, "transaction %T", value)
		for _, statement := range setup {
			_, err := idb.ExecContext(txCtx, statement)
			require.NoError(t, err)
		}
		_, resolveErr = f.module.ResolvePickupExtension(txCtx, f.taskID, []int64{f.templateID})
		if inspect != nil {
			inspect(resolveErr)
		}
		return resolveErr
	})
	if resolveErr == nil {
		return err
	}
	return resolveErr
}

// privateClonePool opens a second pool on the test's isolated clone with the
// given connection parameters (driver settings such as read_timeout, or
// session settings such as deadlock_timeout) and closes it before the clone
// is dropped.
func privateClonePool(t *testing.T, params map[string]string) *bun.DB {
	t.Helper()
	dsn, err := url.Parse(testpkg.IsolatedTestDSN(t))
	require.NoError(t, err)
	query := dsn.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	dsn.RawQuery = query.Encode()
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(testpkg.OpenPostgresSQL(dsn.String())))
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

func sqlState(err error) string {
	var state interface{ Field(byte) string }
	if errors.As(err, &state) {
		return state.Field('C')
	}
	return ""
}

// resolverWaitingPID returns the backend that waits for an instance row lock.
func (f *pickupChainFixture) resolverWaitingPID(t *testing.T) int64 {
	t.Helper()
	f.waitForInstanceLockWait(t)
	var pid int64
	require.NoError(t, f.db.NewRaw(`SELECT pid FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%activity_instances%'
		ORDER BY pid LIMIT 1`).Scan(context.Background(), &pid))
	return pid
}

// A block that no longer fits before the resolve refuses with the block-gone
// conflict at once, although the next block is locked elsewhere: nothing
// after the refused block is locked or waited for.
func TestPickupChainRefusesMovedBlockWithoutWaitingForTheNext(t *testing.T) {
	t.Parallel()
	f := newPickupChainFixture(t, testpkg.SetupIsolatedTestDB(t), pickupChainOptions{})
	_, err := f.db.ExecContext(context.Background(), `UPDATE schedule.activity_instances SET start_time = '16:00', end_time = '16:30' WHERE id = ?`, f.instances[0])
	require.NoError(t, err)
	holder := f.holdInstance(t, f.instances[1], "")

	outcome := f.awaitOutcome(t, f.resolveBlocksAsync(f.templateID))
	require.ErrorIs(t, outcome.err, timetable.ErrPickupExtensionBlockGone)
	require.NoError(t, holder.Rollback())
	f.assertNothingWritten(t)
}

// With ids against the date order, the planned (date) order decides: the
// earlier block with the larger id refuses before the later block with the
// smaller id, which another transaction holds, is locked.
func TestPickupChainFollowsPlannedOrderAgainstIDOrder(t *testing.T) {
	t.Parallel()
	f := newPickupChainFixture(t, testpkg.SetupIsolatedTestDB(t), pickupChainOptions{
		creation: []string{pickupChainTuesdays[2], pickupChainTuesdays[0], pickupChainTuesdays[1]},
	})
	require.Greater(t, f.instances[0], f.instances[2], "the first planned block has the larger id")
	_, err := f.db.ExecContext(context.Background(), `UPDATE schedule.activity_instances SET start_time = '16:00', end_time = '16:30' WHERE id = ?`, f.instances[0])
	require.NoError(t, err)
	holder := f.holdInstance(t, f.instances[2], "")

	outcome := f.awaitOutcome(t, f.resolveBlocksAsync(f.templateID))
	require.ErrorIs(t, outcome.err, timetable.ErrPickupExtensionBlockGone)
	require.NoError(t, holder.Rollback())
	f.assertNothingWritten(t)
}

// A change committed while the resolve waits for a block is checked on the
// fresh row: every change that makes the block no choice refuses without
// locking the next block; a change that keeps it a choice lets the resolve
// go on to the next block. This pins the statement's check to the
// application's check, including the time bounds.
func TestPickupChainChecksTheFreshRowAfterWaiting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, tc := range []struct {
		name    string
		change  string
		refuses bool
	}{
		{name: "cancelled", change: `UPDATE schedule.activity_instances SET status = 'cancelled' WHERE id = ?`, refuses: true},
		{name: "spontaneous", change: `UPDATE schedule.activity_instances SET is_spontaneous = true WHERE id = ?`, refuses: true},
		{name: "moved to another date", change: `UPDATE schedule.activity_instances SET date = date + 1 WHERE id = ?`, refuses: true},
		{name: "detached from the template", change: `UPDATE schedule.activity_instances SET activity_group_id = NULL WHERE id = ?`, refuses: true},
		{name: "starts at the new pickup", change: `UPDATE schedule.activity_instances SET start_time = '16:00', end_time = '16:30' WHERE id = ?`, refuses: true},
		{name: "ends at the old pickup", change: `UPDATE schedule.activity_instances SET start_time = '14:00', end_time = '14:45' WHERE id = ?`, refuses: true},
		{name: "deleted", change: `DELETE FROM schedule.activity_instances WHERE id = ?`, refuses: true},
		{name: "starts a minute before the new pickup", change: `UPDATE schedule.activity_instances SET start_time = '15:59', end_time = '16:30' WHERE id = ?`},
		{name: "ends a minute after the old pickup", change: `UPDATE schedule.activity_instances SET start_time = '14:00', end_time = '14:46' WHERE id = ?`},
		{name: "renamed", change: `UPDATE schedule.activity_instances SET title = 'Umbenannt' WHERE id = ?`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPickupChainFixture(t, db, pickupChainOptions{})
			changer := f.holdWith(t, tc.change, f.instances[0])
			holder := f.holdInstance(t, f.instances[1], "")
			done := f.resolveBlocksAsync(f.templateID)
			f.waitForInstanceLockWait(t)
			require.NoError(t, changer.Commit())
			if tc.refuses {
				outcome := f.awaitOutcome(t, done)
				require.ErrorIs(t, outcome.err, timetable.ErrPickupExtensionBlockGone)
				require.NoError(t, holder.Rollback())
				f.assertNothingWritten(t)
				return
			}
			// The resolve now waits for the second block.
			deadline := time.Now().Add(15 * time.Second)
			for !f.instanceLockedElsewhere(t, f.instances[0]) {
				require.True(t, time.Now().Before(deadline), "the first block was never locked by the resolve")
				time.Sleep(20 * time.Millisecond)
			}
			f.waitForInstanceLockWait(t)
			require.NoError(t, holder.Rollback())
			outcome := f.awaitOutcome(t, done)
			require.NoError(t, outcome.err)
			assert.Equal(t, f.instances, outcome.resolved.InstanceIDs)
		})
	}
}

// A lock_timeout set by the caller ends the wait for the second block with
// 55P03. While the resolve waits, the first block is locked and the third is
// not; a resolve inside an outer transaction is not replayed, and the
// aborted transaction holds no lock afterwards.
func TestPickupChainLockTimeoutStopsAtTheWaitedBlock(t *testing.T) {
	t.Parallel()
	f := newPickupChainFixture(t, testpkg.SetupIsolatedTestDB(t), pickupChainOptions{})
	holder := f.holdInstance(t, f.instances[1], "")
	done := make(chan error, 1)
	go func() {
		done <- f.resolveInTransaction(t, []string{`SET LOCAL lock_timeout = '3s'`}, nil)
	}()
	f.waitForInstanceLockWait(t)
	assert.True(t, f.instanceLockedElsewhere(t, f.instances[0]), "the first block is locked while the second is awaited")
	assert.False(t, f.instanceLockedElsewhere(t, f.instances[2]), "the third block is not locked while the second is awaited")
	var err error
	select {
	case err = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the lock timeout never ended the resolve")
	}
	assert.Equal(t, "55P03", sqlState(err), "err=%v", err)
	assert.False(t, f.instanceLockedElsewhere(t, f.instances[0]), "the aborted resolve keeps no lock")
	require.NoError(t, holder.Rollback())
	f.assertNothingWritten(t)
}

// A cancelled wait (57014) ends the resolve with that error and writes
// nothing.
func TestPickupChainCancelledWaitWritesNothing(t *testing.T) {
	t.Parallel()
	f := newPickupChainFixture(t, testpkg.SetupIsolatedTestDB(t), pickupChainOptions{})
	holder := f.holdInstance(t, f.instances[1], "")
	done := f.resolveBlocksAsync(f.templateID)
	pid := f.resolverWaitingPID(t)
	assert.False(t, f.instanceLockedElsewhere(t, f.instances[2]), "the third block is not locked while the second is awaited")
	_, err := f.db.ExecContext(context.Background(), `SELECT pg_cancel_backend(?)`, pid)
	require.NoError(t, err)
	outcome := f.awaitOutcome(t, done)
	require.Error(t, outcome.err)
	assert.Equal(t, "57014", sqlState(outcome.err), "err=%v", outcome.err)
	require.NoError(t, holder.Rollback())
	f.assertNothingWritten(t)
}

// A deadlock while waiting for the second block: the resolve is the victim.
// In its own transaction it is replayed and assigns every block once; inside
// an outer transaction it ends with 40P01.
//
// The victim follows from the order of the waits, not from timing: a gate
// holds the first block and the holder the second. The resolve locks its task
// and waits at the gate; then the holder waits for the task. Only when the
// gate opens does the resolve lock the first block and wait for the second,
// and that wait closes the cycle. A backend checks for a deadlock once, after
// deadlock_timeout of its current wait: the resolve's check (server default,
// 1 s) always finds the closed cycle however late it runs, while the holder
// waited before the cycle existed and checks only after 30 s.
func TestPickupChainDeadlockReplaysOnlyItsOwnTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	deadlock := func(t *testing.T, f *pickupChainFixture, start func() <-chan pickupResolveOutcome) (bun.Tx, <-chan pickupResolveOutcome) {
		t.Helper()
		gate := f.holdInstance(t, f.instances[0], "")
		holder := f.holdInstance(t, f.instances[1], "")
		_, err := holder.ExecContext(context.Background(), `SET LOCAL deadlock_timeout = '30s'`)
		require.NoError(t, err)
		done := start()
		f.waitForLockWaiters(t, 1) // the resolve holds its task and waits at the gate
		crossed := make(chan error, 1)
		go func() {
			_, err := holder.ExecContext(context.Background(), `SELECT id FROM schedule.pickup_extension_tasks WHERE id = ? FOR UPDATE`, f.taskID)
			crossed <- err
		}()
		f.waitForLockWaiters(t, 2) // the holder waits for the task
		require.NoError(t, gate.Rollback())
		select {
		case err := <-crossed:
			require.NoError(t, err, "the holder is not the deadlock victim")
		case <-time.After(15 * time.Second):
			t.Fatal("the deadlock was never resolved")
		}
		return holder, done
	}

	t.Run("own transaction", func(t *testing.T) {
		f := newPickupChainFixture(t, db, pickupChainOptions{})
		holder, done := deadlock(t, f, func() <-chan pickupResolveOutcome { return f.resolveBlocksAsync(f.templateID) })
		require.NoError(t, holder.Rollback())
		outcome := f.awaitOutcome(t, done)
		require.NoError(t, outcome.err)
		assert.Equal(t, f.instances, outcome.resolved.InstanceIDs, "every block once")
		roster, err := f.module.ListInstanceStudents(f.ctx, timetable.InstanceStudentFilter{InstanceIDs: f.instances, StudentIDs: []int64{f.childID}})
		require.NoError(t, err)
		assert.Len(t, roster, len(f.instances), "one participant per block")
	})
	t.Run("outer transaction", func(t *testing.T) {
		f := newPickupChainFixture(t, db, pickupChainOptions{})
		holder, done := deadlock(t, f, func() <-chan pickupResolveOutcome {
			result := make(chan pickupResolveOutcome, 1)
			go func() { result <- pickupResolveOutcome{err: f.resolveInTransaction(t, nil, nil)} }()
			return result
		})
		outcome := f.awaitOutcome(t, done)
		assert.Equal(t, "40P01", sqlState(outcome.err), "err=%v", outcome.err)
		require.NoError(t, holder.Rollback())
		f.assertNothingWritten(t)
	})
}

// Enrollment failures after all blocks are locked end the resolve and keep
// everything: a failing insert of the weekday enrollment, and an update that
// finds no row (ErrStudentEnrollmentNotFound).
func TestPickupChainEnrollmentFailureWritesNothing(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	t.Run("create fails", func(t *testing.T) {
		f := newPickupChainFixture(t, db, pickupChainOptions{})
		createFailTrigger(t, db, "pickup_chain_enrollment_insert", "activities.student_enrollments", "BEFORE INSERT",
			fmt.Sprintf("NEW.student_id = %d", f.childID), "RAISE EXCEPTION 'injected enrollment failure';")
		_, err := f.module.ResolvePickupExtension(f.ctx, f.taskID, []int64{f.templateID})
		require.ErrorContains(t, err, "injected enrollment failure")
		f.assertNothingWritten(t)
	})
	t.Run("update finds no row", func(t *testing.T) {
		f := newPickupChainFixture(t, db, pickupChainOptions{})
		_, err := f.module.CreateStudentEnrollment(f.ctx, timetable.StudentEnrollmentInput{
			StudentID: f.childID, ActivityGroupID: f.templateID, ValidFrom: "2099-03-01", SelectedWeekdays: []int{timetable.WeekdayWednesday},
		})
		require.NoError(t, err)
		createFailTrigger(t, db, "pickup_chain_enrollment_update", "activities.student_enrollments", "BEFORE UPDATE",
			fmt.Sprintf("NEW.student_id = %d", f.childID), "RETURN NULL;")
		_, err = f.module.ResolvePickupExtension(f.ctx, f.taskID, []int64{f.templateID})
		require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
		f.assertNothingWritten(t)
	})
}

// Task failures: a missing task is not found before any lock; a failing task
// delete after every write rolls all of them back.
func TestPickupChainTaskFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newPickupChainFixture(t, db, pickupChainOptions{})
	_, err := f.module.ResolvePickupExtension(f.ctx, f.taskID+1_000_000, []int64{f.templateID})
	require.ErrorIs(t, err, timetable.ErrPickupExtensionNotFound)

	createFailTrigger(t, db, "pickup_chain_task_delete", "schedule.pickup_extension_tasks", "BEFORE DELETE",
		fmt.Sprintf("OLD.student_id = %d", f.childID), "RAISE EXCEPTION 'injected task delete failure';")
	_, err = f.module.ResolvePickupExtension(f.ctx, f.taskID, []int64{f.templateID})
	require.ErrorContains(t, err, "injected task delete failure")
	f.assertNothingWritten(t)
}

// Participant inserts keep their per-block order and statements: a failing
// insert of the second block ends the resolve; an AFTER INSERT failure of
// the first block ends it with P0001 at once, although the second block's
// existing participant row is locked elsewhere.
func TestPickupChainParticipantInsertFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	t.Run("second insert fails", func(t *testing.T) {
		f := newPickupChainFixture(t, db, pickupChainOptions{})
		createFailTrigger(t, db, "pickup_chain_participant_second", "schedule.instance_students", "BEFORE INSERT",
			fmt.Sprintf("NEW.student_id = %d AND NEW.instance_id = %d", f.childID, f.instances[1]), "RAISE EXCEPTION 'injected participant failure';")
		_, err := f.module.ResolvePickupExtension(f.ctx, f.taskID, []int64{f.templateID})
		require.ErrorContains(t, err, "injected participant failure")
		f.assertNothingWritten(t)
	})
	t.Run("after insert fails before a locked participant", func(t *testing.T) {
		sessions := &fixedSessions{}
		f := newPickupChainFixture(t, db, pickupChainOptions{sessions: sessions})
		existing := createOwnedInstanceStudent(t, f.module, f.ctx, f.instances[1], f.childID)
		sessions.notScheduled = []int64{existing.ID}
		createFailTrigger(t, db, "pickup_chain_participant_after", "schedule.instance_students", "AFTER INSERT",
			fmt.Sprintf("NEW.student_id = %d AND NEW.instance_id = %d", f.childID, f.instances[0]), "RAISE EXCEPTION 'injected after-insert failure';")
		holder := f.holdWith(t, `SELECT id FROM schedule.instance_students WHERE id = ? FOR UPDATE`, existing.ID)

		outcome := f.awaitOutcome(t, f.resolveBlocksAsync(f.templateID))
		assert.Equal(t, "P0001", sqlState(outcome.err), "err=%v", outcome.err)
		require.ErrorContains(t, outcome.err, "injected after-insert failure")
		require.NoError(t, holder.Rollback())
		roster, err := f.module.ListInstanceStudents(f.ctx, timetable.InstanceStudentFilter{InstanceIDs: f.instances, StudentIDs: []int64{f.childID}})
		require.NoError(t, err)
		require.Len(t, roster, 1, "only the existing participant")
		assert.Equal(t, existing.ID, roster[0].ID)
		assert.Equal(t, 1, countPickupExtensionRows(t, f.db, f.childID))
	})
}

// Two chosen templates: a refusal in the second template rolls the first
// template's enrollment and participants back.
func TestPickupChainRefusalInLaterTemplateRollsBackTheEarlier(t *testing.T) {
	t.Parallel()
	f := newPickupChainFixture(t, testpkg.SetupIsolatedTestDB(t), pickupChainOptions{second: true})
	_, err := f.db.ExecContext(context.Background(), `UPDATE schedule.activity_instances SET start_time = '16:00', end_time = '16:30' WHERE id = ?`, f.second[1])
	require.NoError(t, err)
	_, err = f.module.ResolvePickupExtension(f.ctx, f.taskID, []int64{f.templateID, f.secondTemplate})
	require.ErrorIs(t, err, timetable.ErrPickupExtensionBlockGone)
	f.assertNothingWritten(t)
}

// A day extension keeps lock and insert per block: a failing insert of the
// first block ends the resolve with that failure before the second block,
// which another transaction is cancelling, is locked.
func TestPickupDayExtensionInsertFailurePrecedesLaterBlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "pickup-day-chain")
	suffix := time.Now().UnixNano()
	child := testpkg.CreateTestStudent(t, db, "Mia", fmt.Sprintf("DayChain-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Ben", fmt.Sprintf("DayChain-%d", suffix), "1a")
	staff := testpkg.CreateTestStaff(t, db, "Swantje", fmt.Sprintf("DayChain-%d", suffix))
	const date = "2099-03-03"
	exception := testpkg.CreateTestPickupException(t, db, child.ID, testpkg.Date(2099, time.March, 3), staff.ID, "16:00", "")
	first := pickupExtensionInstance(t, module, ctx, fixture, date, "14:45:00", "16:00:00", "Freies Spiel")
	second := pickupExtensionInstance(t, module, ctx, fixture, date, "15:00:00", "16:00:00", "Werkstatt")
	createOwnedInstanceStudent(t, module, ctx, first.ID, other.ID)
	createOwnedInstanceStudent(t, module, ctx, second.ID, other.ID)
	require.NoError(t, module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{
		StudentID: child.ID, PickupExceptionID: exception.ID, Date: date, PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	createFailTrigger(t, db, "pickup_day_chain_insert", "schedule.instance_students", "BEFORE INSERT",
		fmt.Sprintf("NEW.student_id = %d AND NEW.instance_id = %d", child.ID, first.ID), "RAISE EXCEPTION 'injected day insert failure';")
	f := &pickupChainFixture{pickupLockFixture: &pickupLockFixture{db: db, module: module, ctx: ctx, childID: child.ID, taskID: task.ID}}
	canceller := f.holdWith(t, `UPDATE schedule.activity_instances SET status = 'cancelled' WHERE id = ?`, second.ID)

	outcome := f.awaitOutcome(t, f.resolveBlocksAsync(first.ID, second.ID))
	require.ErrorContains(t, outcome.err, "injected day insert failure", "the insert failure comes before the second block's refusal")
	require.NoError(t, canceller.Commit())
	roster, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{first.ID, second.ID}, StudentIDs: []int64{child.ID}})
	require.NoError(t, err)
	assert.Empty(t, roster)
	assert.Equal(t, 1, countPickupExtensionRows(t, db, child.ID))
}

// createFailTrigger installs a row trigger on a table of the isolated clone.
func createFailTrigger(t *testing.T, db *bun.DB, name, table, timing, condition, action string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), fmt.Sprintf(`
		CREATE FUNCTION public.%[1]s() RETURNS trigger AS $$
		BEGIN
			IF %[4]s THEN %[5]s END IF;
			RETURN COALESCE(NEW, OLD);
		END $$ LANGUAGE plpgsql;
		CREATE TRIGGER %[1]s %[3]s ON %[2]s FOR EACH ROW EXECUTE FUNCTION public.%[1]s();`, name, table, timing, condition, action))
	require.NoError(t, err)
}

// waitForLockWaiters waits until n backends of the clone wait for a lock.
func (f *pickupChainFixture) waitForLockWaiters(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(context.Background(), &waiting))
		if waiting >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fewer than %d backends ever waited for a lock", n)
}

// Pickup against the other multi-row writers of the same blocks (#6232):
// offboarding (one statement), the planned-block delete and the list-kind
// propagation (one statement over the ids each). The writers over ids run
// with a sequential scan over a heap whose order runs against the ids
// (moveSmallestBlocksToHeapEnd), so a statement that took its row locks in
// scan order would deadlock with ids in date order. Every writer runs inside an outer transaction, so a deadlock
// surfaces as 40P01 instead of a replay. Every writer locks blocks by
// ascending id, so with ids in date order, against it, and across two
// chosen templates the pair serialises without a deadlock. Before #6232 the
// resolve locked in planned (date, id) order per template and deadlocked
// once against offboarding and the planned delete.
func TestPickupChainCreatesNoNewDeadlockPairs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	against := []string{pickupChainTuesdays[2], pickupChainTuesdays[0], pickupChainTuesdays[1]}
	writers := map[string]func(t *testing.T, f *pickupChainFixture) func(context.Context) error{
		"offboarding": func(t *testing.T, f *pickupChainFixture) func(context.Context) error {
			for _, instance := range f.instances {
				createOwnedInstanceStaff(t, f.module, f.ctx, instance, f.staffID, true, false)
			}
			offboarding, err := NewOffboarding(OffboardingDependencies{DB: f.db, Sessions: &fixedSessions{}, Observe: func(Observation) {}})
			require.NoError(t, err)
			preview, err := offboarding.Preview(f.ctx, f.staffID, "2099-03-01")
			require.NoError(t, err)
			return func(ctx context.Context) error {
				_, err := offboarding.Execute(ctx, f.staffID, "2099-03-01", preview.Revision)
				return err
			}
		},
		"planned delete": func(t *testing.T, f *pickupChainFixture) func(context.Context) error {
			f.moveSmallestBlocksToHeapEnd(t)
			return func(ctx context.Context) error {
				if err := scanSequentially(ctx); err != nil {
					return err
				}
				_, err := f.module.DeletePlannedActivityInstances(ctx, "2099-03-01", nil, &f.templateID, false)
				return err
			}
		},
		"list kind": func(t *testing.T, f *pickupChainFixture) func(context.Context) error {
			f.moveSmallestBlocksToHeapEnd(t)
			kind := timetable.ListKindMensa
			return func(ctx context.Context) error {
				if err := scanSequentially(ctx); err != nil {
					return err
				}
				_, err := f.module.PropagateActivityInstanceListKind(ctx, f.templateID, nil, &kind, "2099-03-01")
				return err
			}
		},
	}
	type pair struct {
		name     string
		writer   string
		creation []string
	}
	pairs := []pair{
		{name: "offboarding, ids in date order", writer: "offboarding"},
		{name: "offboarding, ids against date order", writer: "offboarding", creation: against},
		{name: "planned delete, ids in date order", writer: "planned delete"},
		{name: "planned delete, ids against date order", writer: "planned delete", creation: against},
		{name: "list kind, ids in date order", writer: "list kind"},
		{name: "list kind, ids against date order", writer: "list kind", creation: against},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			f := newPickupChainFixture(t, db, pickupChainOptions{creation: p.creation})
			write := writers[p.writer](t, f)
			holder := f.holdInstance(t, f.instances[1], "")
			resolve := func() <-chan error {
				done := make(chan error, 1)
				go func() {
					done <- testpkg.TenantCommand0(t, f.ctx, f.db, func(txCtx context.Context) error {
						_, err := f.module.ResolvePickupExtension(txCtx, f.taskID, []int64{f.templateID})
						return err
					})
				}()
				return done
			}
			writeAsync := func() <-chan error {
				done := make(chan error, 1)
				go func() { done <- testpkg.TenantCommand0(t, f.ctx, f.db, write) }()
				return done
			}
			resolved := resolve()
			f.waitForLockWaiters(t, 1)
			written := writeAsync()
			f.waitForLockWaiters(t, 2)
			require.NoError(t, holder.Rollback())

			for range 2 {
				select {
				case err := <-resolved:
					assert.NotEqual(t, "40P01", sqlState(err), "the resolve is no deadlock victim: %v", err)
					require.NoError(t, err)
				case err := <-written:
					assert.NotEqual(t, "40P01", sqlState(err), "the %s is no deadlock victim: %v", p.writer, err)
					require.NoError(t, err)
				case <-time.After(20 * time.Second):
					t.Fatal("a writer never finished")
				}
			}
		})
	}
}

// moveSmallestBlocksToHeapEnd gives the two blocks with the smallest ids new
// row versions at the end of the heap, in id order: a sequential scan visits
// the largest id first, then the smallest. While the resolve holds the
// smallest and waits for the middle one, a statement that locked in scan
// order would hold the largest and wait for the smallest: a cycle once the
// resolve reaches the largest.
func (f *pickupChainFixture) moveSmallestBlocksToHeapEnd(t *testing.T) {
	t.Helper()
	ids := slices.Sorted(slices.Values(f.instances))
	for _, id := range ids[:2] {
		_, err := f.db.ExecContext(context.Background(), `UPDATE schedule.activity_instances SET title = title || ' ' WHERE id = ?`, id)
		require.NoError(t, err)
	}
}

// scanSequentially makes the ambient transaction scan tables in heap order.
func scanSequentially(ctx context.Context) error {
	value, ok := testpkg.TransactionFromContext(ctx)
	if !ok {
		return errors.New("no ambient transaction")
	}
	idb, ok := value.(bun.IDB)
	if !ok {
		return fmt.Errorf("transaction %T", value)
	}
	_, err := idb.ExecContext(ctx, `SET LOCAL enable_indexscan = off; SET LOCAL enable_bitmapscan = off; SET LOCAL enable_indexonlyscan = off`)
	return err
}

// Two chosen templates (#6232): the first template gained a block after the
// second template's blocks were planned, so its ids are not contiguous. The
// resolve locks the blocks of both templates in one ascending pass before it
// writes, so offboarding the second template's staff (ascending ids) waits
// instead of closing a cycle. Before #6232 the resolve locked and wrote one
// template after the other and deadlocked here once.
func TestPickupChainLocksBlocksAcrossTemplatesByAscendingID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newPickupChainFixture(t, db, pickupChainOptions{second: true})
	late := pickupExtensionInstance(t, f.module, f.ctx, ownedActivityInstanceFixture{roomID: f.roomOf(t, f.instances[0]), groupID: f.templateID},
		pickupChainTuesdays[3], "14:45:00", "16:00:00", "Freies Spiel")
	require.Greater(t, late.ID, slices.Max(f.second), "the late block of the first template has the largest id")
	for _, instance := range append(slices.Clone(f.second), late.ID) {
		createOwnedInstanceStaff(t, f.module, f.ctx, instance, f.staffID, true, false)
	}
	offboarding, err := NewOffboarding(OffboardingDependencies{DB: f.db, Sessions: &fixedSessions{}, Observe: func(Observation) {}})
	require.NoError(t, err)
	preview, err := offboarding.Preview(f.ctx, f.staffID, "2099-03-01")
	require.NoError(t, err)

	holder := f.holdInstance(t, late.ID, "")
	resolved := make(chan error, 1)
	go func() {
		resolved <- testpkg.TenantCommand0(t, f.ctx, f.db, func(txCtx context.Context) error {
			_, err := f.module.ResolvePickupExtension(txCtx, f.taskID, []int64{f.templateID, f.secondTemplate})
			return err
		})
	}()
	f.waitForLockWaiters(t, 1)
	written := make(chan error, 1)
	go func() {
		written <- testpkg.TenantCommand0(t, f.ctx, f.db, func(ctx context.Context) error {
			_, err := offboarding.Execute(ctx, f.staffID, "2099-03-01", preview.Revision)
			return err
		})
	}()
	f.waitForLockWaiters(t, 2)
	require.NoError(t, holder.Rollback())
	for range 2 {
		select {
		case err := <-resolved:
			assert.NotEqual(t, "40P01", sqlState(err), "the resolve is no deadlock victim: %v", err)
			require.NoError(t, err)
		case err := <-written:
			assert.NotEqual(t, "40P01", sqlState(err), "offboarding is no deadlock victim: %v", err)
			require.NoError(t, err)
		case <-time.After(20 * time.Second):
			t.Fatal("a writer never finished")
		}
	}
}

// roomOf reads the planned room of a block.
func (f *pickupChainFixture) roomOf(t *testing.T, instanceID int64) int64 {
	t.Helper()
	var roomID int64
	require.NoError(t, f.db.NewRaw(`SELECT room_id FROM schedule.activity_instances WHERE id = ?`, instanceID).Scan(context.Background(), &roomID))
	return roomID
}

// The driver gives up a read that waits longer than its read timeout
// (pgdriver: 10 s by default, not overridden in production). PostgreSQL sends
// no row while a statement waits for a lock, so one statement that waits for
// several blocks in turn adds their waits up within a single read (#5816
// review). With one statement per block each read waits for one block only:
// two blocks held 1.2 s each under a 2 s read timeout still refuse the
// changed third block with the block-gone conflict, not with a driver error.
// Each hold starts once the resolve is seen waiting for that block, so the
// waits add up to more than the read timeout.
func TestPickupResolveWaitsForEachBlockWithinOneRead(t *testing.T) {
	t.Parallel()
	const readTimeout, hold = 2 * time.Second, 1200 * time.Millisecond
	testpkg.SetupIsolatedTestDB(t)
	pool := privateClonePool(t, map[string]string{"read_timeout": readTimeout.String()})
	f := newPickupChainFixture(t, pool, pickupChainOptions{})
	f.ctx = testpkg.WithTenantRuntime(t, f.ctx, pool)
	// The third block changes while the resolve waits: a block whose
	// committed state no longer fits refuses before any wait (#6232).
	changer := f.holdWith(t, `UPDATE schedule.activity_instances SET start_time = '16:00', end_time = '16:30' WHERE id = ?`, f.instances[2])

	first := f.holdInstance(t, f.instances[0], "")
	second := f.holdInstance(t, f.instances[1], "")
	done := f.resolveBlocksAsync(f.templateID)
	f.waitForInstanceLockWait(t)
	waitStarted := time.Now()
	// The hold times are the subject of the test.
	time.Sleep(hold)
	require.NoError(t, first.Rollback())
	deadline := time.Now().Add(15 * time.Second)
	for !f.instanceLockedElsewhere(t, f.instances[0]) {
		require.True(t, time.Now().Before(deadline), "the resolve never locked the first block")
		time.Sleep(20 * time.Millisecond)
	}
	f.waitForInstanceLockWait(t)
	time.Sleep(hold)
	require.NoError(t, second.Rollback())
	for !f.instanceLockedElsewhere(t, f.instances[1]) {
		require.True(t, time.Now().Before(deadline), "the resolve never locked the second block")
		time.Sleep(20 * time.Millisecond)
	}
	f.waitForInstanceLockWait(t)
	require.NoError(t, changer.Commit())

	outcome := f.awaitOutcome(t, done)
	assert.Greater(t, time.Since(waitStarted), readTimeout, "the waits together exceed the read timeout")
	require.ErrorIs(t, outcome.err, timetable.ErrPickupExtensionBlockGone, "err=%v", outcome.err)
	f.assertNothingWritten(t)
}
