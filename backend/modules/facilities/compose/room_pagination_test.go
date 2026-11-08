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

// One statement returns the page and its total (#5888); a page past the end
// carries no row, so the total still has to come back, and a filter that
// matches nothing has a total of zero.
func TestModuleRoomPageTotalsMatchTheFilterOnEveryPage(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))
	building := "Totals"
	for _, name := range []string{"Charlie", "Alpha", "Bravo"} {
		_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: name, Building: building})
		require.NoError(t, err)
	}
	filter := facilities.RoomFilter{Building: &building}

	first, err := module.ListRoomsPage(ctx, filter, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, first.Total)
	require.Len(t, first.Rooms, 2)
	assert.Equal(t, []string{"Alpha", "Bravo"}, []string{first.Rooms[0].Name, first.Rooms[1].Name})

	last, err := module.ListRoomsPage(ctx, filter, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, last.Total)
	require.Len(t, last.Rooms, 1)
	assert.Equal(t, "Charlie", last.Rooms[0].Name)

	beyond, err := module.ListRoomsPage(ctx, filter, 10, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, beyond.Total, "a page past the end still reports the total")
	assert.Empty(t, beyond.Rooms)

	missing := "Nowhere"
	none, err := module.ListRoomsPage(ctx, facilities.RoomFilter{Building: &missing}, 0, 2)
	require.NoError(t, err)
	assert.Zero(t, none.Total)
	assert.Empty(t, none.Rooms)
}
