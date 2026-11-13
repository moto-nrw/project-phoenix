package behaviortest_test

import (
	"context"

	presence "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence"
)

// CheckinDeviceFault injects a lookup failure without replacing other device operations.
type CheckinDeviceFault struct {
	presence.SessionDeviceDirectory
	ReadErr error
}

func NewCheckinDeviceFault(directory presence.SessionDeviceDirectory) *CheckinDeviceFault {
	return &CheckinDeviceFault{SessionDeviceDirectory: directory}
}

func (r *CheckinDeviceFault) ManualAttendanceDeviceID(ctx context.Context) (int64, error) {
	if r.ReadErr != nil {
		return 0, r.ReadErr
	}
	return r.SessionDeviceDirectory.ManualAttendanceDeviceID(ctx)
}
