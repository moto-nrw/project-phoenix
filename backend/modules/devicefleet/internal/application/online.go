package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// DeviceOnlineWindow applies the resolved tenant override or registry default.
// A missing resolver or a non-positive resolved window uses
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
func (s *Service) IsDeviceOnlineAt(ctx context.Context, device devicefleet.Device, now time.Time) bool {
	if device.LastSeen == nil {
		return false
	}
	return now.Sub(*device.LastSeen) <= s.DeviceOnlineWindow(ctx)
}
