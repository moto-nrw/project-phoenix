package common

import (
	"context"
	"errors"
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/require"
)

type snapshotOnlySettings struct {
	resolve func(context.Context, []string) (*configSvc.SettingsSnapshot, error)
}

func (s snapshotOnlySettings) ResolveMany(ctx context.Context, keys []string) (*configSvc.SettingsSnapshot, error) {
	return s.resolve(ctx, keys)
}

func TestPrefetchSettingsNeedsOnlySnapshotCapability(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, tc := range []struct {
		name     string
		snapshot *configSvc.SettingsSnapshot
		err      error
		attached bool
	}{
		{name: "success", snapshot: &configSvc.SettingsSnapshot{}, attached: true},
		{name: "nil snapshot"},
		{name: "read failure", err: errors.New("snapshot unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			settings := snapshotOnlySettings{resolve: func(gotCtx context.Context, keys []string) (*configSvc.SettingsSnapshot, error) {
				calls++
				require.Equal(t, ctx, gotCtx)
				require.Equal(t, []string{"first", "second"}, keys)
				return tc.snapshot, tc.err
			}}
			got := PrefetchSettings(ctx, settings, "first", "second")
			require.Equal(t, 1, calls)
			if tc.attached {
				require.NotEqual(t, ctx, got)
			} else {
				require.Equal(t, ctx, got)
			}
		})
	}
	require.Equal(t, ctx, PrefetchSettings(ctx, nil, "first"))
	require.Equal(t, ctx, PrefetchSettings(ctx, struct{}{}, "first"))
}
