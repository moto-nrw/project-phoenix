package identityaccess_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Without a settings service the gate is off, trusted devices are allowed,
// and the numeric parameters answer zero so the flows use their fallbacks.
func TestMFASettingsWithoutServiceAnswersTheUnconfiguredGate(t *testing.T) {
	t.Parallel()
	const tenantID int64 = 10
	ctx := context.Background()
	settings := identityaccess.MFASettingsWithoutService{}

	mode, err := settings.MFAMode(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.MFAModeOff, mode)
	mode, err = settings.MFAModeInTx(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.MFAModeOff, mode)

	enabled, err := settings.TrustedDeviceEnabled(ctx, tenantID)
	require.NoError(t, err)
	assert.True(t, enabled)

	days, err := settings.TrustedDeviceDays(ctx, tenantID)
	require.NoError(t, err)
	assert.Zero(t, days)
	threshold, err := settings.LockoutThreshold(ctx, tenantID)
	require.NoError(t, err)
	assert.Zero(t, threshold)
	duration, err := settings.LockoutDuration(ctx, tenantID)
	require.NoError(t, err)
	assert.Zero(t, duration)
}

// The relying-party name and the demo's operator sign-in stay pinned.
func TestPasskeyAndDemoConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "moto", identityaccess.PasskeyRPName)
	assert.True(t, identityaccess.DemoOperatorWithoutSecondFactor)
}
