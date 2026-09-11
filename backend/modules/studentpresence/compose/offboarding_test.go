package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffOffboardingSupervisionBlocker(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Presence", "Offboarding")
	activity := testpkg.CreateTestActivityGroup(t, db, "Offboarding")
	room := testpkg.CreateTestRoom(t, db, "Offboarding")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	date := supervisor.StartDate.String()

	_, err = module.LockStaffSupervision(ctx, staff.ID, date)
	require.ErrorContains(t, err, "transaction is required")
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		ids, err := module.LockStaffSupervision(txCtx, staff.ID, date)
		require.NoError(t, err)
		require.Equal(t, []int64{supervisor.ID}, ids)
		return nil
	}))
	t.Run("foreign tenant", func(t *testing.T) {
		testpkg.OwnTenant(t)
		require.NoError(t, tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
			ids, err := module.LockStaffSupervision(txCtx, staff.ID, date)
			require.NoError(t, err)
			require.Empty(t, ids)
			return nil
		}))
	})
	testpkg.EndTestActiveGroup(t, db, testpkg.EndedActiveGroup{GroupID: group.ID, SupervisorID: supervisor.ID})
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		ids, err := module.LockStaffSupervision(txCtx, staff.ID, date)
		require.NoError(t, err)
		require.Empty(t, ids, "ended supervision is retained history, not a blocker")
		return nil
	}))
}
