package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModulePaginatesRoomQueriesInTheOwner(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	building := "Pagination"

	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Alpha", Building: building})
	require.NoError(t, err)
	_, err = module.CreateRoom(ctx, facilities.CreateRoom{Name: "Bravo", Building: building})
	require.NoError(t, err)

	page, err := module.ListRoomsPage(ctx, facilities.RoomFilter{Building: &building}, 1, 1)

	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	require.Len(t, page.Rooms, 1)
	assert.Equal(t, "Bravo", page.Rooms[0].Name)
}
