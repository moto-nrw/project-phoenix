package data

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/device"
)

func (rs *Resource) recordUnregisteredTagScan(ctx context.Context, rfid string) {
	if rs.UnregisteredTagScans == nil {
		return
	}
	var deviceID *int64
	if deviceCtx := device.DeviceFromCtx(ctx); deviceCtx != nil && deviceCtx.ID > 0 {
		id := deviceCtx.ID
		deviceID = &id
	}
	if err := rs.UnregisteredTagScans.Record(ctx, rfid, deviceID); err != nil {
		slog.ErrorContext(ctx, "failed to record unregistered RFID scan",
			slog.String("rfid", rfid),
			slog.String("error", err.Error()),
		)
	}
}
