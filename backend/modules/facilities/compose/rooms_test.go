package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ facilities.Catalog = (facilities.Capability)(nil)
)

func TestNewRoomsRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	_, err := NewRooms(Dependencies{Observe: func(Observation) {}})
	require.EqualError(t, err, "facilities compose: all dependencies are required")
	_, err = NewRooms(Dependencies{DB: db})
	require.EqualError(t, err, "facilities compose: all dependencies are required")
}

// The Rooms a root builds before the deletion guard serve the reads and the
// create and update writes over the same rows as the whole module.
func TestRoomsServeTheCatalogBeforeTheDeletionGuard(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	rooms, err := NewRooms(Dependencies{DB: db, Observe: log.record})
	require.NoError(t, err)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	created, err := rooms.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
	require.NoError(t, err)
	updated, err := rooms.UpdateRoom(ctx, facilities.UpdateRoom{ID: created.ID, Name: "Fuchsbau"})
	require.NoError(t, err)
	assert.Equal(t, "Fuchsbau", updated.Name)

	found, err := buildModule(t, db, func(Observation) {}).FindRoom(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, updated, found)

	_, err = rooms.FindRoom(ctx, created.ID+1_000_000)
	assert.ErrorIs(t, err, facilities.ErrRoomNotFound)
	assert.Equal(t, []string{"create_room", "update_room", "find_room"}, log.operations())
	assert.ErrorIs(t, log.seen[2].Err, facilities.ErrRoomNotFound, "observations carry the public error")
}

func TestRoomRecordsReadTheRetainedRowsOverTheRooms(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rooms, err := NewRooms(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	igel := testpkg.CreateTestRoom(t, db, "Igelraum")
	records := NewRoomRecords(rooms)
	ctx := testpkg.Ctx(t)

	found, err := records.FindByID(ctx, igel.ID)
	require.NoError(t, err)
	assert.Equal(t, igel.Name, found.Name)
	listed, err := records.FindByIDs(ctx, []int64{igel.ID})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	names, err := NewRoomNameDirectory(records).RoomNamesByID(ctx, []int64{igel.ID})
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{igel.ID: igel.Name}, names)
	_, err = records.FindByID(ctx, "1")
	require.EqualError(t, err, "room ID must be an integer, got string")
}
