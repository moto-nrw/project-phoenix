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
	retention := func(context.Context) (int, error) { return 90, nil }

	_, err := NewBoundSettings(nil, retention)
	require.Error(t, err)
	_, err = NewBoundSettings(enabled, nil)
	require.Error(t, err)

	settings, err := NewBoundSettings(enabled, retention)
	require.NoError(t, err)
	feedbackEnabled, err := settings.FeedbackEnabled(context.Background())
	require.NoError(t, err)
	assert.True(t, feedbackEnabled)
	days, err := settings.FeedbackRetentionDays(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 90, days)
}
