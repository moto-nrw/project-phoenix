package platform_test

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_OperatorActionsOnRoomAssignedDevice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildProvisioningService(t, db)
	room := testpkg.CreateTestRoom(t, db, "Operator room")
	device := testpkg.CreateTestDevice(t, db, "operator-room-device")
	operator := testpkg.CreateTestOperator(t, db)
	device.RoomID = &room.ID
	_, err := db.NewUpdate().Model(device).ModelTableExpr(`iot.devices AS "device"`).Column("room_id").WherePK().Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("transfer status", func(t *testing.T) {
		status, err := service.GetDeviceTransferStatus(ctx, device.ID)
		require.NoError(t, err)
		assert.Nil(t, status.ActiveSession)
	})
	t.Run("rotate API key", func(t *testing.T) {
		updated, err := service.SetDeviceAPIKey(ctx, device.ID, nil, operator.ID, testClientIP)
		require.NoError(t, err)
		require.NotNil(t, updated.APIKey)
		assert.NotEqual(t, *device.APIKey, *updated.APIKey)
		var storedKey string
		require.NoError(t, db.NewSelect().TableExpr("iot.devices").Column("api_key").Where("id = ?", device.ID).Scan(ctx, &storedKey))
		assert.Equal(t, *updated.APIKey, storedKey)
	})
	t.Run("delete device", func(t *testing.T) {
		require.NoError(t, service.DeleteDevice(ctx, device.ID, operator.ID, testClientIP))
		exists, err := db.NewSelect().TableExpr("iot.devices").Where("id = ?", device.ID).Exists(ctx)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestIntegration_OperatorTransferStatusIncludesRoomAssignedSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildProvisioningService(t, db)
	room := testpkg.CreateTestRoom(t, db, "Session room")
	device := testpkg.CreateTestDevice(t, db, "operator-session-device")
	activity := testpkg.CreateTestActivityGroup(t, db, "Session activity")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	_, err := db.NewUpdate().TableExpr("iot.devices").Set("room_id = ?", room.ID).Where("id = ?", device.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	_, err = db.NewUpdate().TableExpr("active.groups").Set("device_id = ?", device.ID).Where("id = ?", group.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	status, err := service.GetDeviceTransferStatus(context.Background(), device.ID)
	require.NoError(t, err)
	require.NotNil(t, status.ActiveSession)
	assert.Equal(t, group.ID, status.ActiveSession.ID)
	require.NotNil(t, status.ActiveSession.RoomName)
	assert.Equal(t, room.Name, *status.ActiveSession.RoomName)
	require.NotNil(t, status.ActiveSession.ActivityName)
	assert.Equal(t, activity.Name, *status.ActiveSession.ActivityName)
	assert.False(t, status.CanTransfer)
}
