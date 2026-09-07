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

var _ studentpresence.GroupRecovery = (*studentpresence.Module)(nil)

func TestGroupRecoveryPreservesTenantAndRollsBackSnapshotMismatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	own := testpkg.CreateTestEndedActiveGroup(t, db, "Own")
	var foreign testpkg.EndedActiveGroup
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign = testpkg.CreateTestEndedActiveGroup(t, db, "Foreign")
	})
	now := time.Now()

	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.RestoreGroup(txCtx, foreign.GroupID, now)
	})
	require.ErrorContains(t, err, "snapshot mismatch for active group: expected 1 rows, updated 0")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.RestoreSupervisors(txCtx, []int64{own.SupervisorID, foreign.SupervisorID})
	})
	require.ErrorContains(t, err, "snapshot mismatch for supervisors: expected 2 rows, updated 1")
	groupEnded, supervisorEnded := testpkg.ActiveGroupEnded(t, db, own)
	assert.True(t, groupEnded)
	assert.True(t, supervisorEnded, "snapshot mismatch must roll back the own-tenant update")
	groupEnded, supervisorEnded = testpkg.ActiveGroupEnded(t, db, foreign)
	assert.True(t, groupEnded)
	assert.True(t, supervisorEnded)

	abort := errors.New("failure after group restoration")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := module.RestoreGroup(txCtx, own.GroupID, now); err != nil {
			return err
		}
		if err := module.RestoreSupervisors(txCtx, []int64{own.SupervisorID}); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	groupEnded, supervisorEnded = testpkg.ActiveGroupEnded(t, db, own)
	assert.True(t, groupEnded, "an error after the group restore must roll it back")
	assert.True(t, supervisorEnded)

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.LockSupervisors(txCtx, []int64{own.SupervisorID}))
		if err := module.RestoreGroup(txCtx, own.GroupID, now); err != nil {
			return err
		}
		return module.RestoreSupervisors(txCtx, []int64{own.SupervisorID})
	}))
	groupEnded, supervisorEnded = testpkg.ActiveGroupEnded(t, db, own)
	assert.False(t, groupEnded)
	assert.False(t, supervisorEnded)
	groupEnded, supervisorEnded = testpkg.ActiveGroupEnded(t, db, foreign)
	assert.True(t, groupEnded, "the foreign tenant's group stays ended")
	assert.True(t, supervisorEnded)

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return module.LockOpenSupervisors(txCtx, own.GroupID)
	}))
	require.Error(t, module.LockOpenSupervisors(ctx, own.GroupID), "locks require the caller's transaction")
	require.Error(t, module.RestoreGroup(ctx, own.GroupID, now), "a write requires the caller's transaction")
}
