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

// The visit revision of a room orders the kiosk's room-wide counter (#5205,
// ADR 0133). It is the sum of the change events stamped with the room that a
// read sees. These tests pin its invariants:
//
//  1. Scope: one room of one school; it never mixes with a session revision.
//  2. Monotone: every committed change of the room's count raises it, also
//     a session's end, reopening, deletion and room change; nothing lowers
//     it, also not the nightly compaction or a deleted session.
//  3. Older never wins: a read that started before a write committed
//     carries a lower revision than the writer's own answer.
//  4. Concurrent writers: overlapping writers of one room can answer the
//     same revision; the next read after both commits is higher. The one
//     further blur: a visit write overlapping its session's room change
//     raises the room it saw, so the new room keeps its revision for the
//     changed count until its next change.
//  5. No lock edge: the triggers only insert; room changes and session ends
//     do not wait for visit writers, and room reads wait for nobody.
//  6. Tenant: another school's room reads count 0 and revision 0.

func readRoomSnapshot(t *testing.T, ctx context.Context, module studentpresence.Owner, roomID int64) countSnapshot {
	t.Helper()
	count, revision, err := module.CountOpenVisitsInRoomWithRevision(ctx, roomID)
	require.NoError(t, err)
	return countSnapshot{count: count, revision: revision}
}

func sessionInRoom(t *testing.T, db *bun.DB, roomID int64, name string) *testpkg.ActiveGroupRow {
	t.Helper()
	activity := testpkg.CreateTestActivityGroup(t, db, name+" activity")
	return testpkg.CreateTestActiveGroup(t, db, activity.ID, roomID)
}

func execSQL(t *testing.T, ctx context.Context, db bun.IDB, query string, args ...any) {
	t.Helper()
	_, err := db.NewRaw(query, args...).Exec(ctx)
	require.NoError(t, err)
}

func moveSessionToRoom(t *testing.T, ctx context.Context, module studentpresence.Owner, db *bun.DB, groupID, roomID int64) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		group, err := module.LockGroup(txCtx, groupID)
		if err != nil {
			return err
		}
		group.RoomID = roomID
		_, err = module.ReviseGroup(txCtx, group)
		return err
	}))
}

func TestRoomRevisionFollowsEveryRoomCountChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	room := testpkg.CreateTestRoom(t, db, "Room revision")
	elsewhere := testpkg.CreateTestRoom(t, db, "Room revision elsewhere")
	first := sessionInRoom(t, db, room.ID, "Room first")
	second := sessionInRoom(t, db, room.ID, "Room second")
	away := sessionInRoom(t, db, elsewhere.ID, "Room away")

	assert.Equal(t, countSnapshot{count: 0, revision: 0}, readRoomSnapshot(t, ctx, module, room.ID), "a room starts at revision 0")

	moving := openVisit(t, ctx, module, db, first.ID, "Moving")
	openVisit(t, ctx, module, db, second.ID, "Staying")
	assert.Equal(t, countSnapshot{count: 2, revision: 2}, readRoomSnapshot(t, ctx, module, room.ID), "both sessions raise their room")

	exit := time.Now().Add(-time.Minute)
	closedStudent := testpkg.CreateTestStudent(t, db, "Closed", "Room", "3a")
	_, err := module.RecordVisit(ctx, studentpresence.Visit{StudentID: closedStudent.ID, ActiveGroupID: first.ID, EntryTime: time.Now().Add(-time.Hour), ExitTime: &exit})
	require.NoError(t, err)
	execSQL(t, ctx, db, "UPDATE active.groups SET last_activity = NOW() WHERE id = ?", first.ID)
	assert.Equal(t, countSnapshot{count: 2, revision: 2}, readRoomSnapshot(t, ctx, module, room.ID), "a closed visit and a heartbeat keep the room")

	moved := moving
	moved.ActiveGroupID = away.ID
	_, err = module.ReviseVisit(ctx, moved)
	require.NoError(t, err)
	assert.Equal(t, countSnapshot{count: 1, revision: 3}, readRoomSnapshot(t, ctx, module, room.ID), "a visit moved out raises the room")
	assert.Equal(t, countSnapshot{count: 1, revision: 1}, readRoomSnapshot(t, ctx, module, elsewhere.ID), "and the room it moved into")

	sessionBefore := readSnapshot(t, ctx, module, second.ID)
	require.NoError(t, module.EndGroup(ctx, second.ID, time.Now()))
	assert.Equal(t, countSnapshot{count: 0, revision: 4}, readRoomSnapshot(t, ctx, module, room.ID), "ending a session with an open visit raises the room")
	assert.Equal(t, sessionBefore, readSnapshot(t, ctx, module, second.ID), "a session end without a visit change keeps the session revision")

	execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NULL WHERE id = ?", second.ID)
	assert.Equal(t, countSnapshot{count: 1, revision: 5}, readRoomSnapshot(t, ctx, module, room.ID), "reopening the session raises the room")

	moveSessionToRoom(t, ctx, module, db, second.ID, elsewhere.ID)
	assert.Equal(t, countSnapshot{count: 0, revision: 6}, readRoomSnapshot(t, ctx, module, room.ID), "a session leaving the room raises it")
	assert.Equal(t, countSnapshot{count: 2, revision: 2}, readRoomSnapshot(t, ctx, module, elsewhere.ID), "a session entering a room raises it")

	gone := sessionInRoom(t, db, room.ID, "Room deleted")
	openVisit(t, ctx, module, db, gone.ID, "Deleted")
	assert.Equal(t, countSnapshot{count: 1, revision: 7}, readRoomSnapshot(t, ctx, module, room.ID))
	require.NoError(t, module.DeleteGroup(ctx, gone.ID))
	assert.Equal(t, countSnapshot{count: 0, revision: 8}, readRoomSnapshot(t, ctx, module, room.ID),
		"deleting an open session with open visits raises the room once")

	ended := sessionInRoom(t, db, room.ID, "Room ended deleted")
	execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NOW() WHERE id = ?", ended.ID)
	afterEnd := readRoomSnapshot(t, ctx, module, room.ID)
	require.NoError(t, module.DeleteGroup(ctx, ended.ID))
	assert.Equal(t, afterEnd, readRoomSnapshot(t, ctx, module, room.ID), "deleting an ended session keeps the room")
}

func TestRoomRevisionStaysInsideTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	var foreignRoom int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx := testpkg.Ctx(t)
		foreignRoom = testpkg.CreateTestRoom(t, db, "Foreign room revision").ID
		group := sessionInRoom(t, db, foreignRoom, "Foreign room")
		openVisit(t, foreignCtx, module, db, group.ID, "Foreign")
		require.Equal(t, countSnapshot{count: 1, revision: 1}, readRoomSnapshot(t, foreignCtx, module, foreignRoom))
	})
	assert.Equal(t, countSnapshot{count: 0, revision: 0}, readRoomSnapshot(t, ctx, module, foreignRoom),
		"another school's room shows neither its count nor its revision")
	_, _, err := module.CountOpenVisitsInRoomWithRevision(ctx, 0)
	require.Error(t, err)
}

// TestRoomRevisionUnderConcurrentWriters runs the transaction races of the
// room revision one after another: each holds two or three transactions
// open at once, and in sequence they stay within the shared test pool.
func TestRoomRevisionUnderConcurrentWriters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := newRevisionModule(t, db)
	for _, race := range []struct {
		name string
		run  func(*testing.T, context.Context, *bun.DB, studentpresence.Owner)
	}{
		{"poll before the scan committed", roomPollBeforeTheScanCommitted},
		{"concurrent writers meet on the next read", roomWritersMeetOnTheNextRead},
		{"session lifecycle racing visit writes", roomLifecycleRacingVisitWrites},
		{"room change racing a visit write", roomChangeRacingAVisitWrite},
		{"room lifecycle takes no visit lock", roomLifecycleTakesNoVisitLock},
		{"nightly close keeps room revisions", roomNightlyCloseKeepsRevisions},
	} {
		t.Run(race.name, func(t *testing.T) { race.run(t, ctx, db, module) })
	}
}

// roomPollBeforeTheScanCommitted drives the race of
// #5205 on a room with two sessions: the scan books a child into one of
// them and answers before it commits; a later read of the room still sees
// the old count with a lower revision.
func roomPollBeforeTheScanCommitted(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner) {
	room := testpkg.CreateTestRoom(t, db, "Room race")
	first := sessionInRoom(t, db, room.ID, "Room race first")
	second := sessionInRoom(t, db, room.ID, "Room race second")
	openVisit(t, ctx, module, db, first.ID, "Present")
	arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Room", "3a")

	scan := startTx(t, ctx, db)
	var scanAnswer countSnapshot
	awaitStep(t, scan.run(func(txCtx context.Context) error {
		if _, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: second.ID, EntryTime: time.Now()}); err != nil {
			return err
		}
		scanAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
		return nil
	}))
	poll := readRoomSnapshot(t, ctx, module, room.ID)
	scan.commit(t)

	assert.Equal(t, 2, scanAnswer.count)
	assert.Equal(t, 1, poll.count, "the poll read before the scan committed")
	assert.Less(t, poll.revision, scanAnswer.revision, "the older count carries the lower revision")
	assert.Equal(t, scanAnswer, readRoomSnapshot(t, ctx, module, room.ID), "the same revision names the same count")
}

// roomWritersMeetOnTheNextRead: a check-in and a
// checkout in two sessions of one room overlap. Both answers may carry the
// same revision; the next read carries both changes above both.
func roomWritersMeetOnTheNextRead(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner) {
	room := testpkg.CreateTestRoom(t, db, "Room overlap")
	first := sessionInRoom(t, db, room.ID, "Room overlap first")
	second := sessionInRoom(t, db, room.ID, "Room overlap second")
	present := openVisit(t, ctx, module, db, first.ID, "Present")
	before := readRoomSnapshot(t, ctx, module, room.ID)
	arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Room overlap", "3a")

	checkin := startTx(t, ctx, db)
	checkout := startTx(t, ctx, db)
	var checkinAnswer, checkoutAnswer countSnapshot
	awaitStep(t, checkin.run(func(txCtx context.Context) error {
		if _, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: second.ID, EntryTime: time.Now()}); err != nil {
			return err
		}
		checkinAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
		return nil
	}))
	awaitStep(t, checkout.run(func(txCtx context.Context) error {
		if _, err := module.CloseVisits(txCtx, []int64{present.ID}, time.Now()); err != nil {
			return err
		}
		checkoutAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
		return nil
	}))
	checkout.commit(t)
	checkin.commit(t)

	assert.Equal(t, countSnapshot{count: 2, revision: before.revision + 1}, checkinAnswer)
	assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 1}, checkoutAnswer)
	assert.Equal(t, countSnapshot{count: 1, revision: before.revision + 2}, readRoomSnapshot(t, ctx, module, room.ID),
		"the next read carries both changes above both answers")
}

// roomLifecycleRacingVisitWrites: a session end and a
// reopening that overlap a visit write of the same session take no lock
// from each other, and the room's next read is above every answer.
func roomLifecycleRacingVisitWrites(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner) {

	t.Run("checkout during session end", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "Room end race")
		group := sessionInRoom(t, db, room.ID, "Room end race")
		leaving := openVisit(t, ctx, module, db, group.ID, "Leaving")
		openVisit(t, ctx, module, db, group.ID, "Staying")
		before := readRoomSnapshot(t, ctx, module, room.ID)

		checkout := startTx(t, ctx, db)
		sessionEnd := startTx(t, ctx, db)
		var checkoutAnswer, endAnswer countSnapshot
		awaitStep(t, checkout.run(func(txCtx context.Context) error {
			if _, err := module.CloseVisits(txCtx, []int64{leaving.ID}, time.Now()); err != nil {
				return err
			}
			checkoutAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
			return nil
		}))
		awaitStep(t, sessionEnd.run(func(txCtx context.Context) error {
			if err := module.EndGroup(txCtx, group.ID, time.Now()); err != nil {
				return err
			}
			endAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
			return nil
		}))
		sessionEnd.commit(t)
		checkout.commit(t)

		assert.Equal(t, countSnapshot{count: 1, revision: before.revision + 1}, checkoutAnswer)
		assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 1}, endAnswer)
		assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 2}, readRoomSnapshot(t, ctx, module, room.ID))
	})

	t.Run("check-in during reopening", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "Room reopen race")
		group := sessionInRoom(t, db, room.ID, "Room reopen race")
		execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NOW() WHERE id = ?", group.ID)
		before := readRoomSnapshot(t, ctx, module, room.ID)
		arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Room reopen", "3a")

		reopen := startTx(t, ctx, db)
		checkin := startTx(t, ctx, db)
		var reopenAnswer, checkinAnswer countSnapshot
		awaitStep(t, reopen.run(func(txCtx context.Context) error {
			if _, err := reopen.tx.NewRaw("UPDATE active.groups SET end_time = NULL WHERE id = ?", group.ID).Exec(txCtx); err != nil {
				return err
			}
			reopenAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
			return nil
		}))
		awaitStep(t, checkin.run(func(txCtx context.Context) error {
			if _, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: group.ID, EntryTime: time.Now()}); err != nil {
				return err
			}
			checkinAnswer = readRoomSnapshot(t, txCtx, module, room.ID)
			return nil
		}))
		checkin.commit(t)
		reopen.commit(t)

		assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 1}, reopenAnswer)
		assert.Equal(t, countSnapshot{count: 0, revision: before.revision + 1}, checkinAnswer, "the check-in saw the session ended and still raised its room")
		assert.Equal(t, countSnapshot{count: 1, revision: before.revision + 2}, readRoomSnapshot(t, ctx, module, room.ID))
	})
}

// roomChangeRacingAVisitWrite pins the one blur ADR
// 0133 accepts beyond overlapping writers: a check-in that runs before its
// session's room change commits raises the room it saw. The new room then
// names its count before and after the check-in with one revision until its
// next change; no revision falls.
func roomChangeRacingAVisitWrite(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner) {
	from := testpkg.CreateTestRoom(t, db, "Room blur from")
	to := testpkg.CreateTestRoom(t, db, "Room blur to")
	group := sessionInRoom(t, db, from.ID, "Room blur")
	openVisit(t, ctx, module, db, group.ID, "Present")
	fromBefore := readRoomSnapshot(t, ctx, module, from.ID)
	arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Room blur", "3a")

	checkin := startTx(t, ctx, db)
	move := startTx(t, ctx, db)
	awaitStep(t, checkin.run(func(txCtx context.Context) error {
		_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: group.ID, EntryTime: time.Now()})
		return err
	}))
	awaitStep(t, move.run(func(txCtx context.Context) error {
		_, err := move.tx.NewRaw("UPDATE active.groups SET room_id = ? WHERE id = ?", to.ID, group.ID).Exec(txCtx)
		return err
	}))
	move.commit(t)
	toBetween := readRoomSnapshot(t, ctx, module, to.ID)
	checkin.commit(t)
	toAfter := readRoomSnapshot(t, ctx, module, to.ID)

	assert.Equal(t, countSnapshot{count: 1, revision: 1}, toBetween)
	assert.Equal(t, countSnapshot{count: 2, revision: 1}, toAfter, "the blur: one revision of the new room names two counts")
	assert.Greater(t, readRoomSnapshot(t, ctx, module, from.ID).revision, fromBefore.revision, "the old room rose with the move and the check-in")
	openVisit(t, ctx, module, db, group.ID, "Later")
	assert.Equal(t, countSnapshot{count: 3, revision: 2}, readRoomSnapshot(t, ctx, module, to.ID), "the room's next change ends the blur")
}

// roomLifecycleTakesNoVisitLock pins that the room events add no lock
// edge: a room change, a session end and a reopening complete and commit
// while another transaction holds an uncommitted checkout and check-in of
// that session, and a room read completes while the session is locked FOR
// UPDATE.
func roomLifecycleTakesNoVisitLock(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner) {
	room := testpkg.CreateTestRoom(t, db, "Room no lock")
	other := testpkg.CreateTestRoom(t, db, "Room no lock other")
	group := sessionInRoom(t, db, room.ID, "Room no lock")
	closing := openVisit(t, ctx, module, db, group.ID, "Closing")
	arriving := testpkg.CreateTestStudent(t, db, "Arriving", "Room no lock", "3a")

	writer := startTx(t, ctx, db)
	awaitStep(t, writer.run(func(txCtx context.Context) error {
		if _, err := module.CloseVisits(txCtx, []int64{closing.ID}, time.Now()); err != nil {
			return err
		}
		_, err := module.RecordVisit(txCtx, studentpresence.Visit{StudentID: arriving.ID, ActiveGroupID: group.ID, EntryTime: time.Now()})
		return err
	}))
	lifecycle := startTx(t, ctx, db)
	awaitStep(t, lifecycle.run(func(txCtx context.Context) error {
		if _, err := lifecycle.tx.NewRaw("UPDATE active.groups SET room_id = ? WHERE id = ?", other.ID, group.ID).Exec(txCtx); err != nil {
			return err
		}
		if _, err := lifecycle.tx.NewRaw("UPDATE active.groups SET end_time = NOW() WHERE id = ?", group.ID).Exec(txCtx); err != nil {
			return err
		}
		_, err := lifecycle.tx.NewRaw("UPDATE active.groups SET end_time = NULL WHERE id = ?", group.ID).Exec(txCtx)
		return err
	}))
	lifecycle.commit(t)
	writer.commit(t)

	holder := startTx(t, ctx, db)
	awaitStep(t, holder.run(func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		return err
	}))
	readRoomSnapshot(t, ctx, module, other.ID)
	holder.commit(t)

	assert.Equal(t, 1, readRoomSnapshot(t, ctx, module, other.ID).count)
	assert.Equal(t, 0, readRoomSnapshot(t, ctx, module, room.ID).count)
}

// roomNightlyCloseKeepsRevisions pins the compaction for rooms: it
// folds the events of ended sessions, of deleted sessions and the room
// events of each room into one row per scope, keeps every room revision,
// and leaves a concurrent writer's event as its own row.
func roomNightlyCloseKeepsRevisions(t *testing.T, ctx context.Context, db *bun.DB, module studentpresence.Owner) {
	room := testpkg.CreateTestRoom(t, db, "Room compact")
	other := testpkg.CreateTestRoom(t, db, "Room compact other")
	ended := sessionInRoom(t, db, room.ID, "Room compact ended")
	deleted := sessionInRoom(t, db, room.ID, "Room compact deleted")
	running := sessionInRoom(t, db, room.ID, "Room compact running")
	moved := sessionInRoom(t, db, room.ID, "Room compact moved")
	stillOpen := openVisit(t, ctx, module, db, ended.ID, "Late")
	openVisit(t, ctx, module, db, ended.ID, "Kept")
	gone := openVisit(t, ctx, module, db, deleted.ID, "Gone")
	require.NoError(t, module.DeleteVisit(ctx, gone.ID))
	openVisit(t, ctx, module, db, running.ID, "Running")
	openVisit(t, ctx, module, db, moved.ID, "Moved")
	execSQL(t, ctx, db, "UPDATE active.groups SET end_time = NOW() WHERE id = ?", ended.ID)
	require.NoError(t, module.DeleteGroup(ctx, deleted.ID))
	moveSessionToRoom(t, ctx, module, db, moved.ID, other.ID)
	roomBefore := readRoomSnapshot(t, ctx, module, room.ID)
	otherBefore := readRoomSnapshot(t, ctx, module, other.ID)

	writer := startTx(t, ctx, db)
	awaitStep(t, writer.run(func(txCtx context.Context) error {
		_, err := module.CloseVisits(txCtx, []int64{stillOpen.ID}, time.Now())
		return err
	}))
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, err := module.EndGroupSessions(txCtx, []int64{running.ID, moved.ID}, time.Now())
		return err
	}))
	writer.commit(t)

	assert.Equal(t, countSnapshot{count: 0, revision: roomBefore.revision + 3}, readRoomSnapshot(t, ctx, module, room.ID),
		"the compaction keeps the room revision; closing and ending the running session and the concurrent checkout raised it")
	assert.Equal(t, countSnapshot{count: 0, revision: otherBefore.revision + 2}, readRoomSnapshot(t, ctx, module, other.ID),
		"closing and ending the moved session raised its new room")
	var rows []struct {
		Session *int64 `bun:"active_group_id"`
		Rows    int    `bun:"n"`
	}
	require.NoError(t, db.NewRaw(`SELECT active_group_id, count(*) AS n FROM active.visit_revision_events
		WHERE room_id = ? GROUP BY active_group_id`, room.ID).Scan(ctx, &rows))
	perSession := map[int64]int{}
	for _, row := range rows {
		key := int64(0)
		if row.Session != nil {
			key = *row.Session
		}
		perSession[key] = row.Rows
	}
	assert.Equal(t, map[int64]int{0: 1, ended.ID: 2, running.ID: 1, moved.ID: 1}, perSession,
		"one row per ended session and one room row; the deleted session's events joined the room row; the concurrent writer keeps its own")

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, err := module.EndGroupSessions(txCtx, nil, time.Now())
		return err
	}))
	assert.Equal(t, roomBefore.revision+3, readRoomSnapshot(t, ctx, module, room.ID).revision, "a second compaction changes no revision")
}
