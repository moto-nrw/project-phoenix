package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindFacilitiesRequiresCapability(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { (&repositories.Factory{}).BindFacilities(nil) })
}

// The active group, visit, education group and device reads used to join
// facilities.rooms themselves. After the cutover (#2665) the factory binds
// the room owner into every one of them, so a bare NewFactory graph resolves
// rooms exactly like the observed production graph.
func TestFactoryResolvesRoomsThroughTheOwner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	factory := repositories.NewFactory(db)

	activity := testpkg.CreateTestActivityGroup(t, db, "Room Activity")
	room := testpkg.CreateTestRoom(t, db, "Igelraum")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Room", "Student", "1a")
	testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now().Add(-time.Minute), nil)
	educationGroup := testpkg.CreateTestEducationGroup(t, db, "Igel")
	device := testpkg.CreateTestDevice(t, db, "room-device")
	ctx := testpkg.Ctx(t)
	_, err := db.NewUpdate().TableExpr("education.groups").Set("room_id = ?", room.ID).Where("id = ?", educationGroup.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().TableExpr("iot.devices").Set("room_id = ?", room.ID).Where("id = ?", device.ID).Exec(ctx)
	require.NoError(t, err)

	err = testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		groups, err := factory.ActiveGroup.FindByIDs(ctx, []int64{activeGroup.ID})
		require.NoError(t, err)
		require.NotNil(t, groups[activeGroup.ID].Room, "active group carries its room")
		assert.Equal(t, room.Name, groups[activeGroup.ID].Room.Name)
		assert.Equal(t, room.Building, groups[activeGroup.ID].Room.Building, "the full row, colour and capacity included")
		assert.Equal(t, room.Capacity, groups[activeGroup.ID].Room.Capacity)

		locations, err := factory.ActiveVisit.GetCurrentRoomNamesForStudents(ctx, []int64{student.ID})
		require.NoError(t, err)
		assert.Equal(t, map[int64]string{student.ID: room.Name}, locations)

		current, err := factory.ActiveVisit.GetCurrentByStudentIDWithRoom(ctx, student.ID)
		require.NoError(t, err)
		require.NotNil(t, current.ActiveGroup)
		require.NotNil(t, current.ActiveGroup.Room)
		assert.Equal(t, room.ID, current.ActiveGroup.Room.ID)

		withRoom, err := factory.Group.FindWithRoom(ctx, educationGroup.ID)
		require.NoError(t, err)
		require.NotNil(t, withRoom.Room, "education group carries its room")
		assert.Equal(t, room.Name, withRoom.Room.Name)
		assert.Equal(t, room.Building, withRoom.Room.Building)

		found, err := factory.Device.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, found.RoomName, "device carries its room name")
		assert.Equal(t, room.Name, *found.RoomName)
		return nil
	})
	require.NoError(t, err)
}

// Deviceless claiming is limited to rooms named "Schulhof"; the filter moved
// from the former INNER JOIN into the owner-backed read.
func TestFindUnclaimedKeepsOnlySchulhofGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	factory := repositories.NewFactory(db)

	var schulhofID int64
	err := db.NewRaw("INSERT INTO facilities.rooms (tenant_id, name) VALUES (?, ?) RETURNING id", tenantID, "Schulhof").Scan(testpkg.Ctx(t), &schulhofID)
	require.NoError(t, err)
	other := testpkg.CreateTestRoom(t, db, "Igelraum")
	activity := testpkg.CreateTestActivityGroup(t, db, "Unclaimed Activity")
	yard := testpkg.CreateTestActiveGroup(t, db, activity.ID, schulhofID)
	testpkg.CreateTestActiveGroup(t, db, activity.ID, other.ID)

	err = testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		groups, err := factory.ActiveGroup.FindUnclaimed(ctx)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, yard.ID, groups[0].ID)
		require.NotNil(t, groups[0].Room)
		assert.Equal(t, "Schulhof", groups[0].Room.Name)
		return nil
	})
	require.NoError(t, err)
}
