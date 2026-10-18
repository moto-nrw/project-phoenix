package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// SessionDeviceIDs projects lookup results without exposing device records.
type SessionDeviceIDs struct {
	devicefleet.SessionDeviceRecords
}

func (d SessionDeviceIDs) ManualAttendanceDeviceID(ctx context.Context) (int64, error) {
	id, found, err := d.LookupSessionDevice(ctx, devicefleet.WebManualDeviceID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("%s is not configured", devicefleet.WebManualDeviceID)
	}
	return id, nil
}
