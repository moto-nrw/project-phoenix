package active

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
)

// DirectoryDevice is the Device Fleet projection this package reads.
// iot.devices belongs to that owner (#2676); the composition root binds the
// directory behind DeviceDirectory instead of the retired SQL join.
type DirectoryDevice struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeviceID   string
	DeviceType string
	Name       *string
	Status     string
	LastSeen   *time.Time
}

// DeviceDirectory is the owner query session reads resolve devices through.
// Every method fails while unbound; there is no fallback join.
type DeviceDirectory interface {
	// ListDevicesByID returns the devices visible in the caller's
	// transaction. Missing IDs are absent, like the retired LEFT JOIN.
	ListDevicesByID(ctx context.Context, ids []int64) ([]DirectoryDevice, error)
}

var errDeviceDirectoryRequired = errors.New("active repositories: device directory is not bound")

// attachDevices fills Group.Device from the owner. The retired join also
// required the device to belong to the session's tenant, so a device of
// another tenant leaves the relation unset exactly as before.
func attachDevices(ctx context.Context, directory DeviceDirectory, groups []*active.Group) error {
	if directory == nil {
		return errDeviceDirectoryRequired
	}
	ids := make([]int64, 0, len(groups))
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group == nil || group.DeviceID == nil || *group.DeviceID <= 0 {
			continue
		}
		if _, found := seen[*group.DeviceID]; found {
			continue
		}
		seen[*group.DeviceID] = struct{}{}
		ids = append(ids, *group.DeviceID)
	}
	if len(ids) == 0 {
		return nil
	}
	devices, err := directory.ListDevicesByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]DirectoryDevice, len(devices))
	for _, device := range devices {
		byID[device.ID] = device
	}
	for _, group := range groups {
		if group == nil || group.DeviceID == nil {
			continue
		}
		device, ok := byID[*group.DeviceID]
		if !ok || device.TenantID != group.TenantID {
			continue
		}
		group.Device = &iot.Device{
			ID: device.ID, CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt,
			TenantID: device.TenantID, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
			Name: device.Name, Status: iot.DeviceStatus(device.Status), LastSeen: device.LastSeen,
		}
	}
	return nil
}
