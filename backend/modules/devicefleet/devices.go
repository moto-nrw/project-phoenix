package devicefleet

import (
	"context"
	"time"
)

// DeviceOnlineWindow resolves the per-tenant online window. A device seen
// within that window counts as online.
func (m *Module) DeviceOnlineWindow(ctx context.Context) time.Duration {
	return m.engine.DeviceOnlineWindow(ctx)
}

// IsDeviceOnline reports whether the device is online right now.
func (m *Module) IsDeviceOnline(ctx context.Context, device Device) bool {
	return m.engine.IsDeviceOnlineAt(ctx, device, m.engine.Now())
}

// IsDeviceOnlineAt reports whether the device was online at the supplied
// observation time.
func (m *Module) IsDeviceOnlineAt(ctx context.Context, device Device, now time.Time) bool {
	return m.engine.IsDeviceOnlineAt(ctx, device, now)
}

// FindDevice returns one live device including its Facilities room name.
func (m *Module) FindDevice(ctx context.Context, id int64) (Device, error) {
	if id <= 0 {
		return Device{}, m.reject("find_device", ErrInvalidDevice)
	}
	return m.engine.FindDevice(ctx, id)
}

// FindDeviceForUpdate returns one live device while holding an update lock
// until the caller's transaction ends. Device transfer uses it so at most one
// transfer can archive a source device.
func (m *Module) FindDeviceForUpdate(ctx context.Context, id int64) (Device, error) {
	if id <= 0 {
		return Device{}, m.reject("find_device_for_update", ErrInvalidDevice)
	}
	return m.engine.FindDeviceForUpdate(ctx, id)
}

// FindDeviceByDeviceID returns one live device by its tenant-unique
// device_id, including its room name.
func (m *Module) FindDeviceByDeviceID(ctx context.Context, deviceID string) (Device, error) {
	if deviceID == "" {
		return Device{}, m.reject("find_device_by_device_id", ErrInvalidDevice)
	}
	return m.engine.FindDeviceByDeviceID(ctx, deviceID)
}

// FindDeviceByAPIKey returns one live device by its API key. Device
// authentication calls it before any tenant is known.
func (m *Module) FindDeviceByAPIKey(ctx context.Context, apiKey string) (Device, error) {
	if apiKey == "" {
		return Device{}, m.reject("find_device_by_api_key", ErrInvalidDevice)
	}
	return m.engine.FindDeviceByAPIKey(ctx, apiKey)
}

// ListDevices returns every live device matching filter, including room names.
func (m *Module) ListDevices(ctx context.Context, filter DeviceFilter) ([]Device, error) {
	if filter.Status != nil && !ValidDeviceStatus(*filter.Status) {
		return nil, m.reject("list_devices", ErrInvalidDevice)
	}
	return m.engine.ListDevices(ctx, filter)
}

// ListDevicesByID returns the live devices visible for the given primary
// keys. Missing IDs are simply absent, like the retired LEFT JOIN.
func (m *Module) ListDevicesByID(ctx context.Context, ids []int64) ([]Device, error) {
	if len(ids) == 0 {
		return []Device{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, m.reject("list_devices_by_id", ErrInvalidDevice)
		}
	}
	return m.engine.ListDevicesByID(ctx, ids)
}

// ListOfflineDevices returns devices unseen for at least offlineSince.
func (m *Module) ListOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]Device, error) {
	if offlineSince <= 0 {
		return nil, m.reject("list_offline_devices", ErrInvalidDevice)
	}
	return m.engine.ListOfflineDevices(ctx, offlineSince)
}

// CountDevicesByType counts live devices grouped by device_type.
func (m *Module) CountDevicesByType(ctx context.Context) (map[string]int, error) {
	return m.engine.CountDevicesByType(ctx)
}

// CreateDevice registers a device, minting an API key when none was supplied.
func (m *Module) CreateDevice(ctx context.Context, input CreateDevice) (Device, error) {
	return m.engine.CreateDevice(ctx, input)
}

// UpdateDevice replaces one device's writable columns.
func (m *Module) UpdateDevice(ctx context.Context, input UpdateDevice) (Device, error) {
	if input.ID <= 0 {
		return Device{}, m.reject("update_device", ErrInvalidDevice)
	}
	return m.engine.UpdateDevice(ctx, input)
}

// UpdateDeviceColumns writes exactly the named columns of one device. It is
// the seam device transfer uses to archive a source device atomically.
func (m *Module) UpdateDeviceColumns(ctx context.Context, input UpdateDevice, columns []string) (int64, error) {
	if input.ID <= 0 || len(columns) == 0 {
		return 0, m.reject("update_device_columns", ErrInvalidDevice)
	}
	return m.engine.UpdateDeviceColumns(ctx, input, columns)
}

// DeleteDevice removes one device. The reserved web-manual device is
// protected.
func (m *Module) DeleteDevice(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_device", ErrInvalidDevice)
	}
	return m.engine.DeleteDevice(ctx, id)
}

// UpdateDeviceStatus sets the status of one live device by its device_id.
func (m *Module) UpdateDeviceStatus(ctx context.Context, deviceID string, status DeviceStatus) error {
	if deviceID == "" || !ValidDeviceStatus(status) {
		return m.reject("update_device_status", ErrInvalidDevice)
	}
	return m.engine.UpdateDeviceStatus(ctx, deviceID, status)
}

// UpdateDeviceLastSeen writes last_seen for one device addressed by its
// globally unique primary key, so a ping stays cross-tenant safe.
func (m *Module) UpdateDeviceLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	if id <= 0 {
		return m.reject("update_device_last_seen", ErrInvalidDevice)
	}
	return m.engine.UpdateDeviceLastSeen(ctx, id, lastSeen)
}

// UpdateDeviceRoom moves one device into a room.
func (m *Module) UpdateDeviceRoom(ctx context.Context, id, roomID int64) error {
	if id <= 0 || roomID <= 0 {
		return m.reject("update_device_room", ErrInvalidDevice)
	}
	return m.engine.UpdateDeviceRoom(ctx, id, roomID)
}
