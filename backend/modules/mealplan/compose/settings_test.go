package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoundSettingsResolvesThroughTheBoundResolvers(t *testing.T) {
	t.Parallel()
	enabled := func(context.Context) (bool, error) { return true, nil }
	registration := func(context.Context) (bool, error) { return false, nil }
	cutoff := func(context.Context) (OrderingSettings, error) { return singleDeadlineSettings("08:30"), nil }

	for name, construct := range map[string]func() (*Settings, error){
		"plan toggle":         func() (*Settings, error) { return NewBoundSettings(nil, registration, cutoff) },
		"registration toggle": func() (*Settings, error) { return NewBoundSettings(enabled, nil, cutoff) },
		"cutoff":              func() (*Settings, error) { return NewBoundSettings(enabled, registration, nil) },
	} {
		_, err := construct()
		require.Error(t, err, name)
	}

	settings, err := NewBoundSettings(enabled, registration, cutoff)
	require.NoError(t, err)
	ctx := context.Background()
	planEnabled, err := settings.MealPlanEnabled(ctx)
	require.NoError(t, err)
	assert.True(t, planEnabled)
	registrationEnabled, err := settings.MealRegistrationEnabled(ctx)
	require.NoError(t, err)
	assert.False(t, registrationEnabled)
	ordering, err := settings.MealOrderingSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, "08:30", ordering.RegistrationTime)
}
