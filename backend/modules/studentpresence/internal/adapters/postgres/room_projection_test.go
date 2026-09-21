package postgres_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGroupRoomProjectionPreservesOpenRoomFlag(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	for _, open := range []bool{true, false} {
		_, err := db.NewUpdate().Table("facilities.rooms").
			Set("is_open_room = ?", open).
			Where("id = ? AND tenant_id = ?", group.RoomID, testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
		groups, err := sessionGroups(repos.ActiveGroup).FindByIDs(ctx, []int64{group.ID})
		require.NoError(t, err)
		require.NotNil(t, groups[group.ID])
		require.NotNil(t, groups[group.ID].Room)
		require.Equal(t, open, groups[group.ID].Room.IsOpenRoom)
	}
}
