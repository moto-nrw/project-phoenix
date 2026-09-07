package legacy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
)

// stubResolver drives the device-online window resolution without a database.
// It is a pure in-memory stub: no entity IDs, no DB rows.
type stubResolver struct {
	hasOverride    bool
	hasOverrideErr error
	minutes        int
	resolveErr     error
}

func (s stubResolver) HasTenantOverride(_ context.Context, key string) (bool, error) {
	if key != configModel.KeyDeviceOnlineWindowMinutes {
		return false, nil
	}
	return s.hasOverride, s.hasOverrideErr
}

func (s stubResolver) ResolveInt(_ context.Context, key string) (int, error) {
	if key != configModel.KeyDeviceOnlineWindowMinutes {
		return 0, nil
	}
	return s.minutes, s.resolveErr
}

func TestNewOnlineWindowResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resolver stubResolver
		want     time.Duration
	}{
		{
			name:     "tenant override widens the window",
			resolver: stubResolver{hasOverride: true, minutes: 15},
			want:     15 * time.Minute,
		},
		{
			name:     "tenant override narrows the window",
			resolver: stubResolver{hasOverride: true, minutes: 2},
			want:     2 * time.Minute,
		},
		{
			name:     "no override leaves the window unresolved",
			resolver: stubResolver{hasOverride: false},
			want:     0,
		},
		{
			name:     "override-check error leaves the window unresolved",
			resolver: stubResolver{hasOverrideErr: errors.New("boom")},
			want:     0,
		},
		{
			name:     "resolve error leaves the window unresolved",
			resolver: stubResolver{hasOverride: true, resolveErr: errors.New("boom")},
			want:     0,
		},
		{
			name:     "non-positive override leaves the window unresolved",
			resolver: stubResolver{hasOverride: true, minutes: 0},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolve := legacy.NewOnlineWindowResolver(tt.resolver, nil)
			if got := resolve(context.Background()); got != tt.want {
				t.Errorf("online window = %v, want %v", got, tt.want)
			}
		})
	}
}
