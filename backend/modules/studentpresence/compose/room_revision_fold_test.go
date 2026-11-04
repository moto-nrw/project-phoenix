package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The nightly close folds every settled session into its rooms (#5273, ADR
// 0133): an ended session that no transaction is writing to leaves the set
// of sessions a room's revision reads, and its revision joins a room row.
// A room's read therefore covers the sessions since the last nightly close,
// not its whole history. These tests pin that the fold keeps every room
// revision and that a writer racing it can never lower one.

// heldSessions counts the sessions whose revision a room's read still sums.
func heldSessions(t *testing.T, db *bun.DB, roomID int64) int {
	t.Helper()
	var held int
	require.NoError(t, db.NewRaw(`SELECT count(DISTINCT entered_group_id) FROM active.visit_revision_events
		WHERE room_id = ? AND entered_group_id IS NOT NULL`, roomID).Scan(context.Background(), &held))
	return held
}

func nightlyClose(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner, groupIDs ...int64) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, err := module.EndGroupSessions(txCtx, groupIDs, time.Now())
		return err
	}))
}

func TestNightlyCloseFoldsSettledSessionsIntoTheirRooms(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	room := testpkg.CreateTestRoom(t, db, "Room fold")
	elsewhere := testpkg.CreateTestRoom(t, db, "Room fold elsewhere")
	reopened := sessionInRoom(t, db, room.ID, "Room fold reopened")
	late := sessionInRoom(t, db, room.ID, "Room fold late")
	deleted := sessionInRoom(t, db, room.ID, "Room fold deleted")
	moved := sessionInRoom(t, db, room.ID, "Room fold moved")
	running := sessionInRoom(t, db, room.ID, "Room fold running")
	for _, group := range []*testpkg.ActiveGroupRow{reopened, deleted, moved, running} {
		openVisit(t, ctx, module, db, group.ID, "Present")
	}
	lateVisit := openVisit(t, ctx, module, db, late.ID, "Late")
	moveSessionToRoom(t, ctx, module, db, moved.ID, elsewhere.ID)
	tracked := trackRoom(t, ctx, module, room.ID)
	other := trackRoom(t, ctx, module, elsewhere.ID)

	nightlyClose(t, ctx, db, module, reopened.ID, late.ID, deleted.ID, moved.ID)
	tracked.rose(1, "ending three of the room's sessions raises it")
	other.rose(0, "ending the moved session raises its new room")
	assert.Equal(t, 1, heldSessions(t, db, room.ID), "only the running session is still read; the ended ones joined the room row")
	assert.Zero(t, heldSessions(t, db, elsewhere.ID), "the moved session left both rooms it held")

	nightlyClose(t, ctx, db, module)
	tracked.kept("a second nightly close keeps the room")
	other.kept("and the other room")

	execSQL(t, ctx, db, "UPDATE active.visits SET exit_time = NULL WHERE id = ?", lateVisit.ID)
	tracked.kept("a late change of an ended session changes no room count and leaves the room")
	require.NoError(t, module.DeleteGroup(ctx, deleted.ID))
	tracked.kept("deleting a folded session keeps the room")
	nightlyClose(t, ctx, db, module)
	tracked.kept("folding a folded session's deletion keeps the room")

	execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NULL WHERE id = ?", late.ID)
	tracked.rose(2, "reopening a folded session brings its open visit back and raises the room")
	execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NULL WHERE id = ?", reopened.ID)
	tracked.rose(2, "reopening a folded session without an open visit raises the room")
	openVisit(t, ctx, module, db, reopened.ID, "Back")
	tracked.rose(3, "a check-in into a reopened session raises the room")
	nightlyClose(t, ctx, db, module, reopened.ID, late.ID, running.ID)
	tracked.rose(0, "ending the room's sessions again raises it")
	assert.Zero(t, heldSessions(t, db, room.ID), "the room reads no session once all of them ended")
	nightlyClose(t, ctx, db, module)
	tracked.kept("folding a session a second time keeps the room")
}

// TestRoomFoldUnderConcurrentWriters runs the writers that can still change
// an ended session against the fold. A writer that holds a session or one of
// its visits when the fold locks them keeps the session in its rooms until
// the next night; a writer that comes later waits for the nightly close.
func TestRoomFoldUnderConcurrentWriters(t *testing.T) {
	t.Parallel()
	db := lockScenarioDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	for _, race := range []struct {
		name  string
		write func(t *testing.T, group *testpkg.ActiveGroupRow, open studentpresence.Visit, closed studentpresence.Visit) func(context.Context) error
	}{
		{"late checkout", func(t *testing.T, _ *testpkg.ActiveGroupRow, open, _ studentpresence.Visit) func(context.Context) error {
			return func(txCtx context.Context) error {
				_, err := module.CloseVisits(txCtx, []int64{open.ID}, time.Now())
				return err
			}
		}},
		{"check-in into the ended session", func(t *testing.T, group *testpkg.ActiveGroupRow, _, _ studentpresence.Visit) func(context.Context) error {
			arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Room fold", "3a")
			return func(txCtx context.Context) error {
				_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: group.ID, EntryTime: time.Now()})
				return err
			}
		}},
		{"closed visit revised open", func(t *testing.T, _ *testpkg.ActiveGroupRow, _, closed studentpresence.Visit) func(context.Context) error {
			return func(txCtx context.Context) error {
				closed.ExitTime = nil
				_, err := module.ReviseVisit(txCtx, closed)
				return err
			}
		}},
	} {
		t.Run(race.name+" during the fold", func(t *testing.T) {
			room, group, open, closed := endedSessionWithVisits(t, ctx, db, module, race.name)
			before := readRoomSnapshot(t, ctx, module, room.ID)

			writer := startTx(t, ctx, db)
			awaitStep(t, writer.run(race.write(t, group, open, closed)))
			fold := startTx(t, ctx, db)
			awaitStep(t, fold.run(func(txCtx context.Context) error {
				_, err := module.EndGroupSessions(txCtx, nil, time.Now())
				return err
			}))
			writer.commit(t)
			between := readRoomSnapshot(t, ctx, module, room.ID)
			fold.commit(t)
			after := readRoomSnapshot(t, ctx, module, room.ID)

			assert.GreaterOrEqual(t, between.revision, before.revision)
			assert.GreaterOrEqual(t, after.revision, between.revision, "the fold never lowers a revision a read already saw")
			assert.Equal(t, 1, heldSessions(t, db, room.ID), "the session a writer held stays in its room")
			nightlyClose(t, ctx, db, module)
			assert.Zero(t, heldSessions(t, db, room.ID), "the next night folds it")
			assert.Equal(t, after, readRoomSnapshot(t, ctx, module, room.ID), "and keeps the room")
		})

		t.Run(race.name+" after the fold", func(t *testing.T) {
			room, group, open, closed := endedSessionWithVisits(t, ctx, db, module, race.name+" later")
			before := readRoomSnapshot(t, ctx, module, room.ID)

			fold := startTx(t, ctx, db)
			awaitStep(t, fold.run(func(txCtx context.Context) error {
				_, err := module.EndGroupSessions(txCtx, nil, time.Now())
				return err
			}))
			writer := startTx(t, ctx, db)
			written := writer.run(race.write(t, group, open, closed))
			awaitBlockedBy(t, db, writer, fold)
			fold.commit(t)
			awaitStep(t, written)
			writer.commit(t)

			assert.Zero(t, heldSessions(t, db, room.ID), "the fold took the session the writer had to wait for")
			assert.Equal(t, before, readRoomSnapshot(t, ctx, module, room.ID),
				"the write committed after the fold and changes no room count of an ended session")
		})
	}
}

// endedSessionWithVisits starts a session in a room of its own with an open
// and a closed visit and ends it without closing the open one.
func endedSessionWithVisits(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner, name string) (*testpkg.Room, *testpkg.ActiveGroupRow, studentpresence.Visit, studentpresence.Visit) {
	t.Helper()
	room := testpkg.CreateTestRoom(t, db, "Room fold "+name)
	group := sessionInRoom(t, db, room.ID, "Room fold "+name)
	open := openVisit(t, ctx, module, db, group.ID, "Open")
	student := testpkg.CreateTestStudent(t, db, "Closed", "Room fold", "3a")
	exit := time.Now().Add(-time.Minute)
	closed, err := module.RecordVisit(ctx, studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now().Add(-time.Hour), ExitTime: &exit})
	require.NoError(t, err)
	execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NOW() WHERE id = ?", group.ID)
	return room, group, open, closed
}
