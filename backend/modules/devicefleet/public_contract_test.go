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
	require.NotNil(t, device.APIKey)
	require.NotEmpty(t, *device.APIKey)
	found, err := fleet.FindDevice(testpkg.Ctx(t), device.ID)
	require.NoError(t, err)
	require.Equal(t, "contract-reader", found.DeviceID)
	require.Equal(t, &room.ID, found.RoomID)
	require.Equal(t, &room.Name, found.RoomName)
	require.NoError(t, fleet.DeleteDevice(testpkg.Ctx(t), device.ID))
	_, err = fleet.FindDevice(testpkg.Ctx(t), device.ID)
	require.ErrorIs(t, err, devicefleet.ErrDeviceNotFound)
}

func TestDeviceCredentialIsOnlyReturnedAtCreation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fleet, err := api.NewDeviceFleet(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	created, err := fleet.CreateDevice(ctx, devicefleet.CreateDevice{DeviceID: "credential-reader", DeviceType: "terminal"})
	require.NoError(t, err)
	require.NotNil(t, created.APIKey)
	key := *created.APIKey
	found, err := fleet.FindDeviceByAPIKey(ctx, key)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.Len(t, found.MaskedAPIKey, 7, "reads expose only a masked suffix, never the complete key")
	require.True(t, found.APIKey == nil, "reads must never return a device credential")
	listed, err := fleet.ListDevices(ctx, devicefleet.DeviceFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.True(t, listed[0].APIKey == nil, "list must never return a device credential")
	_, err = fleet.FindDevice(testpkg.NewTenantScope(t, db).Context(), created.ID)
	require.ErrorIs(t, err, devicefleet.ErrDeviceNotFound)
}

func TestDeviceCredentialRotationRevokesOldKeyAndPreservesExactBytes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fleet, err := api.NewDeviceFleet(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	oldKey := " " + testpkg.NewTestSigningSecret() + " "
	created, err := fleet.CreateDevice(ctx, devicefleet.CreateDevice{DeviceID: "legacy-reader", DeviceType: "terminal", APIKey: &oldKey})
	require.NoError(t, err)
	found, err := fleet.FindDeviceByAPIKey(ctx, oldKey)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	_, err = fleet.FindDeviceByAPIKey(ctx, oldKey[1:len(oldKey)-1])
	require.ErrorIs(t, err, devicefleet.ErrDeviceNotFound)
	newKey := testpkg.NewTestSigningSecret()
	updated, err := fleet.UpdateDevice(ctx, devicefleet.UpdateDevice{ID: found.ID, DeviceID: found.DeviceID, DeviceType: found.DeviceType, Status: found.Status, APIKey: &newKey})
	require.NoError(t, err)
	require.True(t, updated.APIKey == nil, "storage updates must not disclose credentials")
	_, err = fleet.FindDeviceByAPIKey(ctx, oldKey)
	require.ErrorIs(t, err, devicefleet.ErrDeviceNotFound)
	found, err = fleet.FindDeviceByAPIKey(ctx, newKey)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.Len(t, *found.APIKeyHash, 64)
	require.True(t, *found.APIKeyHash != newKey, "stored value must be a hash, not the credential")
}
