package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOpenRoomSessionsAreBatchedAndTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "OpenRoom")
	activity := testpkg.CreateTestActivityGroup(t, db, "OpenActivity")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	closed := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	_, err := db.NewUpdate().Table("active.groups").Set("end_time = NOW()").Where("tenant_id = ? AND id = ?", testpkg.Tenant(t), closed.ID).Exec(ctx)
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Room", "Supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "lead")
	var queries int
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { queries = o.Queries }})
	require.NoError(t, err)
	rows, err := module.ListOpenRoomSessions(ctx, []int64{room.ID, room.ID})
	require.NoError(t, err)
	require.Equal(t, 1, queries)
	require.Len(t, rows, 1)
	require.Equal(t, group.ID, rows[0].ActiveGroupID)
	require.Equal(t, room.ID, rows[0].RoomID)
	require.Equal(t, &activity.ID, rows[0].ActivityGroupID)
	require.Equal(t, []int64{staff.ID}, rows[0].SupervisorStaffIDs)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, otherTenant, func(otherCtx context.Context) error {
		rows, err := module.ListOpenRoomSessions(otherCtx, []int64{room.ID})
		require.NoError(t, err)
		require.Empty(t, rows)
		return nil
	}))
	rows, err = module.ListOpenRoomSessions(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, queries)
	_, err = module.ListOpenRoomSessions(context.Background(), nil)
	require.Error(t, err)
}
