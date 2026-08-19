// Package active_test tests that device location is auto-updated when sessions start.
package active_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func buildDeviceLocationService(t *testing.T, db *bun.DB) activeSvc.Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

func TestDeviceLocation_UpdatedOnSessionStart(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := buildDeviceLocationService(t, db)
	deviceRepo := repositories.NewFactory(db).Device
	ctx := testpkg.Ctx(t)

	t.Run("session start updates device room_id", func(t *testing.T) {
		// ARRANGE
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Location Update Activity")
		device := testpkg.CreateTestDevice(t, db, "loc-update-device")
		room := testpkg.CreateTestRoom(t, db, "Location Update Room")
		staff := testpkg.CreateTestStaff(t, db, "Location", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff.ID, 0)

		// Verify device has no room before session
		before, err := deviceRepo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Nil(t, before.RoomID, "device should have no room before any session")

		// ACT: start a session on this device in this room
		session, err := service.StartActivitySessionWithSupervisors(
			ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID,
		)
		require.NoError(t, err)
		require.NotNil(t, session)

		// ASSERT: device's room_id should now match the session's room
		after, err := deviceRepo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, after.RoomID, "device room_id should be set after session start")
		assert.Equal(t, room.ID, *after.RoomID)
	})

	t.Run("second session in different room updates device room_id", func(t *testing.T) {
		// ARRANGE
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Room Switch Activity")
		device := testpkg.CreateTestDevice(t, db, "room-switch-device")
		room1 := testpkg.CreateTestRoom(t, db, "First Session Room")
		room2 := testpkg.CreateTestRoom(t, db, "Second Session Room")
		staff := testpkg.CreateTestStaff(t, db, "Switch", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room1.ID, staff.ID, 0)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room2.ID)

		// Start first session in room1
		session1, err := service.StartActivitySessionWithSupervisors(
			ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room1.ID,
		)
		require.NoError(t, err)
		require.NotNil(t, session1)

		mid, err := deviceRepo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, mid.RoomID)
		assert.Equal(t, room1.ID, *mid.RoomID)

		// End the first session
		err = service.EndActivitySession(ctx, session1.ID)
		require.NoError(t, err)

		// ACT: start second session in room2 (force start to handle any lingering state)
		session2, err := service.ForceStartActivitySessionWithSupervisors(
			ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room2.ID,
		)
		require.NoError(t, err)
		require.NotNil(t, session2)

		// ASSERT: device room_id should now be room2
		after, err := deviceRepo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, after.RoomID)
		assert.Equal(t, room2.ID, *after.RoomID,
			"device room_id should reflect the latest session's room")
	})
}
