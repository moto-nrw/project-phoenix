package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisitRecoveryPreservesTenantAndRollsBackSnapshotMismatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Recovery", "Own", "3a")
	activity := testpkg.CreateTestActivityGroup(t, db, "Recovery")
	room := testpkg.CreateTestRoom(t, db, "Recovery")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	at := time.Now()
	closed := testpkg.CreateTestVisit(t, db, student.ID, group.ID, at.Add(-time.Hour), &at)
	var foreignVisitID, foreignStudentID int64
	var foreignCtx context.Context
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		student := testpkg.CreateTestStudent(t, db, "Recovery", "Foreign", "3b")
		activity := testpkg.CreateTestActivityGroup(t, db, "Foreign recovery")
		room := testpkg.CreateTestRoom(t, db, "Foreign recovery")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, at.Add(-time.Hour), &at)
		foreignVisitID, foreignStudentID = visit.ID, student.ID
	})
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.RestoreVisits(txCtx, []int64{closed.ID, foreignVisitID})
	})
	require.ErrorContains(t, err, "snapshot mismatch for visits: expected 2 rows, updated 1")
	open, err := module.ListOpenPresence(ctx, []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, open, "snapshot mismatch must roll back the own-tenant update")
	abort := errors.New("failure after visit restoration")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := module.RestoreVisits(txCtx, []int64{closed.ID}); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.RestoreVisits(txCtx, []int64{closed.ID})
	}))
	open, err = module.ListOpenPresence(ctx, []int64{student.ID, foreignStudentID})
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, open)
	open, err = module.ListOpenPresence(foreignCtx, []int64{foreignStudentID})
	require.NoError(t, err)
	assert.Empty(t, open)
}
