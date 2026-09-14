package legacy

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// SessionDeviceRecords is the legacy device input used by attendance composition.
type SessionDeviceRecords interface {
	FindByDeviceID(context.Context, string) (*iot.Device, error)
	UpdateRoomID(context.Context, int64, int64) error
}

// SessionDeviceIDs projects legacy lookup results without exposing device records.
type SessionDeviceIDs struct{ SessionDeviceRecords }

func (d SessionDeviceIDs) ManualAttendanceDeviceID(ctx context.Context) (int64, error) {
	device, err := d.FindByDeviceID(ctx, devicefleet.WebManualDeviceID)
	if err != nil {
		return 0, err
	}
	if device == nil {
		return 0, fmt.Errorf("%s is not configured", devicefleet.WebManualDeviceID)
	}
	return device.ID, nil
}
