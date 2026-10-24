package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type attendanceSettingSource struct {
	read func(context.Context, string) (bool, error)
}

func (s attendanceSettingSource) ResolveBool(ctx context.Context, key string) (bool, error) {
	return s.read(ctx, key)
}

func TestWebAttendanceReaderPreservesRequestContextAndFailures(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, tc := range []struct {
		name    string
		enabled bool
		err     error
	}{
		{name: "enabled", enabled: true},
		{name: "disabled"},
		{name: "resolution failure", err: errors.New("settings unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			reader := WebAttendanceEnabledReader(attendanceSettingSource{read: func(got context.Context, key string) (bool, error) {
				calls++
				require.Same(t, ctx, got)
				require.Equal(t, KeyAttendanceWebEnabled, key)
				return tc.enabled, tc.err
			}})
			require.Zero(t, calls, "construction must not resolve outside the request tenant")
			enabled, err := reader(ctx)
			require.Equal(t, tc.enabled, enabled)
			require.Equal(t, tc.err, err)
			require.Equal(t, 1, calls)
		})
	}
	require.Nil(t, WebAttendanceEnabledReader(nil))
	require.Nil(t, NewDisplaySettings(nil).WebAttendanceEnabledReader())
}
