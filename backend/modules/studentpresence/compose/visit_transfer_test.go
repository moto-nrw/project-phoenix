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

func TestVisitFacadeTransferAndBulkCloseRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Transfer", "Visit", "3a")
	activity := testpkg.CreateTestActivityGroup(t, db, "Visit transfer")
	room := testpkg.CreateTestRoom(t, db, "Visit transfer")
	from := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	to := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	// A host clock ahead of PostgreSQL must not abort the bulk checkout.
	input := studentpresence.Visit{StudentID: student.ID, ActiveGroupID: from.ID, EntryTime: time.Now().Add(time.Hour)}
	visit, err := module.RecordVisit(ctx, input)
	require.NoError(t, err)
	locations, err := module.ListVisitLocations(ctx, studentpresence.VisitLocationFilter{VisitFilter: studentpresence.VisitFilter{StudentIDs: []int64{student.ID}, OpenOnly: true}, RunningGroupsOnly: true, LatestPerStudent: true})
	require.NoError(t, err)
	require.Len(t, locations, 1)
	require.NotNil(t, locations[0].Group)
	assert.Equal(t, from.ID, locations[0].Group.ID)
	assert.Equal(t, room.ID, locations[0].Group.RoomID)
	assert.Equal(t, visit.ID, locations[0].Visit.ID)
	occupancy, err := module.CountOpenVisitsInRoom(ctx, room.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, occupancy)
	abort := errors.New("abort visit maintenance")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		count, err := module.TransferOpenVisits(txCtx, from.ID, to.ID)
		if err != nil {
			return err
		}
		require.EqualValues(t, 1, count)
		return abort
	})
	require.ErrorIs(t, err, abort)
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Equal(t, from.ID, stored.ActiveGroupID)
	count, err := module.TransferOpenVisits(ctx, from.ID, to.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	count, err = module.TransferOpenVisits(ctx, from.ID, to.ID)
	require.NoError(t, err)
	assert.Zero(t, count)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		count, err := module.CloseGroupVisits(txCtx, []int64{to.ID})
		if err != nil {
			return err
		}
		require.EqualValues(t, 1, count)
		return abort
	})
	require.ErrorIs(t, err, abort)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime)
	count, err = module.CloseGroupVisits(ctx, []int64{to.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	count, err = module.CloseGroupVisits(ctx, []int64{to.ID})
	require.NoError(t, err)
	assert.Zero(t, count)
	count, err = module.TransferOpenVisits(ctx, to.ID, from.ID)
	require.NoError(t, err)
	assert.Zero(t, count, "a stale transfer cannot reopen a completed visit")
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExitTime)
	assert.True(t, stored.EntryTime.Equal(*stored.ExitTime), "clock skew is clamped to a zero-duration interval")
	assert.Equal(t, to.ID, stored.ActiveGroupID)
}

func TestVisitFacadeKeepsHostingSchoolBoundaryForCrossSchoolStudents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, db, "Hosting school")
	room := testpkg.CreateTestRoom(t, db, "Hosting school")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	var foreignStudentID, foreignGroupID int64
	var foreignCtx context.Context
	t.Run("home school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		student := testpkg.CreateTestStudent(t, db, "Holiday", "Visitor", "3a")
		activity := testpkg.CreateTestActivityGroup(t, db, "Home school")
		room := testpkg.CreateTestRoom(t, db, "Home school")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		foreignStudentID, foreignGroupID = student.ID, group.ID
	})
	// Eligibility is checked by the holiday workflow. The recorded location
	// belongs to the hosting school, not the student's home-school tenant.
	visit, err := module.RecordVisit(ctx, studentpresence.Visit{StudentID: foreignStudentID, ActiveGroupID: group.ID, EntryTime: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, tenant.FromContext(ctx), visit.TenantID)
	_, err = module.FindVisit(foreignCtx, visit.ID)
	require.ErrorIs(t, err, studentpresence.ErrVisitNotFound)
	_, err = module.TransferOpenVisits(ctx, group.ID, foreignGroupID)
	require.Error(t, err, "the group must belong to the hosting tenant")
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Equal(t, group.ID, stored.ActiveGroupID)
	assert.Nil(t, stored.ExitTime)
}
