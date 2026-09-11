package compose

// Facilities answers "which rooms are released" (#3065). The shared open-room
// view needs that list to include rooms nobody is currently supervising, so the
// filter reads the stored release and nothing else — not occupancy, not
// sessions, not the room's name.

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openFilter(released bool) facilities.RoomFilter {
	return facilities.RoomFilter{IsOpenRoom: &released}
}

func roomNames(rooms []facilities.Room) []string {
	names := make([]string, 0, len(rooms))
	for _, room := range rooms {
		names = append(names, room.Name)
	}
	return names
}

func TestListRoomsFiltersByTheStoredRelease(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	gym, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Turnhalle", IsOpenRoom: true})
	require.NoError(t, err)
	craft, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Werkraum", IsOpenRoom: true})
	require.NoError(t, err)
	ordinary, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Mensa"})
	require.NoError(t, err)

	released, err := module.ListRooms(ctx, openFilter(true))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Turnhalle", "Werkraum"}, roomNames(released),
		"several rooms can be released at the same time")

	unreleased, err := module.ListRooms(ctx, openFilter(false))
	require.NoError(t, err)
	assert.Contains(t, roomNames(unreleased), "Mensa")
	assert.NotContains(t, roomNames(unreleased), "Turnhalle")

	// No opinion returns both, so the filter never narrows an unrelated query.
	all, err := module.ListRooms(ctx, facilities.RoomFilter{})
	require.NoError(t, err)
	assert.Subset(t, roomNames(all), []string{"Turnhalle", "Werkraum", "Mensa"})

	_ = gym
	_ = craft
	_ = ordinary
}

// TestReleasedRoomsAreListedWhileEmpty is the criterion the previous
// supervision-derived list could not meet: a released room with no session and
// nobody supervising it still has to be reachable.
func TestReleasedRoomsAreListedWhileEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Turnhalle", IsOpenRoom: true})
	require.NoError(t, err)

	released, err := module.ListRooms(ctx, openFilter(true))
	require.NoError(t, err)
	require.Len(t, released, 1)
	assert.Equal(t, "Turnhalle", released[0].Name)
	assert.True(t, released[0].IsOpenRoom)
}

func TestReleasedRoomListIsIsolatedPerTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Turnhalle", IsOpenRoom: true})
	require.NoError(t, err)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenant))

	foreign, err := module.ListRooms(otherCtx, openFilter(true))
	require.NoError(t, err)
	assert.Empty(t, foreign, "another school's released rooms must not leak into this list")
}

// TestReleaseFilterCombinesWithOtherPredicates guards the grouping: the release
// must narrow a query, never replace its other conditions.
func TestReleaseFilterCombinesWithOtherPredicates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	_, err := module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Turnhalle", Building: "Sporthalle", IsOpenRoom: true,
	})
	require.NoError(t, err)
	_, err = module.CreateRoom(ctx, facilities.CreateRoom{
		Name: "Werkraum", Building: "Hauptgebäude", IsOpenRoom: true,
	})
	require.NoError(t, err)

	building := "Sporthalle"
	released := true
	rooms, err := module.ListRooms(ctx, facilities.RoomFilter{
		Building: &building, IsOpenRoom: &released,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Turnhalle"}, roomNames(rooms),
		"the building predicate still applies alongside the release")
}
