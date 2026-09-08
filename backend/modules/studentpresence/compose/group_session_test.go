package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The session end closes the group's open visits and supervisions together
// with the group, leaves everything else alone, and stays inside the caller's
// transaction so a later failure rolls the whole close back.
func TestEndGroupSessionClosesOwnRowsInsideCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)

	activity := testpkg.CreateTestActivityGroup(t, db, "Session end")
	room := testpkg.CreateTestRoom(t, db, "Session end")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	other := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Session", "End")
	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	otherSupervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, other.ID, "supervisor")
	first := testpkg.CreateTestStudent(t, db, "Session", "First", "1a")
	second := testpkg.CreateTestStudent(t, db, "Session", "Second", "1a")
	entry := time.Now().Add(-time.Hour)
	openVisit := testpkg.CreateTestVisit(t, db, first.ID, group.ID, entry, nil)
	alreadyClosed := entry.Add(10 * time.Minute)
	closedVisit := testpkg.CreateTestVisit(t, db, second.ID, group.ID, entry, &alreadyClosed)
	otherVisit := testpkg.CreateTestVisit(t, db, second.ID, other.ID, entry, nil)
	at := time.Now().Truncate(time.Microsecond)

	_, err = module.EndGroupSession(ctx, group.ID, at)
	require.ErrorContains(t, err, "transaction is required")
	_, err = module.LockGroup(ctx, group.ID)
	require.ErrorContains(t, err, "transaction is required")

	abort := errors.New("abort after end")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		locked, err := module.LockGroup(txCtx, group.ID)
		require.NoError(t, err)
		assert.True(t, locked.IsOpen())
		assert.Equal(t, room.ID, locked.RoomID)
		require.NotNil(t, locked.ActivityGroupID)
		assert.Equal(t, activity.ID, *locked.ActivityGroupID)

		ended, err := module.EndGroupSession(txCtx, group.ID, at)
		require.NoError(t, err)
		require.Len(t, ended.ClosedVisits, 1, "only the open visit closes")
		assert.Equal(t, openVisit.ID, ended.ClosedVisits[0].ID)
		assert.Equal(t, first.ID, ended.ClosedVisits[0].StudentID)
		require.NotNil(t, ended.ClosedVisits[0].ExitTime)
		assert.WithinDuration(t, at, *ended.ClosedVisits[0].ExitTime, time.Millisecond)
		assert.Equal(t, []int64{supervisor.ID}, ended.EndedSupervisorIDs)

		_, err = module.EndGroupSession(txCtx, group.ID, at)
		require.ErrorIs(t, err, studentpresence.ErrGroupEnded, "the second close in the same transaction sees the ended group")
		return abort
	})
	require.ErrorIs(t, err, abort)

	stillOpen, err := module.ListVisits(ctx, studentpresence.VisitFilter{IDs: []int64{openVisit.ID}})
	require.NoError(t, err)
	require.Len(t, stillOpen, 1)
	assert.Nil(t, stillOpen[0].ExitTime, "the rolled-back close leaves the visit open")
	groupEnded, supervisorEnded := testpkg.ActiveGroupEnded(t, db, testpkg.EndedActiveGroup{GroupID: group.ID, SupervisorID: supervisor.ID})
	assert.False(t, groupEnded)
	assert.False(t, supervisorEnded)

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, at)
		return err
	}))
	groupEnded, supervisorEnded = testpkg.ActiveGroupEnded(t, db, testpkg.EndedActiveGroup{GroupID: group.ID, SupervisorID: supervisor.ID})
	assert.True(t, groupEnded)
	assert.True(t, supervisorEnded)
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{IDs: []int64{openVisit.ID, closedVisit.ID, otherVisit.ID}})
	require.NoError(t, err)
	exits := map[int64]*time.Time{}
	for _, visit := range visits {
		exits[visit.ID] = visit.ExitTime
	}
	require.NotNil(t, exits[openVisit.ID])
	assert.WithinDuration(t, at, *exits[openVisit.ID], time.Millisecond)
	require.NotNil(t, exits[closedVisit.ID])
	assert.WithinDuration(t, alreadyClosed, *exits[closedVisit.ID], time.Millisecond, "a visit closed earlier keeps its exit time")
	assert.Nil(t, exits[otherVisit.ID], "another group's visit stays open")
	otherEnded, otherSupervisorEnded := testpkg.ActiveGroupEnded(t, db, testpkg.EndedActiveGroup{GroupID: other.ID, SupervisorID: otherSupervisor.ID})
	assert.False(t, otherEnded)
	assert.False(t, otherSupervisorEnded)

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.EndGroupSession(txCtx, group.ID, at)
		return err
	})
	require.ErrorIs(t, err, studentpresence.ErrGroupEnded)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, group.ID)
		require.NoError(t, err)
		_, err = module.EndGroupSession(txCtx, 0, at)
		require.Error(t, err)
		_, err = module.EndGroupSession(txCtx, group.ID, time.Time{})
		return err
	})
	require.Error(t, err)
}

// Two tenants: the session end command never sees or changes the other
// school's group, visits, or supervisions, even with the same IDs in hand.
func TestEndGroupSessionRespectsTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)

	var foreignCtx context.Context
	var foreign testpkg.EndedActiveGroup
	var foreignVisitID int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		activity := testpkg.CreateTestActivityGroup(t, db, "Foreign session")
		room := testpkg.CreateTestRoom(t, db, "Foreign session")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Foreign", "Supervisor")
		student := testpkg.CreateTestStudent(t, db, "Foreign", "Student", "2b")
		foreign = testpkg.EndedActiveGroup{GroupID: group.ID, SupervisorID: testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor").ID}
		foreignVisitID = testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now().Add(-time.Hour), nil).ID
	})

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.LockGroup(txCtx, foreign.GroupID)
		require.ErrorIs(t, err, studentpresence.ErrGroupNotFound)
		_, err = module.EndGroupSession(txCtx, foreign.GroupID, time.Now())
		return err
	})
	require.ErrorIs(t, err, studentpresence.ErrGroupNotFound)
	groupEnded, supervisorEnded := testpkg.ActiveGroupEnded(t, db, foreign)
	assert.False(t, groupEnded)
	assert.False(t, supervisorEnded)

	require.NoError(t, tenant.WithinCurrentTenant(foreignCtx, func(txCtx context.Context) error {
		ended, err := module.EndGroupSession(txCtx, foreign.GroupID, time.Now())
		require.NoError(t, err)
		require.Len(t, ended.ClosedVisits, 1)
		assert.Equal(t, foreignVisitID, ended.ClosedVisits[0].ID)
		assert.Equal(t, []int64{foreign.SupervisorID}, ended.EndedSupervisorIDs)
		return nil
	}))
}

// The nightly bulk close ends only the still-open groups among the given IDs,
// with their open visits and supervisions, and reports what it changed. A
// retry finds nothing left to close and rewrites no departure.
func TestEndGroupSessionsClosesOpenGroupsOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)

	activity := testpkg.CreateTestActivityGroup(t, db, "Bulk end")
	room := testpkg.CreateTestRoom(t, db, "Bulk end")
	staff := testpkg.CreateTestStaff(t, db, "Bulk", "End")
	student := testpkg.CreateTestStudent(t, db, "Bulk", "Student", "1a")
	skewedStudent := testpkg.CreateTestStudent(t, db, "Bulk", "Skewed", "1a")
	untouchedStudent := testpkg.CreateTestStudent(t, db, "Bulk", "Untouched", "1a")
	first := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	second := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	untouched := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	firstSupervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, first.ID, "supervisor")
	secondSupervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, second.ID, "supervisor")
	untouchedSupervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, untouched.ID, "supervisor")
	entry := time.Now().Add(-time.Hour)
	firstVisit := testpkg.CreateTestVisit(t, db, student.ID, first.ID, entry, nil)
	// A visit whose entry is ahead of the close instant keeps a zero-length
	// interval instead of an exit before its entry.
	skewedEntry := time.Now().Add(time.Hour)
	skewedVisit := testpkg.CreateTestVisit(t, db, skewedStudent.ID, second.ID, skewedEntry, nil)
	untouchedVisit := testpkg.CreateTestVisit(t, db, untouchedStudent.ID, untouched.ID, entry, nil)
	at := time.Now().Truncate(time.Microsecond)

	var ended studentpresence.EndedGroupSessions
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		ended, err = module.EndGroupSessions(txCtx, []int64{first.ID, second.ID}, at)
		return err
	}))
	assert.EqualValues(t, 2, ended.VisitsClosed)
	assert.EqualValues(t, 2, ended.SessionsEnded)
	assert.EqualValues(t, 2, ended.SupervisorsEnded)
	assert.ElementsMatch(t, []int64{first.ID, second.ID}, ended.EndedActiveGroupIDs)

	for _, group := range []testpkg.EndedActiveGroup{
		{GroupID: first.ID, SupervisorID: firstSupervisor.ID},
		{GroupID: second.ID, SupervisorID: secondSupervisor.ID},
	} {
		groupEnded, supervisorEnded := testpkg.ActiveGroupEnded(t, db, group)
		assert.True(t, groupEnded)
		assert.True(t, supervisorEnded)
	}
	groupEnded, supervisorEnded := testpkg.ActiveGroupEnded(t, db, testpkg.EndedActiveGroup{GroupID: untouched.ID, SupervisorID: untouchedSupervisor.ID})
	assert.False(t, groupEnded, "a group outside the batch keeps running")
	assert.False(t, supervisorEnded)

	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{IDs: []int64{firstVisit.ID, skewedVisit.ID, untouchedVisit.ID}})
	require.NoError(t, err)
	exits := map[int64]*time.Time{}
	for _, visit := range visits {
		exits[visit.ID] = visit.ExitTime
	}
	require.NotNil(t, exits[firstVisit.ID])
	assert.WithinDuration(t, at, *exits[firstVisit.ID], time.Millisecond)
	require.NotNil(t, exits[skewedVisit.ID])
	assert.WithinDuration(t, skewedEntry, *exits[skewedVisit.ID], time.Millisecond, "the exit is clamped to the entry time")
	assert.Nil(t, exits[untouchedVisit.ID])

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		ended, err = module.EndGroupSessions(txCtx, []int64{first.ID, second.ID, untouched.ID}, time.Now())
		return err
	}))
	assert.EqualValues(t, 1, ended.SessionsEnded, "only the group still open is ended on the retry")
	assert.Equal(t, []int64{untouched.ID}, ended.EndedActiveGroupIDs)
	visits, err = module.ListVisits(ctx, studentpresence.VisitFilter{IDs: []int64{firstVisit.ID}})
	require.NoError(t, err)
	require.Len(t, visits, 1)
	assert.WithinDuration(t, at, *visits[0].ExitTime, time.Millisecond, "the retry does not rewrite the recorded departure")

	_, err = module.EndGroupSessions(ctx, []int64{first.ID}, at)
	require.ErrorContains(t, err, "transaction is required")
}
