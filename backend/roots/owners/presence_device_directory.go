package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

// activeDeviceDirectory hands the device owner to the session repository that
// used to join iot.devices itself (#2676).
type activeDeviceDirectory struct{ devices devicefleet.Query }

func (d activeDeviceDirectory) ListDevicesByID(ctx context.Context, ids []int64) ([]presenceCompose.DirectoryDevice, error) {
	devices, err := d.devices.ListDevicesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.DirectoryDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, presenceCompose.DirectoryDevice{
			ID: device.ID, TenantID: device.TenantID, CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
			Name: device.Name, Status: string(device.Status), LastSeen: device.LastSeen,
		})
	}
	return result, nil
}
