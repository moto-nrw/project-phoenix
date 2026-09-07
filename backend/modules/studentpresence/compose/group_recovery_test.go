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

type endedGroup struct {
	groupID      int64
	supervisorID int64
}

func TestGroupRecoveryPreservesTenantAndRollsBackSnapshotMismatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	seedEndedGroup := func(t *testing.T, label string) endedGroup {
		t.Helper()
		staff := testpkg.CreateTestStaff(t, db, "Recovery", label)
		activity := testpkg.CreateTestActivityGroup(t, db, "Recovery "+label)
		room := testpkg.CreateTestRoom(t, db, "Recovery "+label)
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
		_, err := db.NewUpdate().Table("active.groups").Set("end_time = now()").Where("id = ?", group.ID).Exec(context.Background())
		require.NoError(t, err)
		_, err = db.NewUpdate().Table("active.group_supervisors").Set("end_date = CURRENT_DATE").Where("id = ?", supervisor.ID).Exec(context.Background())
		require.NoError(t, err)
		return endedGroup{groupID: group.ID, supervisorID: supervisor.ID}
	}
	groupState := func(t *testing.T, seeded endedGroup) (groupOpen, supervisorOpen bool) {
		t.Helper()
		var groupEnded, supervisorEnded bool
		require.NoError(t, db.NewSelect().TableExpr("active.groups").ColumnExpr("end_time IS NOT NULL").Where("id = ?", seeded.groupID).Scan(context.Background(), &groupEnded))
		require.NoError(t, db.NewSelect().TableExpr("active.group_supervisors").ColumnExpr("end_date IS NOT NULL").Where("id = ?", seeded.supervisorID).Scan(context.Background(), &supervisorEnded))
		return !groupEnded, !supervisorEnded
	}
	own := seedEndedGroup(t, "Own")
	var foreign endedGroup
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign = seedEndedGroup(t, "Foreign")
	})
	now := time.Now()

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.RestoreGroup(txCtx, foreign.groupID, now)
	})
	require.ErrorContains(t, err, "snapshot mismatch for active group: expected 1 rows, updated 0")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.RestoreSupervisors(txCtx, []int64{own.supervisorID, foreign.supervisorID})
	})
	require.ErrorContains(t, err, "snapshot mismatch for supervisors: expected 2 rows, updated 1")
	groupOpen, supervisorOpen := groupState(t, own)
	assert.False(t, groupOpen)
	assert.False(t, supervisorOpen, "snapshot mismatch must roll back the own-tenant update")
	groupOpen, supervisorOpen = groupState(t, foreign)
	assert.False(t, groupOpen)
	assert.False(t, supervisorOpen)

	abort := errors.New("failure after group restoration")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := module.RestoreGroup(txCtx, own.groupID, now); err != nil {
			return err
		}
		if err := module.RestoreSupervisors(txCtx, []int64{own.supervisorID}); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	groupOpen, supervisorOpen = groupState(t, own)
	assert.False(t, groupOpen, "an error after the group restore must roll it back")
	assert.False(t, supervisorOpen)

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.LockSupervisors(txCtx, []int64{own.supervisorID}))
		if err := module.RestoreGroup(txCtx, own.groupID, now); err != nil {
			return err
		}
		return module.RestoreSupervisors(txCtx, []int64{own.supervisorID})
	}))
	groupOpen, supervisorOpen = groupState(t, own)
	assert.True(t, groupOpen)
	assert.True(t, supervisorOpen)
	groupOpen, supervisorOpen = groupState(t, foreign)
	assert.False(t, groupOpen, "the foreign tenant's group stays ended")
	assert.False(t, supervisorOpen)

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.LockOpenSupervisors(txCtx, own.groupID)
	}))
	require.Error(t, module.LockOpenSupervisors(ctx, own.groupID), "locks require the caller's transaction")
	require.Error(t, module.RestoreGroup(ctx, own.groupID, now), "a write requires the caller's transaction")
	var _ studentpresence.GroupRecovery = module
}
