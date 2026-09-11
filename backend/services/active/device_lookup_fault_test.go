package active

import (
	"context"

	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
)

// CheckinDeviceFault injects a lookup failure without replacing other device operations.
type CheckinDeviceFault struct {
	iotModels.DeviceRepository
	ReadErr error
}

func NewCheckinDeviceFault(repo iotModels.DeviceRepository) *CheckinDeviceFault {
	return &CheckinDeviceFault{DeviceRepository: repo}
}

func (r *CheckinDeviceFault) FindByDeviceID(ctx context.Context, code string) (*iotModels.Device, error) {
	if r.ReadErr != nil {
		return nil, r.ReadErr
	}
	return r.DeviceRepository.FindByDeviceID(ctx, code)
}
