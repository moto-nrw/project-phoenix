package devicefleet_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicDeviceContractPersistsRoomAndMapsMissingDevice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fleet, err := api.NewDeviceFleet(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, db, "Device contract room")
	device, err := fleet.CreateDevice(testpkg.Ctx(t), devicefleet.CreateDevice{DeviceID: "contract-reader", DeviceType: "terminal", Status: devicefleet.DeviceStatusActive, RoomID: &room.ID})
	require.NoError(t, err)
	require.Positive(t, device.ID)
	require.Equal(t, testpkg.Tenant(t), device.TenantID)
	require.True(t, device.HasAPIKey())
	found, err := fleet.FindDevice(testpkg.Ctx(t), device.ID)
	require.NoError(t, err)
	require.Equal(t, "contract-reader", found.DeviceID)
	require.Equal(t, &room.ID, found.RoomID)
	require.Equal(t, &room.Name, found.RoomName)
	require.NoError(t, fleet.DeleteDevice(testpkg.Ctx(t), device.ID))
	_, err = fleet.FindDevice(testpkg.Ctx(t), device.ID)
	require.ErrorIs(t, err, devicefleet.ErrDeviceNotFound)
}
