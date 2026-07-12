package shared

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/device"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
)

// RecordUnregisteredTagScan best-effort persists a scan of an RFID tag that
// resolves to no person, stamped with the device from context. Nil service
// (not wired in tests) is a no-op; a persistence error only logs — the scan
// response to the kiosk must not fail because bookkeeping did.
func RecordUnregisteredTagScan(ctx context.Context, scans auditSvc.UnregisteredTagScanService, logger *slog.Logger, rfid string) {
	if scans == nil {
		return
	}
	var deviceID *int64
	if deviceCtx := device.DeviceFromCtx(ctx); deviceCtx != nil && deviceCtx.ID > 0 {
		id := deviceCtx.ID
		deviceID = &id
	}
	if err := scans.Record(ctx, rfid, deviceID); err != nil {
		logger.ErrorContext(ctx, "failed to record unregistered RFID scan",
			slog.String("rfid", rfid),
			slog.String("error", err.Error()),
		)
	}
}
