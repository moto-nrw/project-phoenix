package iot

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
)

// stubResolver is a hand-rolled SettingsResolver used to drive the
// device-online window resolution without a database. It is a pure in-memory
// stub: no entity IDs, no DB rows.
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

func TestService_IsDeviceOnlineAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		resolver SettingsResolver // nil means no resolver injected -> default window
		device   *iotModels.Device
		want     bool
	}{
		{
			name:   "nil device is offline",
			device: nil,
			want:   false,
		},
		{
			name:   "nil LastSeen is offline",
			device: &iotModels.Device{LastSeen: nil},
			want:   false,
		},
		{
			name:   "seen 3 minutes ago is online with default window",
			device: &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-3 * time.Minute))},
			want:   true,
		},
		{
			name:   "seen exactly at the default window is online (inclusive)",
			device: &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-defaultDeviceOnlineWindow))},
			want:   true,
		},
		{
			name:   "seen 10 minutes ago is offline with default window",
			device: &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-10 * time.Minute))},
			want:   false,
		},
		{
			name:     "tenant override widens the window",
			resolver: stubResolver{hasOverride: true, minutes: 15},
			device:   &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-10 * time.Minute))},
			want:     true,
		},
		{
			name:     "tenant override narrows the window",
			resolver: stubResolver{hasOverride: true, minutes: 2},
			device:   &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-3 * time.Minute))},
			want:     false,
		},
		{
			name:     "no override falls back to default window",
			resolver: stubResolver{hasOverride: false},
			device:   &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-4 * time.Minute))},
			want:     true,
		},
		{
			name:     "override-check error falls back to default window",
			resolver: stubResolver{hasOverrideErr: errors.New("boom")},
			device:   &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-4 * time.Minute))},
			want:     true,
		},
		{
			name:     "resolve error falls back to default window",
			resolver: stubResolver{hasOverride: true, resolveErr: errors.New("boom")},
			device:   &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-10 * time.Minute))},
			want:     false,
		},
		{
			name:     "non-positive override falls back to default window",
			resolver: stubResolver{hasOverride: true, minutes: 0},
			device:   &iotModels.Device{LastSeen: testpkg.TimePtr(now.Add(-10 * time.Minute))},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{}
			if tt.resolver != nil {
				svc.SetSettingsService(tt.resolver)
			}

			got := svc.IsDeviceOnlineAt(context.Background(), tt.device, now)
			if got != tt.want {
				t.Errorf("IsDeviceOnlineAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_deviceOnlineWindow_NilResolver(t *testing.T) {
	t.Parallel()

	svc := &service{}
	if got := svc.DeviceOnlineWindow(context.Background()); got != defaultDeviceOnlineWindow {
		t.Errorf("DeviceOnlineWindow() with nil resolver = %v, want %v", got, defaultDeviceOnlineWindow)
	}
}
