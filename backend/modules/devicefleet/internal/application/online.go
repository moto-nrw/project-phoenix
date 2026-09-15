package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// DeviceOnlineWindow resolves the per-tenant online window. A missing
// resolver, a missing override, and a non-positive override all fall back to
// domain.DefaultDeviceOnlineWindow.
func (s *Service) DeviceOnlineWindow(ctx context.Context) time.Duration {
	if s.deps.Window == nil {
		return domain.DefaultDeviceOnlineWindow
	}
	if window := s.deps.Window(ctx); window > 0 {
		return window
	}
	return domain.DefaultDeviceOnlineWindow
}

// IsDeviceOnlineAt reports whether the device was online at the supplied
// observation time. A device is online when its last_seen timestamp lies
// within the resolved window of that moment. The row holds the timestamp;
// this owner holds the rule (#586, Rule 12).
func (s *Service) IsDeviceOnlineAt(ctx context.Context, device domain.Device, now time.Time) bool {
	if device.LastSeen == nil {
		return false
	}
	return now.Sub(*device.LastSeen) <= s.DeviceOnlineWindow(ctx)
}
