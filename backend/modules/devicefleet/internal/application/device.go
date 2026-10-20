package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// CreateDevice registers a device, minting an API key when the caller did not
// supply one. Uniqueness of device_id is decided by the table's constraint,
// so a concurrent registration cannot slip past an application-level check.
func (s *Service) CreateDevice(ctx context.Context, input devicefleet.CreateDevice) (devicefleet.Device, error) {
	var created devicefleet.Device
	err := s.runWrite(ctx, "create_device", func(ctx context.Context, stats *domain.OperationStats) error {
		if err := input.Validate(); err != nil {
			return err
		}
		if input.APIKey == nil || *input.APIKey == "" {
			key, mintErr := s.deps.APIKeys.Mint()
			if mintErr != nil {
				return fmt.Errorf("failed to generate API key: %w", mintErr)
			}
			input.APIKey = &key
		}
		stored, queryStats, err := s.deps.Devices.Create(ctx, input)
		stats.Add(queryStats)
		created = stored
		return err
	})
	return created, err
}

// UpdateDevice replaces one device's writable columns. A row that no longer
// matches reports ErrDeviceNotFound instead of a silent success.
func (s *Service) UpdateDevice(ctx context.Context, input devicefleet.UpdateDevice) (devicefleet.Device, error) {
	if input.ID <= 0 {
		return devicefleet.Device{}, s.reject("update_device", devicefleet.ErrInvalidDevice)
	}
	var updated devicefleet.Device
	err := s.runWrite(ctx, "update_device", func(ctx context.Context, stats *domain.OperationStats) error {
		if err := input.Validate(); err != nil {
			return err
		}
		stored, queryStats, err := s.deps.Devices.Update(ctx, toDomainUpdate(input))
		stats.Add(queryStats)
		updated = stored
		return err
	})
	return updated, err
}

// UpdateDeviceColumns writes exactly the named columns of one device. It is
// the seam device transfer uses to archive a source device atomically.
func (s *Service) UpdateDeviceColumns(ctx context.Context, input devicefleet.UpdateDevice, columns []string) (int64, error) {
	if input.ID <= 0 || len(columns) == 0 {
		return 0, s.reject("update_device_columns", devicefleet.ErrInvalidDevice)
	}
	storedInput := toDomainUpdate(input)
	var affected int64
	err := s.runWrite(ctx, "update_device_columns", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var err error
		affected, queryStats, err = s.deps.Devices.UpdateColumns(ctx, storedInput, columns)
		stats.Add(queryStats)
		return err
	})
	return affected, err
}

// DeleteDevice removes one device. A row that is already gone is not an
// error, matching the retired repository contract.
func (s *Service) DeleteDevice(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_device", devicefleet.ErrInvalidDevice)
	}
	return s.runWrite(ctx, "delete_device", func(ctx context.Context, stats *domain.OperationStats) error {
		_, queryStats, err := s.deps.Devices.Delete(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

// UpdateDeviceStatus sets the status of one live device by its device_id.
func (s *Service) UpdateDeviceStatus(ctx context.Context, deviceID string, status devicefleet.DeviceStatus) error {
	if deviceID == "" || !devicefleet.ValidDeviceStatus(status) {
		return s.reject("update_device_status", devicefleet.ErrInvalidDevice)
	}
	return s.runWrite(ctx, "update_device_status", func(ctx context.Context, stats *domain.OperationStats) error {
		if !devicefleet.ValidDeviceStatus(status) {
			return devicefleet.ErrInvalidDevice
		}
		affected, queryStats, err := s.deps.Devices.UpdateStatusByDeviceID(ctx, deviceID, status)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("devicefleet: expected one status update, got %d", affected)
		}
		return nil
	})
}

// UpdateDeviceLastSeen writes last_seen for one device addressed by its
// globally unique primary key, so a ping stays cross-tenant safe.
func (s *Service) UpdateDeviceLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	if id <= 0 {
		return s.reject("update_device_last_seen", devicefleet.ErrInvalidDevice)
	}
	return s.updateOneColumnSet(ctx, "update_device_last_seen",
		domain.UpdateDevice{ID: id, LastSeen: &lastSeen}, []string{"last_seen"})
}

// UpdateDeviceRoom moves one device into a room.
func (s *Service) UpdateDeviceRoom(ctx context.Context, id, roomID int64) error {
	if id <= 0 || roomID <= 0 {
		return s.reject("update_device_room", devicefleet.ErrInvalidDevice)
	}
	return s.updateOneColumnSet(ctx, "update_device_room",
		domain.UpdateDevice{ID: id, RoomID: &roomID, UpdatedAt: s.deps.Now()}, []string{"room_id", "updated_at"})
}

func (s *Service) updateOneColumnSet(ctx context.Context, operation string, input domain.UpdateDevice, columns []string) error {
	return s.runWrite(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		affected, queryStats, err := s.deps.Devices.UpdateColumns(ctx, input, columns)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("devicefleet: expected 1 rows affected, got %d", affected)
		}
		return nil
	})
}

// findDevice runs an observed lookup and attaches room names when requested.
func (s *Service) findDevice(
	ctx context.Context,
	operation string,
	withRoomName bool,
	read func(context.Context) (devicefleet.Device, bool, domain.OperationStats, error),
) (devicefleet.Device, error) {
	var device devicefleet.Device
	err := s.runInTenant(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		found, exists, queryStats, err := read(ctx)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !exists {
			return devicefleet.ErrDeviceNotFound
		}
		device = found
		if !withRoomName {
			return nil
		}
		devices := []devicefleet.Device{device}
		if err := s.attachRoomNames(ctx, devices); err != nil {
			return err
		}
		device = devices[0]
		return nil
	})
	return device, err
}

// ListDevices reads every live device matching filter, including room names.
func (s *Service) ListDevices(ctx context.Context, filter devicefleet.DeviceFilter) ([]devicefleet.Device, error) {
	if filter.Status != nil && !devicefleet.ValidDeviceStatus(*filter.Status) {
		return nil, s.reject("list_devices", devicefleet.ErrInvalidDevice)
	}
	var devices []devicefleet.Device
	err := s.runInTenant(ctx, "list_devices", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.List(ctx, filter)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if err := s.attachRoomNames(ctx, found); err != nil {
			return err
		}
		devices = found
		return nil
	})
	if devices == nil {
		devices = []devicefleet.Device{}
	}
	return devices, err
}

// ListOfflineDevices reads devices unseen for at least offlineSince.
func (s *Service) ListOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]devicefleet.Device, error) {
	if offlineSince <= 0 {
		return nil, s.reject("list_offline_devices", devicefleet.ErrInvalidDevice)
	}
	var devices []devicefleet.Device
	err := s.runInTenant(ctx, "list_offline_devices", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.ListOffline(ctx, s.deps.Now().Add(-offlineSince))
		stats.Add(queryStats)
		devices = found
		return err
	})
	if devices == nil {
		devices = []devicefleet.Device{}
	}
	return devices, err
}

// CountDevicesByTenant counts live devices grouped by tenant.
func (s *Service) CountDevicesByTenant(ctx context.Context) (map[int64]int, error) {
	var counts map[int64]int
	err := s.runInTenant(ctx, "count_devices_by_tenant", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.CountByTenant(ctx)
		stats.Add(queryStats)
		counts = found
		return err
	})
	return counts, err
}

// ListDevicesByTenants reads the live devices of the given tenants.
func (s *Service) ListDevicesByTenant(ctx context.Context, tenantIDs []int64) ([]devicefleet.Device, error) {
	for _, id := range tenantIDs {
		if id <= 0 {
			return nil, s.reject("list_devices_by_tenant", devicefleet.ErrInvalidDevice)
		}
	}
	var devices []devicefleet.Device
	err := s.runInTenant(ctx, "list_devices_by_tenant", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.ListByTenants(ctx, tenantIDs)
		stats.Add(queryStats)
		devices = found
		return err
	})
	if devices == nil {
		devices = []devicefleet.Device{}
	}
	return devices, err
}

// attachRoomNames fills Device.RoomName from the Facilities owner. The
// retired join also required the room to belong to the device's tenant, so a
// room of another tenant leaves the name unset exactly as before.
func (s *Service) attachRoomNames(ctx context.Context, devices []devicefleet.Device) error {
	ids := make([]int64, 0, len(devices))
	seen := make(map[int64]struct{}, len(devices))
	for _, device := range devices {
		if device.RoomID == nil || *device.RoomID <= 0 {
			continue
		}
		if _, found := seen[*device.RoomID]; found {
			continue
		}
		seen[*device.RoomID] = struct{}{}
		ids = append(ids, *device.RoomID)
	}
	if len(ids) == 0 {
		return nil
	}
	rooms, err := s.deps.Rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]domain.Room, len(rooms))
	for _, room := range rooms {
		byID[room.ID] = room
	}
	for index := range devices {
		if devices[index].RoomID == nil {
			continue
		}
		if room, ok := byID[*devices[index].RoomID]; ok && room.TenantID == devices[index].TenantID {
			name := room.Name
			devices[index].RoomName = &name
		}
	}
	return nil
}
