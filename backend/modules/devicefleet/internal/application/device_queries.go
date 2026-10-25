package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

func (s *Service) FindDevice(ctx context.Context, id int64) (devicefleet.Device, error) {
	if id <= 0 {
		return devicefleet.Device{}, s.reject("find_device", devicefleet.ErrInvalidDevice)
	}
	value, err := s.findDevice(ctx, "find_device", true, func(ctx context.Context) (devicefleet.Device, bool, domain.OperationStats, error) {
		return s.deps.Devices.FindByID(ctx, id, "")
	})

	return value, err
}

func (s *Service) FindDeviceForUpdate(ctx context.Context, id int64) (devicefleet.Device, error) {
	if id <= 0 {
		return devicefleet.Device{}, s.reject("find_device_for_update", devicefleet.ErrInvalidDevice)
	}
	var value devicefleet.Device
	err := s.run(ctx, "find_device_for_update", func(ctx context.Context, stats *domain.OperationStats) error {
		found, exists, queries, err := s.deps.Devices.FindByID(ctx, id, "UPDATE")
		stats.Add(queries)
		if err != nil {
			return err
		}
		if !exists {
			return devicefleet.ErrDeviceNotFound
		}
		value = found
		return nil
	})
	return value, err
}

func (s *Service) FindDeviceByDeviceID(ctx context.Context, deviceID string) (devicefleet.Device, error) {
	if deviceID == "" {
		return devicefleet.Device{}, s.reject("find_device_by_device_id", devicefleet.ErrInvalidDevice)
	}
	value, err := s.findDevice(ctx, "find_device_by_device_id", true, func(ctx context.Context) (devicefleet.Device, bool, domain.OperationStats, error) {
		return s.deps.Devices.FindByDeviceID(ctx, deviceID)
	})

	return value, err
}

func (s *Service) FindDeviceByAPIKey(ctx context.Context, apiKey string) (devicefleet.Device, error) {
	if apiKey == "" {
		return devicefleet.Device{}, s.reject("find_device_by_api_key", devicefleet.ErrInvalidDevice)
	}
	var value devicefleet.Device
	err := s.deps.Tx.AdminScope(ctx, func(ctx context.Context) error {
		var err error
		value, err = s.findDevice(ctx, "find_device_by_api_key", false, func(ctx context.Context) (devicefleet.Device, bool, domain.OperationStats, error) {
			return s.deps.Devices.FindByAPIKey(ctx, s.deps.APIKeys.Hash(apiKey))
		})

		return err
	})
	return value, publicError(err)
}

func (s *Service) ListDevicesByID(ctx context.Context, ids []int64) ([]devicefleet.Device, error) {
	if len(ids) == 0 {
		return []devicefleet.Device{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, s.reject("list_devices_by_id", devicefleet.ErrInvalidDevice)
		}
	}
	var devices []devicefleet.Device
	err := s.run(ctx, "list_devices_by_id", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		devices = found
		return err
	})
	if devices == nil {
		devices = []devicefleet.Device{}
	}
	return devices, err
}

func (s *Service) CountDevicesByType(ctx context.Context) (map[string]int, error) {
	var counts map[string]int
	err := s.runInTenant(ctx, "count_devices_by_type", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Devices.CountByType(ctx)
		stats.Add(queryStats)
		counts = found
		return err
	})
	return counts, err
}

func (s *Service) IsDeviceOnline(ctx context.Context, device devicefleet.Device) bool {
	return s.IsDeviceOnlineAt(ctx, device, s.deps.Now())
}

// retentionClockMargin keeps the cutoff inside the database retention window
// when the database clock runs slightly behind the owner clock.
const retentionClockMargin = time.Minute

// unregisteredTagScanRetentionWindow is the review window as elapsed time.
// The audit retention function evaluates its 90 days in UTC (migration
// 1.15.534), so neither side lengthens a day across a clock change.
const unregisteredTagScanRetentionWindow = time.Duration(devicefleet.UnregisteredTagScanRetentionDays) * 24 * time.Hour

// DeleteUnregisteredTagScansAfterRetention removes the caller tenant's scans
// after the fixed review window.
func (s *Service) DeleteUnregisteredTagScansAfterRetention(ctx context.Context) (int, int, error) {
	cutoff := s.deps.Now().Add(-unregisteredTagScanRetentionWindow - retentionClockMargin)
	deleted, err := s.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
	return int(deleted), devicefleet.UnregisteredTagScanRetentionDays, err
}

func toDomainUpdate(input devicefleet.UpdateDevice) domain.UpdateDevice {
	return domain.UpdateDevice{
		ID: input.ID, DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: string(input.Status), APIKeyHash: input.APIKeyHash, MaskedAPIKey: input.MaskedAPIKey, LastSeen: input.LastSeen,
		RegisteredByID: input.RegisteredByID, RoomID: input.RoomID, ArchivedAt: input.ArchivedAt,
		TransferredToDeviceID: input.TransferredToDeviceID,
	}
}

func toPublicDisplay(value domain.Display) devicefleet.Display {
	return devicefleet.Display{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt, Name: value.Name, IsActive: value.IsActive,
	}
}

var _ devicefleet.Capability = (*Service)(nil)
