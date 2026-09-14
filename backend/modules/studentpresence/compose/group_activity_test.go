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
	"github.com/stretchr/testify/require"
)

func TestGroupActivityWritePreservesTenantAndTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	at := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	read := func() time.Time {
		rows, err := module.ListLiveGroups(ctx, []int64{group.ID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		return rows[0].LastActivity
	}
	original := read()
	abort := errors.New("abort group activity")
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.RecordGroupActivity(txCtx, group.ID, at))
		return abort
	}), abort)
	require.True(t, original.Equal(read()))
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.Error(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		return module.RecordGroupActivity(otherCtx, group.ID, at)
	}))
	require.True(t, original.Equal(read()))
	require.Error(t, module.RecordGroupActivity(context.Background(), group.ID, at))
	require.NoError(t, module.RecordGroupActivity(ctx, group.ID, at))
	require.True(t, at.Equal(read()))
	require.NoError(t, module.EndGroup(ctx, group.ID, at))
	require.ErrorIs(t, module.RecordGroupActivity(ctx, group.ID, at.Add(time.Minute)), studentpresence.ErrGroupNotOpen)
	require.True(t, at.Equal(read()))
}

func TestGroupDeletionIsTenantScopedAndTransactional(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	abort := errors.New("rollback deletion")
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.DeleteGroup(txCtx, group.ID))
		return abort
	}), abort)
	rows, err := module.ListLiveGroups(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		return module.DeleteGroup(otherCtx, group.ID)
	}))
	rows, err = module.ListLiveGroups(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Error(t, module.DeleteGroup(context.Background(), group.ID))
	require.NoError(t, module.DeleteGroup(ctx, group.ID))
	require.NoError(t, module.DeleteGroup(ctx, group.ID), "missing rows remain idempotent")
	rows, err = module.ListLiveGroups(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestGroupRevisionPreservesIdentityAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	rows, err := module.ListLiveGroups(ctx, []int64{group.ID})
	require.NoError(t, err)
	original := rows[0]
	input := original
	input.TimeoutMinutes = 65
	input.CreatedAt = time.Time{}
	input.TenantID = 0
	abort := errors.New("rollback revision")
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.ReviseGroup(txCtx, input)
		require.NoError(t, err)
		return abort
	}), abort)
	rows, err = module.ListLiveGroups(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Equal(t, original.TimeoutMinutes, rows[0].TimeoutMinutes)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.Error(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		_, err := module.ReviseGroup(otherCtx, input)
		return err
	}))
	_, err = module.ReviseGroup(context.Background(), input)
	require.Error(t, err)
	updated, err := module.ReviseGroup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 65, updated.TimeoutMinutes)
	require.Equal(t, original.TenantID, updated.TenantID)
	require.True(t, original.CreatedAt.Equal(updated.CreatedAt))
}

func TestGroupCreationUsesTenantTransactionAndGeneratedIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Native group creation")
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	input := studentpresence.LiveGroup{RoomID: room.ID, StartTime: time.Now(), LastActivity: time.Now()}
	abort := errors.New("rollback creation")
	var abortedID int64
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		row, err := module.RecordGroup(txCtx, input)
		require.NoError(t, err)
		abortedID = row.ID
		return abort
	}), abort)
	rows, err := module.ListLiveGroups(ctx, []int64{abortedID})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = module.RecordGroup(context.Background(), input)
	require.Error(t, err)
	row, err := module.RecordGroup(ctx, input)
	require.NoError(t, err)
	require.Positive(t, row.ID)
	require.Equal(t, testpkg.Tenant(t), row.TenantID)
	require.False(t, row.CreatedAt.IsZero())
	require.False(t, row.UpdatedAt.IsZero())
	require.Nil(t, row.ActivityGroupID)
	require.Nil(t, row.DeviceID)
	require.Zero(t, row.TimeoutMinutes)
	rows, err = module.ListLiveGroups(ctx, []int64{row.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, row, rows[0])
}

func TestSupervisionEndingPreservesTenantAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	staff := testpkg.CreateTestStaff(t, db, "End", "Supervisor")
	supervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	day := supervision.StartDate.String()
	for _, command := range []struct {
		name string
		end  func(context.Context, int64, string) (int, error)
		id   int64
	}{
		{"row", module.EndSupervisionOn, supervision.ID},
		{"staff", module.EndStaffSupervisionsOn, staff.ID},
	} {
		t.Run(command.name, func(t *testing.T) {
			abort := errors.New("rollback supervision ending")
			require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				count, err := command.end(txCtx, command.id, day)
				require.NoError(t, err)
				require.Equal(t, 1, count)
				count, err = command.end(txCtx, command.id, day)
				require.NoError(t, err)
				require.Zero(t, count)
				return abort
			}), abort)
			rows, err := module.ListGroupSupervisions(ctx, group.ID)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Nil(t, rows[0].EndDate)
			other := testpkg.UniqueTestTenantID(t)
			testpkg.EnsureTestTenant(t, db, other)
			require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
				count, err := command.end(otherCtx, command.id, day)
				require.NoError(t, err)
				require.Zero(t, count)
				return err
			}))
			_, err = command.end(context.Background(), command.id, day)
			require.Error(t, err)
			_, err = command.end(ctx, command.id, "invalid-date")
			require.Error(t, err)
		})
	}
	count, err := module.EndStaffSupervisionsOn(ctx, staff.ID, day)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	rows, err := module.ListGroupSupervisions(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, &day, rows[0].EndDate)
}
