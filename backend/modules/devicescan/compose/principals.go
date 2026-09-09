package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// principals reads the identities the device authentication bound to the
// request: the kiosk itself and, after a verified account PIN, the staff
// member operating it.
type principals struct{}

func (principals) Device(ctx context.Context) (*ports.Device, bool) {
	principal := device.DeviceFromCtx(ctx)
	if principal == nil {
		return nil, false
	}
	return &ports.Device{
		ID: principal.ID, DeviceID: principal.DeviceID, DeviceType: principal.DeviceType, Name: principal.Name,
		Status: principal.Status, LastSeen: principal.LastSeen, Active: principal.IsActive(),
	}, true
}

func (principals) Staff(ctx context.Context) (*ports.Staff, bool) {
	principal := device.StaffFromCtx(ctx)
	if principal == nil {
		return nil, false
	}
	return &ports.Staff{ID: principal.ID}, true
}
