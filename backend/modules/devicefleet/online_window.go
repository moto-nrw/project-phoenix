package devicefleet

import (
	"context"
	"time"
)

// OnlineWindowFromMinutes turns a resolver of the raw
// iot.device_online_window_minutes value into the device-online window. A
// non-positive value leaves the window unresolved (zero), and the consumer
// then applies its default window. A nil resolver stays nil.
func OnlineWindowFromMinutes(minutes func(context.Context) int) func(context.Context) time.Duration {
	if minutes == nil {
		return nil
	}
	return func(ctx context.Context) time.Duration {
		resolved := minutes(ctx)
		if resolved <= 0 {
			return 0
		}
		return time.Duration(resolved) * time.Minute
	}
}
