package compose

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Lock invariants of a weekday pickup extension (#5816), per planned block of
// the chosen template, in planned (date) order:
//
//  1. The block's row is locked FOR UPDATE by its own statement before the
//     next block's row; a later block is not locked yet.
//  2. The row is checked again under its lock (planned, same date, not
//     spontaneous, inside the extra time, same template).
//  3. A block that fails the check refuses with ErrPickupExtensionBlockGone
//     (HTTP 409) at once: no later block is locked or waited for, and nothing
//     is written.
//  4. A block locked by another transaction is waited for; once released and
//     still planned, the resolve proceeds and assigns every block.
//
// The weekday enrollment changes only after all blocks are locked; the
// participants are written after it.

type pickupLockFixture struct {
	db         *bun.DB
	module     timetable.Owner
	ctx        context.Context
	childID    int64
	taskID     int64
	templateID int64
	instances  []int64
}

func newPickupLockFixture(t *testing.T) *pickupLockFixture {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup locks %d", suffix))
	other := testpkg.CreateTestStudent(t, db, "Ida", fmt.Sprintf("Locks-%d", suffix), "1a")
	child := testpkg.CreateTestStudent(t, db, "Emil", fmt.Sprintf("Locks-%d", suffix), "1a")
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Pickup locks room %d", suffix))
	freePlay := pickupExtensionTemplate(t, module, ctx, category.ID, "Freies Spiel", "14:45:00", "16:00:00", timetable.WeekdayTuesday)
	_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: other.ID, ActivityGroupID: freePlay.ID, ValidFrom: "2020-01-01",
	})
	require.NoError(t, err)
	fixture := ownedActivityInstanceFixture{roomID: room.ID, groupID: freePlay.ID}
	var instances []int64
	for _, date := range []string{"2099-03-10", "2099-03-17", "2099-03-24"} {
		instances = append(instances, pickupExtensionInstance(t, module, ctx, fixture, date, "14:45:00", "16:00:00", "Freies Spiel").ID)
	}
	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	return &pickupLockFixture{db: db, module: module, ctx: ctx, childID: child.ID, taskID: task.ID, templateID: freePlay.ID, instances: instances}
}

// resolveAsync runs the resolve in its own transaction and reports its result.
func (f *pickupLockFixture) resolveAsync() <-chan pickupResolveOutcome {
	done := make(chan pickupResolveOutcome, 1)
	go func() {
		resolved, err := f.module.ResolvePickupExtension(f.ctx, f.taskID, []int64{f.templateID})
		done <- pickupResolveOutcome{resolved: resolved, err: err}
	}()
	return done
}

type pickupResolveOutcome struct {
	resolved timetable.PickupExtensionResolution
	err      error
}

// holdInstance opens a transaction that locks one instance row, optionally
// after changing it; the caller commits or rolls it back.
func (f *pickupLockFixture) holdInstance(t *testing.T, instanceID int64, update string) bun.Tx {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), &sql.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	if update != "" {
		_, err = tx.ExecContext(context.Background(), `UPDATE schedule.activity_instances SET `+update+` WHERE id = ?`, instanceID)
	} else {
		_, err = tx.ExecContext(context.Background(), `SELECT id FROM schedule.activity_instances WHERE id = ? FOR UPDATE`, instanceID)
	}
	require.NoError(t, err)
	return tx
}

// waitForInstanceLockWait waits until a backend waits for an instance row
// lock. The holders sit idle in their transactions; pg_stat_activity cuts long
// statement texts, so the match does not rely on the FOR UPDATE suffix.
func (f *pickupLockFixture) waitForInstanceLockWait(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
				AND query ILIKE '%activity_instances%'`).Scan(context.Background(), &waiting))
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the resolve never waited for an instance lock")
}

// instanceLockedElsewhere probes an instance row with NOWAIT from a fresh
// transaction: true when another transaction holds its lock.
func (f *pickupLockFixture) instanceLockedElsewhere(t *testing.T, instanceID int64) bool {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(context.Background(), `SELECT id FROM schedule.activity_instances WHERE id = ? FOR UPDATE NOWAIT`, instanceID)
	return err != nil
}

func (f *pickupLockFixture) awaitOutcome(t *testing.T, done <-chan pickupResolveOutcome) pickupResolveOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(15 * time.Second):
		t.Fatal("the resolve did not finish while a later block stayed locked")
		return pickupResolveOutcome{}
	}
}

// TestPickupWeekdayExtensionRefusesCancelledBlockWithoutLaterLocks is the
// review scenario: the first block is cancelled after the planning read and
// the second is locked by another transaction. The resolve refuses with the
// block-gone conflict without locking or waiting for the second block.
func TestPickupWeekdayExtensionRefusesCancelledBlockWithoutLaterLocks(t *testing.T) {
	t.Parallel()
	f := newPickupLockFixture(t)
	canceller := f.holdInstance(t, f.instances[0], `status = 'cancelled'`)
	holder := f.holdInstance(t, f.instances[1], "")

	done := f.resolveAsync()
	f.waitForInstanceLockWait(t)
	assert.False(t, f.instanceLockedElsewhere(t, f.instances[2]), "the third block is not locked while the first is awaited")
	require.NoError(t, canceller.Commit())

	outcome := f.awaitOutcome(t, done)
	require.ErrorIs(t, outcome.err, timetable.ErrPickupExtensionBlockGone)
	assert.True(t, f.instanceLockedElsewhere(t, f.instances[1]), "the second block is still held by the other transaction only")
	require.NoError(t, holder.Rollback())

	roster, err := f.module.ListInstanceStudents(f.ctx, timetable.InstanceStudentFilter{InstanceIDs: f.instances, StudentIDs: []int64{f.childID}})
	require.NoError(t, err)
	assert.Empty(t, roster, "the refusal writes no participant")
	assert.Equal(t, 1, countPickupExtensionRows(t, f.db, f.childID), "the task stays open")
}

// TestPickupWeekdayExtensionLocksBlocksInPlannedOrder holds the second block:
// the resolve holds the first block's lock, has not locked the third, waits,
// and assigns every block once the second is released.
func TestPickupWeekdayExtensionLocksBlocksInPlannedOrder(t *testing.T) {
	t.Parallel()
	f := newPickupLockFixture(t)
	holder := f.holdInstance(t, f.instances[1], "")

	done := f.resolveAsync()
	f.waitForInstanceLockWait(t)
	assert.True(t, f.instanceLockedElsewhere(t, f.instances[0]), "the first block is locked before the second is awaited")
	assert.False(t, f.instanceLockedElsewhere(t, f.instances[2]), "the third block is not locked before the second")
	require.NoError(t, holder.Rollback())

	outcome := f.awaitOutcome(t, done)
	require.NoError(t, outcome.err)
	assert.Equal(t, f.instances, outcome.resolved.InstanceIDs, "every block, in planned order")
	roster, err := f.module.ListInstanceStudents(f.ctx, timetable.InstanceStudentFilter{
		InstanceIDs: f.instances, StudentIDs: []int64{f.childID}, OrderByCreated: true,
	})
	require.NoError(t, err)
	require.Len(t, roster, len(f.instances))
	for i, row := range roster {
		assert.Equal(t, f.instances[i], row.InstanceID, "participants are written in planned order")
	}
	assert.Zero(t, countPickupExtensionRows(t, f.db, f.childID))
}
