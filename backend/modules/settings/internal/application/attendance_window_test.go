package config

import (
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The window the team sees attendance never exceeds the attendance retention
// window (#3957), whichever of the two is written or reset.
func TestAttendanceVisibleDaysNeverExceedRetention(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	const visibleKey = configModel.KeyAttendanceVisibleDays
	const retentionKey = configModel.KeyGDPRAttendanceRetentionDays
	read := func(key string, want int) {
		t.Helper()
		got, err := settings.ResolveInt(ctx, key)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	read(visibleKey, 30)
	read(retentionKey, 365)

	// The moto team shortens the retention: down to the visible window, not below.
	require.NoError(t, settings.SetValue(ctx, retentionKey, 30, nil, nil))
	err := settings.SetValue(ctx, visibleKey, 31, nil, nil)
	require.ErrorIs(t, err, configModel.ErrInvalidValue)
	var window *configModel.AttendanceWindowError
	require.ErrorAs(t, err, &window)
	require.Equal(t, configModel.AttendanceWindowError{Key: visibleKey, VisibleDays: 31, RetentionDays: 30}, *window,
		"the rejection names the retention the school may not exceed")
	read(visibleKey, 30)
	require.NoError(t, settings.SetValue(ctx, retentionKey, 60, nil, nil))

	// The school widens the visible window: up to the retention, not beyond.
	require.NoError(t, settings.SetValue(ctx, visibleKey, 60, nil, nil))
	require.ErrorIs(t, settings.SetValue(ctx, retentionKey, 59, nil, nil), configModel.ErrInvalidValue)
	read(retentionKey, 60)

	// A reset restores the default 30 days of the visible window; a reset of
	// the retention restores 365 days. Both keep the pair valid.
	require.NoError(t, settings.ResetValue(ctx, visibleKey, nil, nil))
	require.NoError(t, settings.SetValue(ctx, retentionKey, 30, nil, nil))
	require.NoError(t, settings.ResetValue(ctx, retentionKey, nil, nil))
	require.NoError(t, settings.SetValue(ctx, visibleKey, 365, nil, nil))
	read(visibleKey, 365)
	read(retentionKey, 365)
}
