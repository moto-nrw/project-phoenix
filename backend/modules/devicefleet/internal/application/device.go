package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// CreateDevice registers a device, minting an API key when the caller did not
// supply one. Uniqueness of device_id is decided by the table's constraint,
// so a concurrent registration cannot slip past an application-level check.
func (s *Service) CreateDevice(ctx context.Context, input domain.CreateDevice) (domain.Device, error) {
	var created domain.Device
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
func (s *Service) UpdateDevice(ctx context.Context, input domain.UpdateDevice) (domain.Device, error) {
	var updated domain.Device
	err := s.runWrite(ctx, "update_device", func(ctx context.Context, stats *domain.OperationStats) error {
		if err := input.Validate(); err != nil {
			return err
		}
		stored, queryStats, err := s.deps.Devices.Update(ctx, input)
		stats.Add(queryStats)
		updated = stored
		return err
	})
	return updated, err
}

// UpdateDeviceColumns writes exactly the named columns of one device. It is
// the seam device transfer uses to archive a source device atomically.
func (s *Service) UpdateDeviceColumns(ctx context.Context, input domain.UpdateDevice, columns []string) (int64, error) {
	var affected int64
	err := s.runWrite(ctx, "update_device_columns", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var err error
		affected, queryStats, err = s.deps.Devices.UpdateColumns(ctx, input, columns)
		stats.Add(queryStats)
		return err
	})
	return affected, err
}

// DeleteDevice removes one device. A row that is already gone is not an
// error, matching the retired repository contract.
func (s *Service) DeleteDevice(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_device", func(ctx context.Context, stats *domain.OperationStats) error {
		_, queryStats, err := s.deps.Devices.Delete(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

// UpdateDeviceStatus sets the status of one live device by its device_id.
func (s *Service) UpdateDeviceStatus(ctx context.Context, deviceID string, status domain.DeviceStatus) error {
	return s.runWrite(ctx, "update_device_status", func(ctx context.Context, stats *domain.OperationStats) error {
		if !domain.ValidDeviceStatus(status) {
			return domain.ErrDeviceInvalid
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
	return s.updateOneColumnSet(ctx, "update_device_last_seen",
		domain.UpdateDevice{ID: id, LastSeen: &lastSeen}, []string{"last_seen"})
}

// UpdateDeviceRoom moves one device into a room.
func (s *Service) UpdateDeviceRoom(ctx context.Context, id, roomID int64) error {
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

// FindDevice reads one live device including its Facilities room name.
func (s *Service) FindDevice(ctx context.Context, id int64) (domain.Device, error) {
	return s.findDevice(ctx, "find_device", true, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
		return s.deps.Devices.FindByID(ctx, id, "")
	})
}

// FindDeviceForUpdate reads and locks one live device. The archived predicate
// is re-evaluated after the lock wait, so at most one transfer can archive a
// source device.
func (s *Service) FindDeviceForUpdate(ctx context.Context, id int64) (domain.Device, error) {
	return s.findDevice(ctx, "find_device_for_update", false, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
		return s.deps.Devices.FindByID(ctx, id, "UPDATE")
	})
}

// FindDeviceByDeviceID reads one live device by its tenant-unique device_id,
// including its room name.
func (s *Service) FindDeviceByDeviceID(ctx context.Context, deviceID string) (domain.Device, error) {
	return s.findDevice(ctx, "find_device_by_device_id", true, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
		return s.deps.Devices.FindByDeviceID(ctx, deviceID)
	})
}

// FindDeviceByAPIKey reads one live device by its API key. Device
// authentication calls it before a tenant is known.
func (s *Service) FindDeviceByAPIKey(ctx context.Context, apiKey string) (domain.Device, error) {
	return s.findDevice(ctx, "find_device_by_api_key", false, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
		return s.deps.Devices.FindByAPIKey(ctx, apiKey)
	})
}

func (s *Service) findDevice(
	ctx context.Context,
	operation string,
	withRoomName bool,
	read func(context.Context) (domain.Device, bool, domain.OperationStats, error),
) (domain.Device, error) {
	var device domain.Device
	err := s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		found, exists, queryStats, err := read(ctx)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrDeviceNotFound
		}
		device = found
		if !withRoomName {
			return nil
		}
		devices := []domain.Device{device}
		if err := s.attachRoomNames(ctx, devices); err != nil {
			return err
		}
		device = devices[0]
		return nil
	})
	return device, err
}

// ListDevices reads every live device matching filter, including room names.
func (s *Service) ListDevices(ctx context.Context, filter domain.DeviceFilter) ([]domain.Device, error) {
	var devices []domain.Device
	err := s.run(ctx, "list_devices", func(ctx context.Context, stats *domain.OperationStats) error {
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
	return devices, err
}

// ListDevicesByIDs reads the live devices for the supplied primary keys.
func (s *Service) ListDevicesByIDs(ctx context.Context, ids []int64) ([]domain.Device, error) {
	var devices []domain.Device
	err := s.run(ctx, "list_devices_by_id", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		devices = found
		return err
	})
	return devices, err
}

// ListOfflineDevices reads devices unseen for at least offlineSince.
func (s *Service) ListOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]domain.Device, error) {
	var devices []domain.Device
	err := s.run(ctx, "list_offline_devices", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.ListOffline(ctx, s.deps.Now().Add(-offlineSince))
		stats.Add(queryStats)
		devices = found
		return err
	})
	return devices, err
}

// CountDevicesByTenant counts live devices grouped by tenant.
func (s *Service) CountDevicesByTenant(ctx context.Context) (map[int64]int, error) {
	var counts map[int64]int
	err := s.run(ctx, "count_devices_by_tenant", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.CountByTenant(ctx)
		stats.Add(queryStats)
		counts = found
		return err
	})
	return counts, err
}

// ListDevicesByTenants reads the live devices of the given tenants.
func (s *Service) ListDevicesByTenants(ctx context.Context, tenantIDs []int64) ([]domain.Device, error) {
	var devices []domain.Device
	err := s.run(ctx, "list_devices_by_tenant", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.ListByTenants(ctx, tenantIDs)
		stats.Add(queryStats)
		devices = found
		return err
	})
	return devices, err
}

// CountDevicesByType counts live devices grouped by device_type.
func (s *Service) CountDevicesByType(ctx context.Context) (map[string]int, error) {
	var counts map[string]int
	err := s.run(ctx, "count_devices_by_type", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.CountByType(ctx)
		stats.Add(queryStats)
		counts = found
		return err
	})
	return counts, err
}

// attachRoomNames fills Device.RoomName from the Facilities owner. The
// retired join also required the room to belong to the device's tenant, so a
// room of another tenant leaves the name unset exactly as before.
func (s *Service) attachRoomNames(ctx context.Context, devices []domain.Device) error {
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
