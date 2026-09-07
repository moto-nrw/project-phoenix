package audit

import (
	"context"
	"errors"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// DirectoryDevice is the Device Fleet projection this package reads.
// iot.devices belongs to that owner (#2676); the composition root binds the
// directory behind DeviceDirectory instead of the retired SQL join.
type DirectoryDevice struct {
	ID       int64
	TenantID int64
	DeviceID string
	Name     *string
}

// DeviceDirectory is the owner query unregistered-tag scans resolve their
// scanning device through. Every method fails while unbound; there is no
// fallback join.
type DeviceDirectory interface {
	// ListDevicesByID returns the devices visible in the caller's
	// transaction. Missing IDs are absent, like the retired LEFT JOIN.
	ListDevicesByID(ctx context.Context, ids []int64) ([]DirectoryDevice, error)
}

var errDeviceDirectoryRequired = errors.New("audit repositories: device directory is not bound")

// attachDeviceIdentity fills the scan's device identifier and name from the
// owner. The retired join also required the device to belong to the scan's
// tenant, so a device of another tenant leaves both fields unset.
func attachDeviceIdentity(ctx context.Context, directory DeviceDirectory, scans []*auditModels.UnregisteredTagScan) error {
	if directory == nil {
		return errDeviceDirectoryRequired
	}
	ids := make([]int64, 0, len(scans))
	seen := make(map[int64]struct{}, len(scans))
	for _, scan := range scans {
		if scan == nil || scan.DeviceID == nil || *scan.DeviceID <= 0 {
			continue
		}
		if _, found := seen[*scan.DeviceID]; found {
			continue
		}
		seen[*scan.DeviceID] = struct{}{}
		ids = append(ids, *scan.DeviceID)
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
	for _, scan := range scans {
		if scan == nil || scan.DeviceID == nil {
			continue
		}
		device, ok := byID[*scan.DeviceID]
		if !ok || device.TenantID != scan.TenantID {
			continue
		}
		identifier := device.DeviceID
		scan.DeviceIdentifier = &identifier
		scan.DeviceName = device.Name
	}
	return nil
}
