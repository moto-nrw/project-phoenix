package repositories

import (
	"context"
	"fmt"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/uptrace/bun"
)

// NewDeviceFleet composes the device owner behind the legacy composition seam
// for graphs that do not record observations and never serve the info-point
// dashboard. Production roots compose the module themselves (api/base.go) so
// runtime evidence and the dashboard collaborators are kept.
func NewDeviceFleet(db *bun.DB, onlineWindows ...func(context.Context) time.Duration) (devicefleet.Capability, error) {
	rooms, err := NewFacilities(db)
	if err != nil {
		return nil, err
	}
	var onlineWindow func(context.Context) time.Duration
	if len(onlineWindows) > 0 {
		onlineWindow = onlineWindows[0]
	}
	return devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB:           db,
		Rooms:        rooms,
		OnlineWindow: onlineWindow,
		Observe:      func(devicefleetCompose.Observation) {},
	})
}

// activeDeviceDirectory hands the device owner to the session repository that
// used to join iot.devices itself (#2676).
type activeDeviceDirectory struct{ devices devicefleet.Query }

func (d activeDeviceDirectory) ListDevicesByID(ctx context.Context, ids []int64) ([]activeRepo.DirectoryDevice, error) {
	devices, err := d.devices.ListDevicesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.DirectoryDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, activeRepo.DirectoryDevice{
			ID: device.ID, TenantID: device.TenantID, CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
			Name: device.Name, Status: string(device.Status), LastSeen: device.LastSeen,
		})
	}
	return result, nil
}

// auditDeviceDirectory hands the device owner to the audit repository that
// used to join iot.devices itself (#2676). Operator listings run without an
// ambient tenant, exactly as the retired cross-tenant join did.
type auditDeviceDirectory struct{ devices devicefleet.Query }

func (d auditDeviceDirectory) ListDevicesByID(ctx context.Context, ids []int64) ([]auditRepo.DirectoryDevice, error) {
	devices, err := d.devices.ListDevicesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]auditRepo.DirectoryDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, auditRepo.DirectoryDevice{
			ID: device.ID, TenantID: device.TenantID, DeviceID: device.DeviceID, Name: device.Name,
		})
	}
	return result, nil
}

func mustNewDeviceFleet(db *bun.DB) devicefleet.Capability {
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose device fleet: %v", err))
	}
	return fleet
}
