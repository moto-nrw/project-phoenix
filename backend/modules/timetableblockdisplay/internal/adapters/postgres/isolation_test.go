package timetableblockdisplay

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestBlockDisplayReturnsOnlyTheNamedTenantsRoomAndTemplate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type side struct {
		tenantID, instanceID int64
		room                 string
	}
	var sides []side
	for _, name := range []string{"Own", "Foreign"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			room := testpkg.CreateTestRoom(t, db, name+" room")
			group := testpkg.CreateTestActivityGroup(t, db, name+" activity")
			instance := testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2026, 8, 24), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID})
			sides = append(sides, side{testpkg.Tenant(t), instance.ID, room.Name})
		})
	}
	for _, side := range sides {
		check := func(ctx context.Context, db bun.IDB) {
			t.Helper()
			rows, err := List(ctx, db, side.tenantID, []int64{sides[0].instanceID, sides[1].instanceID})
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, side.room, rows[side.instanceID].RoomName)
			require.Equal(t, "activity", rows[side.instanceID].ActivityType)
		}
		check(testpkg.Ctx(t), db)
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, side.tenantID, func(ctx context.Context, tx bun.Tx) error { check(ctx, tx); return nil }))
	}
}
