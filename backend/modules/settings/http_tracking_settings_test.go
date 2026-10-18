package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type trackingSettings struct {
	enabled    bool
	labels     map[string]string
	failingKey string
}

func (s trackingSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == s.failingKey {
		return false, errors.New("toggle unavailable")
	}
	return s.enabled, nil
}

func (s trackingSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == s.failingKey {
		return "", errors.New("label unavailable")
	}
	return s.labels[key], nil
}

func TestPrefetchedTrackingIndicatorLabelsTrimAndSkipBlankLabels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	labels := map[string]string{KeyTrackingIndicator1: " Hausaufgaben ", KeyTrackingIndicator2: "  ", KeyTrackingIndicator3: "Essen"}

	got, err := PrefetchedTrackingIndicatorLabels(ctx, trackingSettings{labels: labels})
	require.NoError(t, err)
	require.Nil(t, got, "switched-off indicators have no labels")

	got, err = PrefetchedTrackingIndicatorLabels(ctx, trackingSettings{enabled: true, labels: labels})
	require.NoError(t, err)
	require.Equal(t, []string{"Hausaufgaben", "Essen"}, got)

	got, err = PrefetchedTrackingIndicatorLabels(ctx, trackingSettings{enabled: true})
	require.NoError(t, err)
	require.NotNil(t, got, "enabled indicators without labels answer an empty list")
	require.Empty(t, got)
}

func TestPrefetchedTrackingIndicatorLabelsFailOnAnyRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, key := range []string{KeyTrackingIndicatorsEnabled, KeyTrackingIndicator2} {
		got, err := PrefetchedTrackingIndicatorLabels(ctx, trackingSettings{enabled: true, failingKey: key})
		require.Error(t, err, key)
		require.Nil(t, got, key)
	}
}
