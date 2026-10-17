package settings

import (
	"context"
	"strings"
)

// TrackingIndicatorLabels reads the enabled labels from one settings snapshot.
// Individual unavailable labels are omitted; a failed toggle read is returned.
func TrackingIndicatorLabels(ctx context.Context, settings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveString(context.Context, string) (string, error)
}) (context.Context, []string, error) {
	keys := []string{KeyTrackingIndicator1, KeyTrackingIndicator2, KeyTrackingIndicator3}
	ctx = PrefetchSettings(ctx, settings, append([]string{KeyTrackingIndicatorsEnabled}, keys...)...)
	enabled, err := settings.ResolveBool(ctx, KeyTrackingIndicatorsEnabled)
	if err != nil || !enabled {
		return ctx, nil, err
	}
	var labels []string
	for _, key := range keys {
		value, err := settings.ResolveString(ctx, key)
		if err != nil {
			continue
		}
		if label := strings.TrimSpace(value); label != "" {
			labels = append(labels, label)
		}
	}
	return ctx, labels, nil
}
