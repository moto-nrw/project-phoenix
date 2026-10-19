package api

import (
	"context"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCleanupRootResolvesRoomsThroughOwner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	sessions, _, _ := NewSessionCleanupRepositories(testpkg.NewDatabase(t, db), owners.NewTestTimetable(t, testpkg.MustDatabase(db)))
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	activity := testpkg.CreateTestActivityGroup(t, db, "Room Activity")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	err := testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		groups, err := sessions.FindByIDs(ctx, []int64{group.ID})
		require.NoError(t, err)
		require.NotNil(t, groups[group.ID].Room)
		assert.Equal(t, room.Name, groups[group.ID].Room.Name)
		return nil
	})
	require.NoError(t, err)
}

// The active group, education group and device reads used to join
// facilities.rooms themselves. After the cutover (#2665) the test repositories bind
// the room owner into every one of them, so a bare NewFactory graph resolves
// rooms exactly like the observed production graph.
func TestFactoryResolvesRoomsThroughTheOwner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	factory := owners.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db))
	factoryPresence := NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	devices, err := NewDeviceRepository(testpkg.NewDatabase(t, db))
	require.NoError(t, err)

	activity := testpkg.CreateTestActivityGroup(t, db, "Room Activity")
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	educationGroup := testpkg.CreateTestEducationGroup(t, db, "Igel")
	device := testpkg.CreateTestDevice(t, db, "room-device")
	ctx := testpkg.Ctx(t)
	_, err = db.NewUpdate().TableExpr("education.groups").Set("room_id = ?", room.ID).Where("id = ?", educationGroup.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().TableExpr("iot.devices").Set("room_id = ?", room.ID).Where("id = ?", device.ID).Exec(ctx)
	require.NoError(t, err)

	err = testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		rooms, err := sessionRooms(ctx, factoryPresence.ActiveGroup, []int64{activeGroup.ID})
		require.NoError(t, err)
		require.NotNil(t, rooms[activeGroup.ID], "active group carries its room")
		assert.Equal(t, room.Name, rooms[activeGroup.ID].Name)
		assert.Equal(t, room.Building, rooms[activeGroup.ID].Building, "the full row, colour and capacity included")
		assert.Equal(t, room.Capacity, rooms[activeGroup.ID].Capacity)

		withRoom, err := factory.Group.FindWithRoom(ctx, educationGroup.ID)
		require.NoError(t, err)
		require.NotNil(t, withRoom.Room, "education group carries its room")
		assert.Equal(t, room.Name, withRoom.Room.Name)
		assert.Equal(t, room.Building, withRoom.Room.Building)

		found, err := devices.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, found.RoomName, "device carries its room name")
		assert.Equal(t, room.Name, *found.RoomName)
		return nil
	})
	require.NoError(t, err)
}
