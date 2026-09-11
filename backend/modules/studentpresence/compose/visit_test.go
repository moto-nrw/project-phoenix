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

func TestVisitFacadeRollbackRetryAndCompletedHistory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Visit", "Owner", "3a")
	activity := testpkg.CreateTestActivityGroup(t, db, "Visit owner")
	room := testpkg.CreateTestRoom(t, db, "Visit owner")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	at := time.Now().Add(-time.Minute)
	input := studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: at}
	abort := errors.New("abort after visit insert")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.RecordVisit(txCtx, input)
		if err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	rows, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, rows)
	first, err := module.RecordVisit(ctx, input)
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	_, err = module.RecordVisit(ctx, input)
	require.Error(t, err, "the open-visit uniqueness constraint must remain authoritative")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		ended, err := module.CloseVisits(txCtx, []int64{first.ID}, at.Add(time.Minute))
		if err != nil {
			return err
		}
		require.Len(t, ended, 1)
		return abort
	})
	require.ErrorIs(t, err, abort)
	stored, err := module.FindVisit(ctx, first.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime)
	ended, err := module.CloseVisits(ctx, []int64{first.ID}, at.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, ended, 1)
	ended, err = module.CloseVisits(ctx, []int64{first.ID}, at.Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, ended)
	input.EntryTime = at.Add(2 * time.Minute)
	second, err := module.RecordVisit(ctx, input)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID)
	rows, err = module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}, NewestFirst: true})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, second.ID, rows[0].ID)
	assert.NotNil(t, rows[1].ExitTime)
}

func TestVisitFacadeCannotReadLockOrModifyAnotherTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	var foreignCtx context.Context
	var foreign studentpresence.Visit
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		student := testpkg.CreateTestStudent(t, db, "Visit", "Foreign", "3b")
		activity := testpkg.CreateTestActivityGroup(t, db, "Foreign visit")
		room := testpkg.CreateTestRoom(t, db, "Foreign visit")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		foreign, err = module.RecordVisit(foreignCtx, studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now().Add(-time.Hour)})
		require.NoError(t, err)
	})
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.FindVisit(txCtx, foreign.ID)
		require.ErrorIs(t, err, studentpresence.ErrVisitNotFound)
		rows, err := module.ListVisits(txCtx, studentpresence.VisitFilter{IDs: []int64{foreign.ID}, ForUpdate: true})
		require.NoError(t, err)
		assert.Empty(t, rows)
		locations, err := module.ListVisitLocations(txCtx, studentpresence.VisitLocationFilter{VisitFilter: studentpresence.VisitFilter{IDs: []int64{foreign.ID}}})
		require.NoError(t, err)
		assert.Empty(t, locations)
		count, err := module.CountOpenVisitsInGroup(txCtx, foreign.ActiveGroupID)
		require.NoError(t, err)
		assert.Zero(t, count)
		rooms, err := module.ListOpenVisitRooms(txCtx, 0)
		require.NoError(t, err)
		assert.Empty(t, rooms)
		ended, err := module.CloseVisits(txCtx, []int64{foreign.ID}, time.Now())
		require.NoError(t, err)
		assert.Empty(t, ended)
		return module.DeleteVisit(txCtx, foreign.ID)
	}))
	copy := foreign
	copy.TenantID = tenant.FromContext(ctx)
	_, err = module.ReviseVisit(ctx, copy)
	require.Error(t, err)
	stored, err := module.FindVisit(foreignCtx, foreign.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime)
	_, err = module.ListVisits(ctx, studentpresence.VisitFilter{ForUpdate: true})
	require.Error(t, err, "locks require a surrounding transaction")
	_, err = module.ListVisits(context.Background(), studentpresence.VisitFilter{})
	require.Error(t, err, "missing tenant cannot become an unscoped query")
}
