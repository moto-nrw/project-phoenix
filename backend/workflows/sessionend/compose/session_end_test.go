package compose_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingCompletion stands in for the Timetable owner's bridged completion.
// It records the transaction it ran in and can fail on demand so the test can
// prove that the presence writes share the unit and roll back with it. The
// real bridge is exercised through the kiosk endpoint in api/iot/sessions.
type recordingCompletion struct {
	mu    sync.Mutex
	calls [][]int64
	inTx  []bool
	fail  error
}

func (c *recordingCompletion) CompleteActiveByActiveGroupIDs(ctx context.Context, ids []int64, _ time.Time) (int64, error) {
	c.mu.Lock()
	_, inTx := tenant.TransactionFromContext(ctx)
	c.calls = append(c.calls, ids)
	c.inTx = append(c.inTx, inTx)
	c.mu.Unlock()
	if c.fail != nil {
		return 0, c.fail
	}
	return 1, nil
}

type recordingWaker struct {
	mu    sync.Mutex
	wakes [][2]int64
}

func (w *recordingWaker) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wakes = append(w.wakes, [2]int64{tenantID, studentID})
}

type stack struct {
	presence   *studentpresence.Module
	timetable  timetable.Capability
	completion *recordingCompletion
	bc         *testpkg.RecordingBroadcaster
	waker      *recordingWaker
	observed   []compose.Observation
	command    sessionend.Command
	// open builds a running kiosk session in the test database; ended reads
	// the group and supervisor rows back. Both close over the package pool so
	// the test never names the persistence library.
	open  func(*testing.T, string) session
	ended func(testpkg.EndedActiveGroup) (bool, bool)
	// openGroup builds an unmirrored session with one supervisor.
	openGroup func(*testing.T, string) testpkg.EndedActiveGroup
}

func newStack(t *testing.T) *stack {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	people, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	rooms, err := facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:            db,
		DeletionLock:  func(context.Context) error { return nil },
		DeletionGuard: func(context.Context, int64) error { return nil },
		Observe:       func(facilitiesCompose.Observation) {},
	})
	require.NoError(t, err)
	s := &stack{
		presence:   presence,
		timetable:  timetabletest.New(t, db),
		completion: &recordingCompletion{},
		bc:         testpkg.NewRecordingBroadcaster(),
		waker:      &recordingWaker{},
	}
	s.open = func(t *testing.T, label string) session {
		t.Helper()
		activity := testpkg.CreateTestActivityGroup(t, db, "Session "+label)
		room := testpkg.CreateTestRoom(t, db, "Room "+label)
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Session", label)
		supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
		first := testpkg.CreateTestStudent(t, db, "First", label, "1a")
		second := testpkg.CreateTestStudent(t, db, "Second", label, "1a")
		entry := time.Now().Add(-time.Hour)
		visits := []int64{
			testpkg.CreateTestVisit(t, db, first.ID, group.ID, entry, nil).ID,
			testpkg.CreateTestVisit(t, db, second.ID, group.ID, entry, nil).ID,
		}
		activityID := activity.ID
		groupID := group.ID
		instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
			Status: "active", ActivityGroupID: &activityID, ActiveGroupID: &groupID, IsSpontaneous: true, Title: "Mirrored " + label,
		})
		return session{
			group:      testpkg.EndedActiveGroup{GroupID: group.ID, SupervisorID: supervisor.ID},
			roomID:     room.ID,
			roomName:   room.Name,
			activityID: activity.ID,
			instanceID: instance.ID,
			students:   []int64{first.ID, second.ID},
			visitIDs:   visits,
		}
	}
	s.ended = func(group testpkg.EndedActiveGroup) (bool, bool) {
		return testpkg.ActiveGroupEnded(t, db, group)
	}
	s.openGroup = func(t *testing.T, label string) testpkg.EndedActiveGroup {
		t.Helper()
		activity := testpkg.CreateTestActivityGroup(t, db, label)
		room := testpkg.CreateTestRoom(t, db, label)
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, label, "Staff")
		return testpkg.EndedActiveGroup{GroupID: group.ID, SupervisorID: testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor").ID}
	}
	s.command, err = compose.New(compose.Dependencies{
		Presence:    presence,
		Timetable:   s.timetable,
		Completion:  s.completion,
		Students:    people,
		Rooms:       rooms,
		Broadcaster: s.bc,
		Guardians:   s.waker,
		Observe:     func(o compose.Observation) { s.observed = append(s.observed, o) },
	})
	require.NoError(t, err)
	return s
}

type session struct {
	group      testpkg.EndedActiveGroup
	roomID     int64
	roomName   string
	activityID int64
	instanceID int64
	students   []int64
	visitIDs   []int64
}

func (s *stack) openVisits(t *testing.T, ctx context.Context, ids []int64) int {
	t.Helper()
	visits, err := s.presence.ListVisits(ctx, studentpresence.VisitFilter{IDs: ids})
	require.NoError(t, err)
	open := 0
	for _, visit := range visits {
		if visit.ExitTime == nil {
			open++
		}
	}
	return open
}

// The kiosk close runs in the request's tenant transaction. Both owners write
// in that unit, nothing is announced before the commit, and afterwards the
// announcements name exactly the children, the room, and the block.
func TestEndSessionJoinsTheCallerTransactionAndAnnouncesAfterCommit(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ctx := testpkg.Ctx(t)
	open := s.open(t, "join")

	var result sessionend.Result
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.command.EndSession(txCtx, open.group.GroupID)
		require.NoError(t, err)
		assert.Empty(t, s.bc.Calls(), "nothing is announced before the caller commits")
		assert.Empty(t, s.waker.wakes)
		return nil
	}))

	assert.Equal(t, open.group.GroupID, result.ActiveGroupID)
	assert.Equal(t, 2, result.StudentsCheckedOut)
	assert.Equal(t, 1, result.SupervisorsEnded)
	require.NotNil(t, result.MirroredInstanceID)
	assert.Equal(t, open.instanceID, *result.MirroredInstanceID)

	groupEnded, supervisorEnded := s.ended(open.group)
	assert.True(t, groupEnded)
	assert.True(t, supervisorEnded)
	assert.Zero(t, s.openVisits(t, ctx, open.visitIDs))
	require.Len(t, s.completion.calls, 1)
	assert.Equal(t, []int64{open.group.GroupID}, s.completion.calls[0])
	assert.True(t, s.completion.inTx[0], "the timetable completion runs in the same unit as the presence writes")

	tenantID := testpkg.Tenant(t)
	groupTopic := s.bc.GroupCallsForTopic(itoa(open.group.GroupID))
	require.Len(t, groupTopic, 2, "one bulk check-out and one activity end on the session topic")
	assert.Equal(t, realtime.EventBulkStudentCheckOut, groupTopic[0].Event.Type)
	assert.Equal(t, tenantID, groupTopic[0].TenantID)
	require.NotNil(t, groupTopic[0].Event.Data.StudentIDs)
	assert.ElementsMatch(t, []string{itoa(open.students[0]), itoa(open.students[1])}, *groupTopic[0].Event.Data.StudentIDs)
	assert.Nil(t, groupTopic[0].Event.Data.GroupIDs, "children without an education group scope nothing")
	assert.Equal(t, realtime.EventActivityEnd, groupTopic[1].Event.Type)
	require.NotNil(t, groupTopic[1].Event.Data.ActivityName)
	assert.Equal(t, "Session join", *groupTopic[1].Event.Data.ActivityName)
	require.NotNil(t, groupTopic[1].Event.Data.RoomName)
	assert.Equal(t, open.roomName, *groupTopic[1].Event.Data.RoomName)
	require.NotNil(t, groupTopic[1].Event.Data.RoomID)
	assert.Equal(t, itoa(open.roomID), *groupTopic[1].Event.Data.RoomID)

	tenantWide := s.bc.CallsByMethod("tenant")
	require.Len(t, tenantWide, 3, "two dashboard refreshes and the completed block")
	assert.Equal(t, realtime.EventDashboardCountsChanged, tenantWide[0].Event.Type)
	assert.Equal(t, "student_moved", *tenantWide[0].Event.Data.Reason)
	assert.Equal(t, realtime.EventDashboardCountsChanged, tenantWide[1].Event.Type)
	assert.Equal(t, "activity_ended", *tenantWide[1].Event.Data.Reason)
	assert.Equal(t, realtime.EventInstanceCompleted, tenantWide[2].Event.Type)
	require.NotNil(t, tenantWide[2].Event.Data.InstanceID)
	assert.Equal(t, itoa(open.instanceID), *tenantWide[2].Event.Data.InstanceID)
	assert.Equal(t, testpkg.TodayDate().String(), *tenantWide[2].Event.Data.InstanceDate)
	assert.Equal(t, "14:00:00", *tenantWide[2].Event.Data.InstanceStartTime)
	for _, call := range tenantWide {
		assert.Equal(t, tenantID, call.TenantID)
	}
	assert.ElementsMatch(t, [][2]int64{{tenantID, open.students[0]}, {tenantID, open.students[1]}}, s.waker.wakes)

	require.Len(t, s.observed, 1)
	assert.Equal(t, "end_session", s.observed[0].Operation)
	assert.NoError(t, s.observed[0].Err)
	assert.Equal(t, 2, s.observed[0].StudentsCheckedOut)
	assert.True(t, s.observed[0].InstanceCompleted)

	// A second close finds nothing open.
	_, err := s.command.EndSession(ctx, open.group.GroupID)
	require.ErrorIs(t, err, sessionend.ErrSessionAlreadyEnded)
	require.Len(t, s.observed, 2)
	assert.ErrorIs(t, s.observed[1].Err, sessionend.ErrSessionAlreadyEnded)
}

// Without an ambient transaction the command opens its own unit and still
// announces only after that unit committed.
func TestEndSessionOpensItsOwnUnitWhenNoneIsAmbient(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ctx := testpkg.Ctx(t)
	open := s.open(t, "own")

	result, err := s.command.EndSession(ctx, open.group.GroupID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsCheckedOut)
	groupEnded, _ := s.ended(open.group)
	assert.True(t, groupEnded)
	assert.True(t, s.bc.HasEventType(realtime.EventActivityEnd))
	assert.True(t, s.bc.HasEventType(realtime.EventInstanceCompleted))

	_, err = s.command.EndSession(context.Background(), open.group.GroupID)
	require.Error(t, err, "a missing tenant must not become an unscoped write")
}

// A failing timetable side rolls the presence side back: the session stays
// open, the visits stay open, and nobody hears about a close that did not
// happen. That is the reason the close is one unit and not two calls.
func TestEndSessionRollsBackPresenceWhenTheTimetableFails(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ctx := testpkg.Ctx(t)
	open := s.open(t, "rollback")
	s.completion.fail = errors.New("attendance finalization failed")

	_, err := s.command.EndSession(ctx, open.group.GroupID)
	require.ErrorIs(t, err, s.completion.fail)

	groupEnded, supervisorEnded := s.ended(open.group)
	assert.False(t, groupEnded, "the presence writes did not survive the failed timetable write")
	assert.False(t, supervisorEnded)
	assert.Equal(t, 2, s.openVisits(t, ctx, open.visitIDs))
	assert.Empty(t, s.bc.Calls())
	assert.Empty(t, s.waker.wakes)

	// The same holds when the caller owns the transaction and rolls it back
	// after a successful close.
	s.completion.fail = nil
	abort := errors.New("caller aborts")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := s.command.EndSession(txCtx, open.group.GroupID)
		require.NoError(t, err)
		return abort
	})
	require.ErrorIs(t, err, abort)
	groupEnded, _ = s.ended(open.group)
	assert.False(t, groupEnded)
	assert.Equal(t, 2, s.openVisits(t, ctx, open.visitIDs))
	assert.Empty(t, s.bc.Calls(), "the queued announcements died with the rollback")
}

// A session that was never mirrored into the timetable closes on the presence
// side only and does not announce a completed block.
func TestEndSessionWithoutMirroredInstanceClosesPresenceOnly(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ctx := testpkg.Ctx(t)
	group := s.openGroup(t, "Unmirrored")

	result, err := s.command.EndSession(ctx, group.GroupID)
	require.NoError(t, err)
	assert.Nil(t, result.MirroredInstanceID)
	assert.Zero(t, result.StudentsCheckedOut)
	assert.Equal(t, 1, result.SupervisorsEnded)
	assert.Empty(t, s.completion.calls, "no instance, no completion")
	groupEnded, supervisorEnded := s.ended(group)
	assert.True(t, groupEnded)
	assert.True(t, supervisorEnded)
	assert.False(t, s.bc.HasEventType(realtime.EventBulkStudentCheckOut), "no child was checked out")
	assert.False(t, s.bc.HasEventType(realtime.EventInstanceCompleted))
	assert.True(t, s.bc.HasEventType(realtime.EventActivityEnd))
}

// Two tenants: a school cannot end another school's session, and ending its
// own leaves the other school's rows in all five tables untouched.
func TestEndSessionRespectsTwoTenantRLS(t *testing.T) {
	t.Parallel()
	s := newStack(t)
	ctx := testpkg.Ctx(t)
	own := s.open(t, "own-tenant")

	var foreignCtx context.Context
	var foreign session
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		foreign = s.open(t, "foreign-tenant")
	})

	_, err := s.command.EndSession(ctx, foreign.group.GroupID)
	require.ErrorIs(t, err, sessionend.ErrSessionNotFound, "another school's session does not exist for this tenant")
	assert.Empty(t, s.completion.calls)
	assert.Empty(t, s.bc.Calls())

	result, err := s.command.EndSession(ctx, own.group.GroupID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsCheckedOut)

	foreignEnded, foreignSupervisorEnded := s.ended(foreign.group)
	assert.False(t, foreignEnded)
	assert.False(t, foreignSupervisorEnded)
	assert.Equal(t, 2, s.openVisits(t, foreignCtx, foreign.visitIDs))
	foreignInstances, err := s.timetable.ListActivityInstances(foreignCtx, timetable.ActivityInstanceFilter{IDs: []int64{foreign.instanceID}})
	require.NoError(t, err)
	require.Len(t, foreignInstances, 1)
	assert.Equal(t, "active", foreignInstances[0].Status)
	for _, call := range s.bc.Calls() {
		assert.Equal(t, testpkg.Tenant(t), call.TenantID, "announcements stay inside the closing tenant")
	}

	_, err = s.command.EndSession(foreignCtx, foreign.group.GroupID)
	require.NoError(t, err, "the other school closes its own session")
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
