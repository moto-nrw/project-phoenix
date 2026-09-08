package compose_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestNewFailsFastWithoutTheRequiredCapabilities(t *testing.T) {
	t.Parallel()
	assert.PanicsWithValue(t, "device scan composition: fleet, presence, rooms, active and users are required", func() {
		devicescanCompose.New(devicescanCompose.Dependencies{})
	})
}

// The composed workflow answers the device it was authenticated as and
// records its heartbeat through the real Device Fleet capability.
func TestComposedWorkflowServesTheKiosk(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupCheckinModule(t)
	device := testpkg.CreateTestDevice(t, db, "composed-kiosk")
	req := testutil.NewAuthenticatedRequest(t, "POST", "/ping", nil, testutil.WithDeviceContext(device))
	ctx := testpkg.WithTestTenantRuntime(t, req.Context())

	identity, err := module.DeviceScan.Device(ctx)
	require.NoError(t, err)
	assert.Equal(t, device.ID, identity.ID)
	assert.Equal(t, device.DeviceID, identity.DeviceID)

	ping, err := module.DeviceScan.Ping(ctx)
	require.NoError(t, err)
	assert.Equal(t, device.DeviceID, ping.Device.DeviceID)
	assert.False(t, ping.SessionActive)

	status, err := module.DeviceScan.Status(ctx)
	require.NoError(t, err)
	assert.True(t, status.Device.Active)
}

func TestComposedWorkflowRejectsRequestsWithoutADevice(t *testing.T) {
	t.Parallel()
	_, module := testutil.SetupCheckinModule(t)

	_, err := module.DeviceScan.Device(testpkg.Ctx(t))

	require.Error(t, err)
	assert.EqualError(t, err, "device API key is required")
}
