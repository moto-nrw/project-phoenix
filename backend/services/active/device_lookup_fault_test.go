package active

import (
	"context"
)

// CheckinDeviceFault injects a lookup failure without replacing other device operations.
type CheckinDeviceFault struct {
	SessionDeviceDirectory
	ReadErr error
}

func NewCheckinDeviceFault(directory SessionDeviceDirectory) *CheckinDeviceFault {
	return &CheckinDeviceFault{SessionDeviceDirectory: directory}
}

func (r *CheckinDeviceFault) ManualAttendanceDeviceID(ctx context.Context) (int64, error) {
	if r.ReadErr != nil {
		return 0, r.ReadErr
	}
	return r.SessionDeviceDirectory.ManualAttendanceDeviceID(ctx)
}
