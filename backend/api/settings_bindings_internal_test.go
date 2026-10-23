package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type roomHistorySettingsDouble struct {
	roomHistorySettingsReader
	failKey string
}

func (s roomHistorySettingsDouble) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == s.failKey {
		return false, errors.New("settings unavailable")
	}
	return true, nil
}

func (s roomHistorySettingsDouble) ResolveInt(_ context.Context, key string) (int, error) {
	if key == s.failKey {
		return 0, errors.New("settings unavailable")
	}
	return 3, nil
}

func TestRoomHistorySettingsPropagateReadFailures(t *testing.T) {
	t.Parallel()
	prefetch := func(ctx context.Context, _ any, _ ...string) context.Context { return ctx }
	for _, key := range []string{"gdpr.attendance_log_enabled", "gdpr.room_detail_visible_days"} {
		t.Run(key, func(t *testing.T) {
			read := NewRoomHistorySettings(roomHistorySettingsDouble{failKey: key}, nil, prefetch)
			_, _, _, err := read(context.Background())
			require.Error(t, err)
		})
	}
	require.Panics(t, func() { NewRoomHistorySettings(nil, nil, prefetch) })
}
