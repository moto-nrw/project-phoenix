package compose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestUnclaimedGroupOwnerIsolationRollbackAndLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, db, "Unclaimed owner")
	room := testpkg.CreateTestRoom(t, db, "Unclaimed owner")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Unclaimed", "Owner")
	claim := studentpresence.GroupClaim{GroupID: group.ID, StaffID: staff.ID, Role: "supervisor", Date: testpkg.TodayDate().String()}
	var foreignID int64
	t.Run("foreign tenant", func(t *testing.T) {
		testpkg.OwnTenant(t)
		activity := testpkg.CreateTestActivityGroup(t, db, "Foreign")
		room := testpkg.CreateTestRoom(t, db, "Foreign")
		foreignID = testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID).ID
	})
	rows, err := module.UnclaimedGroups(ctx, claim.Date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, group.ID, rows[0].ID)
	_, err = module.ClaimGroup(ctx, claim)
	require.ErrorContains(t, err, "transaction is required")
	abort := errors.New("abort after claim")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		row, err := module.ClaimGroup(txCtx, claim)
		require.NoError(t, err)
		require.Equal(t, staff.ID, row.StaffID)
		// An independent connection cannot acquire the lifecycle lock while
		// the owner claim is awaiting the surrounding transaction's decision.
		var id int64
		lockErr := db.NewRaw("SELECT id FROM active.groups WHERE id = ? FOR UPDATE NOWAIT", group.ID).Scan(context.Background(), &id)
		require.ErrorContains(t, lockErr, "could not obtain lock")
		return abort
	})
	require.ErrorIs(t, err, abort)
	rows, err = module.UnclaimedGroups(ctx, claim.Date)
	require.NoError(t, err)
	require.Len(t, rows, 1, "rolled-back claim leaves the group unclaimed")
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.ClaimGroup(txCtx, claim)
		return err
	}))
	rows, err = module.UnclaimedGroups(ctx, claim.Date)
	require.NoError(t, err)
	require.Empty(t, rows)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.ClaimGroup(txCtx, claim)
		return err
	})
	require.ErrorIs(t, err, studentpresence.ErrAlreadySupervising)
	claim.GroupID = foreignID
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.ClaimGroup(txCtx, claim)
		return err
	})
	require.ErrorIs(t, err, studentpresence.ErrGroupNotFound)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = module.UnclaimedGroups(cancelled, claim.Date)
	require.ErrorIs(t, err, context.Canceled)
}
