package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

func timePtr(value time.Time) *time.Time { return &value }

func fixedWindow(window time.Duration) ports.WindowResolver {
	return func(context.Context) time.Duration { return window }
}

func TestServiceIsDeviceOnlineAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		// window nil means no resolver injected, so the default applies.
		window ports.WindowResolver
		device domain.Device
		want   bool
	}{
		{
			name:   "nil LastSeen is offline",
			device: domain.Device{LastSeen: nil},
			want:   false,
		},
		{
			name:   "seen 3 minutes ago is online with default window",
			device: domain.Device{LastSeen: timePtr(now.Add(-3 * time.Minute))},
			want:   true,
		},
		{
			name:   "seen exactly at the default window is online (inclusive)",
			device: domain.Device{LastSeen: timePtr(now.Add(-domain.DefaultDeviceOnlineWindow))},
			want:   true,
		},
		{
			name:   "seen 10 minutes ago is offline with default window",
			device: domain.Device{LastSeen: timePtr(now.Add(-10 * time.Minute))},
			want:   false,
		},
		{
			name:   "tenant override widens the window",
			window: fixedWindow(15 * time.Minute),
			device: domain.Device{LastSeen: timePtr(now.Add(-10 * time.Minute))},
			want:   true,
		},
		{
			name:   "tenant override narrows the window",
			window: fixedWindow(2 * time.Minute),
			device: domain.Device{LastSeen: timePtr(now.Add(-3 * time.Minute))},
			want:   false,
		},
		{
			name:   "unresolved override falls back to the default window",
			window: fixedWindow(0),
			device: domain.Device{LastSeen: timePtr(now.Add(-4 * time.Minute))},
			want:   true,
		},
		{
			name:   "unresolved override still rejects a stale device",
			window: fixedWindow(0),
			device: domain.Device{LastSeen: timePtr(now.Add(-10 * time.Minute))},
			want:   false,
		},
		{
			name:   "negative override falls back to the default window",
			window: fixedWindow(-time.Minute),
			device: domain.Device{LastSeen: timePtr(now.Add(-10 * time.Minute))},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := &Service{deps: Dependencies{Window: tt.window}}
			if got := service.IsDeviceOnlineAt(context.Background(), tt.device, now); got != tt.want {
				t.Errorf("IsDeviceOnlineAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceDeviceOnlineWindowFallsBackWithoutResolver(t *testing.T) {
	t.Parallel()

	service := &Service{deps: Dependencies{}}
	if got := service.DeviceOnlineWindow(context.Background()); got != domain.DefaultDeviceOnlineWindow {
		t.Errorf("DeviceOnlineWindow() without resolver = %v, want %v", got, domain.DefaultDeviceOnlineWindow)
	}
}
