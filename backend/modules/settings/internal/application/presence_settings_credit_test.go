package config

import (
	"context"
	"errors"
	"testing"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// creditSettingsDouble answers the presence settings source from a map; a
// missing key is a resolution error.
type creditSettingsDouble map[string]any

func (d creditSettingsDouble) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}

func (d creditSettingsDouble) ResolveString(_ context.Context, key string) (string, error) {
	value, ok := d[key].(string)
	if !ok {
		return "", errors.New("unreadable " + key)
	}
	return value, nil
}

func (d creditSettingsDouble) ResolveInt(_ context.Context, key string) (int, error) {
	value, ok := d[key].(int)
	if !ok {
		return 0, errors.New("unreadable " + key)
	}
	return value, nil
}

func (d creditSettingsDouble) ResolveBool(context.Context, string) (bool, error) {
	return false, nil
}

func TestPresenceSettings_WorkTimeCredit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	packUp, interval, mode, err := NewPresenceSettings(creditSettingsDouble{
		configModels.KeyTrackingPackUpMinutes:           5,
		configModels.KeyTrackingRoundingIntervalMinutes: "15",
		configModels.KeyTrackingRoundingMode:            configModels.TrackingRoundingDown,
	}).WorkTimeCredit(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, packUp)
	assert.Equal(t, 15, interval)
	assert.Equal(t, configModels.TrackingRoundingDown, mode)

	packUp, interval, mode, err = NewPresenceSettings(creditSettingsDouble{
		configModels.KeyTrackingPackUpMinutes:           0,
		configModels.KeyTrackingRoundingIntervalMinutes: configModels.TrackingRoundingOff,
	}).WorkTimeCredit(ctx)
	require.NoError(t, err, "with rounding off the direction is not read")
	assert.Zero(t, packUp)
	assert.Zero(t, interval)
	assert.Empty(t, mode)

	for name, source := range map[string]creditSettingsDouble{
		"pack-up":   {configModels.KeyTrackingRoundingIntervalMinutes: "0"},
		"interval":  {configModels.KeyTrackingPackUpMinutes: 5},
		"direction": {configModels.KeyTrackingPackUpMinutes: 5, configModels.KeyTrackingRoundingIntervalMinutes: "15"},
	} {
		_, _, _, err := NewPresenceSettings(source).WorkTimeCredit(ctx)
		require.Error(t, err, "an unreadable %s is an error, not a default", name)
	}
}
