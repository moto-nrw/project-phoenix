package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

// NewEngine serves the Device Fleet capability from the application service;
// observe receives the rejected requests and now is the fleet clock.
func NewEngine(service *Service, observe ports.Observer, now func() time.Time) Engine {
	return Engine{service: service, observe: observe, now: now}
}

// Engine is the Device Fleet capability over the application service.
type Engine struct {
	service *Service
	observe ports.Observer
	now     func() time.Time
}

func (e Engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e Engine) Now() time.Time { return e.now() }

func (e Engine) DeviceOnlineWindow(ctx context.Context) time.Duration {
	return e.service.DeviceOnlineWindow(ctx)
}

func (e Engine) IsDeviceOnlineAt(ctx context.Context, device devicefleet.Device, now time.Time) bool {
	return e.service.IsDeviceOnlineAt(ctx, toDomainDevice(device), now)
}

func toDomainDevice(device devicefleet.Device) domain.Device {
	return domain.Device{
		ID: device.ID, TenantID: device.TenantID, DeviceID: device.DeviceID,
		Status: domain.DeviceStatus(device.Status), LastSeen: device.LastSeen,
	}
}

func (e Engine) FindDevice(ctx context.Context, id int64) (devicefleet.Device, error) {
	value, err := e.service.findDevice(ctx, "find_device", true, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
		return e.service.deps.Devices.FindByID(ctx, id, "")
	})

	return toPublicDevice(value), MapError(err)
}

func (e Engine) FindDeviceForUpdate(ctx context.Context, id int64) (devicefleet.Device, error) {
	var value domain.Device
	err := e.service.run(ctx, "find_device_for_update", func(ctx context.Context, stats *domain.OperationStats) error {
		found, exists, queries, err := e.service.deps.Devices.FindByID(ctx, id, "UPDATE")
		stats.Add(queries)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrDeviceNotFound
		}
		value = found
		return nil
	})
	return toPublicDevice(value), MapError(err)
}

func (e Engine) FindDeviceByDeviceID(ctx context.Context, deviceID string) (devicefleet.Device, error) {
	value, err := e.service.findDevice(ctx, "find_device_by_device_id", true, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
		return e.service.deps.Devices.FindByDeviceID(ctx, deviceID)
	})

	return toPublicDevice(value), MapError(err)
}

func (e Engine) FindDeviceByAPIKey(ctx context.Context, apiKey string) (devicefleet.Device, error) {
	var value domain.Device
	err := e.service.deps.Tx.AdminScope(ctx, func(ctx context.Context) error {
		var err error
		value, err = e.service.findDevice(ctx, "find_device_by_api_key", false, func(ctx context.Context) (domain.Device, bool, domain.OperationStats, error) {
			return e.service.deps.Devices.FindByAPIKey(ctx, apiKey)
		})

		return err
	})
	return toPublicDevice(value), MapError(err)
}

func (e Engine) ListDevices(ctx context.Context, filter devicefleet.DeviceFilter) ([]devicefleet.Device, error) {
	values, err := e.service.ListDevices(ctx, toDomainFilter(filter))
	return toPublicDevices(values), MapError(err)
}

func (e Engine) ListDevicesByID(ctx context.Context, ids []int64) ([]devicefleet.Device, error) {
	var devices []domain.Device
	err := e.service.run(ctx, "list_devices_by_id", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := e.service.deps.Devices.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		devices = found
		return err
	})
	return toPublicDevices(devices), MapError(err)
}

func (e Engine) ListOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]devicefleet.Device, error) {
	values, err := e.service.ListOfflineDevices(ctx, offlineSince)
	return toPublicDevices(values), MapError(err)
}

func (e Engine) CountDevicesByType(ctx context.Context) (map[string]int, error) {
	var counts map[string]int
	err := e.service.runInTenant(ctx, "count_devices_by_type", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := e.service.deps.Devices.CountByType(ctx)
		stats.Add(queryStats)
		counts = found
		return err
	})
	return counts, MapError(err)
}

func (e Engine) CountDevicesByTenant(ctx context.Context) (map[int64]int, error) {
	counts, err := e.service.CountDevicesByTenant(ctx)
	return counts, MapError(err)
}

func (e Engine) ListDevicesByTenant(ctx context.Context, tenantIDs []int64) ([]devicefleet.Device, error) {
	values, err := e.service.ListDevicesByTenants(ctx, tenantIDs)
	return toPublicDevices(values), MapError(err)
}

func (e Engine) CreateDevice(ctx context.Context, input devicefleet.CreateDevice) (devicefleet.Device, error) {
	value, err := e.service.CreateDevice(ctx, domain.CreateDevice{
		DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: domain.DeviceStatus(input.Status), APIKey: input.APIKey, LastSeen: input.LastSeen,
		RegisteredByID: input.RegisteredByID, RoomID: input.RoomID,
	})
	return toPublicDevice(value), MapError(err)
}

func (e Engine) UpdateDevice(ctx context.Context, input devicefleet.UpdateDevice) (devicefleet.Device, error) {
	value, err := e.service.UpdateDevice(ctx, toDomainUpdate(input))
	return toPublicDevice(value), MapError(err)
}

func (e Engine) UpdateDeviceColumns(ctx context.Context, input devicefleet.UpdateDevice, columns []string) (int64, error) {
	affected, err := e.service.UpdateDeviceColumns(ctx, toDomainUpdate(input), columns)
	return affected, MapError(err)
}

func (e Engine) DeleteDevice(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteDevice(ctx, id))
}

func (e Engine) UpdateDeviceStatus(ctx context.Context, deviceID string, status devicefleet.DeviceStatus) error {
	return MapError(e.service.UpdateDeviceStatus(ctx, deviceID, domain.DeviceStatus(status)))
}

func (e Engine) UpdateDeviceLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	return MapError(e.service.UpdateDeviceLastSeen(ctx, id, lastSeen))
}

func (e Engine) UpdateDeviceRoom(ctx context.Context, id, roomID int64) error {
	return MapError(e.service.UpdateDeviceRoom(ctx, id, roomID))
}

func (e Engine) ListDisplays(ctx context.Context) ([]devicefleet.Display, error) {
	values, err := e.service.ListDisplays(ctx)
	result := make([]devicefleet.Display, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicDisplay(value))
	}
	return result, MapError(err)
}

func (e Engine) CreateDisplay(ctx context.Context, name string) (devicefleet.Display, string, error) {
	value, rawToken, err := e.service.CreateDisplay(ctx, name)
	return toPublicDisplay(value), rawToken, MapError(err)
}

func (e Engine) UpdateDisplay(ctx context.Context, id int64, name *string, isActive *bool) (devicefleet.Display, error) {
	value, err := e.service.UpdateDisplay(ctx, id, name, isActive)
	return toPublicDisplay(value), MapError(err)
}

func (e Engine) RegenerateDisplayToken(ctx context.Context, id int64) (string, error) {
	_, rawToken, err := e.service.RegenerateDisplayToken(ctx, id)
	return rawToken, MapError(err)
}

func (e Engine) DeleteDisplay(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteDisplay(ctx, id))
}

func (e Engine) Dashboard(ctx context.Context, rawToken string) (devicefleet.Dashboard, error) {
	value, err := e.service.ResolveDashboard(ctx, rawToken)
	if err != nil {
		return devicefleet.Dashboard{}, MapError(err)
	}
	return toPublicDashboard(value), nil
}

func (e Engine) FindUnregisteredTagScan(ctx context.Context, id int64) (devicefleet.UnregisteredTagScan, error) {
	value, err := e.service.FindUnregisteredTagScan(ctx, id)
	return toPublicUnregisteredTagScan(value), MapError(err)
}

func (e Engine) ListUnregisteredTagScans(ctx context.Context, filter devicefleet.UnregisteredTagScanFilter) ([]devicefleet.UnregisteredTagScan, error) {
	values, err := e.service.ListUnregisteredTagScans(ctx, domain.UnregisteredTagScanFilter{
		TenantIDs: filter.TenantIDs, UnresolvedOnly: filter.UnresolvedOnly, Limit: filter.Limit,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]devicefleet.UnregisteredTagScan, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicUnregisteredTagScan(value))
	}
	return result, nil
}

func (e Engine) RecordUnregisteredTagScan(ctx context.Context, input devicefleet.RecordUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error) {
	value, err := e.service.RecordUnregisteredTagScan(ctx, domain.RecordUnregisteredTagScan{
		TagUID: input.TagUID, DeviceID: input.DeviceID, ScannedAt: input.ScannedAt,
	})
	return toPublicUnregisteredTagScan(value), MapError(err)
}

func (e Engine) ResolveUnregisteredTagScan(ctx context.Context, input devicefleet.ResolveUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error) {
	value, err := e.service.ResolveUnregisteredTagScan(ctx, domain.ResolveUnregisteredTagScan{
		ID: input.ID, OperatorID: input.OperatorID, Note: input.Note,
	})
	return toPublicUnregisteredTagScan(value), MapError(err)
}

func (e Engine) DeleteExpiredUnregisteredTagScans(ctx context.Context, cutoff time.Time) (int64, error) {
	deleted, err := e.service.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
	return deleted, MapError(err)
}

func toPublicUnregisteredTagScan(value domain.UnregisteredTagScan) devicefleet.UnregisteredTagScan {
	return devicefleet.UnregisteredTagScan{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		TagUID: value.TagUID, DeviceID: value.DeviceID, ScannedAt: value.ScannedAt, ResolvedAt: value.ResolvedAt,
		ResolvedByOperatorID: value.ResolvedByOperatorID, ResolutionNote: value.ResolutionNote,
		DeviceIdentifier: value.DeviceIdentifier, DeviceName: value.DeviceName,
	}
}

func toDomainFilter(filter devicefleet.DeviceFilter) domain.DeviceFilter {
	result := domain.DeviceFilter{
		DeviceIDContains: filter.DeviceIDContains, NameContains: filter.NameContains,
		DeviceType: filter.DeviceType, ExcludeDeviceType: filter.ExcludeDeviceType,
		ExcludeDeviceID: filter.ExcludeDeviceID, SeenAfter: filter.SeenAfter,
		SeenBefore: filter.SeenBefore, RoomID: filter.RoomID,
		RegisteredByID: filter.RegisteredByID, HasName: filter.HasName,
	}
	if filter.Status != nil {
		status := domain.DeviceStatus(*filter.Status)
		result.Status = &status
	}
	return result
}

func toDomainUpdate(input devicefleet.UpdateDevice) domain.UpdateDevice {
	return domain.UpdateDevice{
		ID: input.ID, DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: domain.DeviceStatus(input.Status), APIKey: input.APIKey, LastSeen: input.LastSeen,
		RegisteredByID: input.RegisteredByID, RoomID: input.RoomID, ArchivedAt: input.ArchivedAt,
		TransferredToDeviceID: input.TransferredToDeviceID,
	}
}

func toPublicDevices(values []domain.Device) []devicefleet.Device {
	result := make([]devicefleet.Device, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicDevice(value))
	}
	return result
}

func toPublicDevice(value domain.Device) devicefleet.Device {
	return devicefleet.Device{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		DeviceID: value.DeviceID, DeviceType: value.DeviceType, Name: value.Name,
		Status: devicefleet.DeviceStatus(value.Status), APIKey: value.APIKey, LastSeen: value.LastSeen,
		RegisteredByID: value.RegisteredByID, RoomID: value.RoomID, ArchivedAt: value.ArchivedAt,
		TransferredToDeviceID: value.TransferredToDeviceID, RoomName: value.RoomName,
	}
}

func toPublicDisplay(value domain.Display) devicefleet.Display {
	return devicefleet.Display{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt, Name: value.Name, IsActive: value.IsActive,
	}
}

func toPublicDashboard(value Dashboard) devicefleet.Dashboard {
	dashboard := devicefleet.Dashboard{
		Status: value.Status, SchoolName: value.SchoolName, DisplayName: value.DisplayName,
		ServerTime: value.ServerTime, Date: value.Date,
		RoomOccupancy:      make([]devicefleet.RoomOccupancy, 0, len(value.RoomOccupancy)),
		RunningActivities:  make([]devicefleet.RunningActivity, 0, len(value.RunningActivities)),
		UpcomingActivities: make([]devicefleet.UpcomingActivity, 0, len(value.UpcomingActivities)),
		PickupTimes:        make([]devicefleet.PickupBucket, 0, len(value.PickupTimes)),
		StudentsPresent:    value.StudentsPresent, RoomsOccupied: value.RoomsOccupied,
		ActivitiesRunning: value.ActivitiesRunning,
	}
	for _, room := range value.RoomOccupancy {
		dashboard.RoomOccupancy = append(dashboard.RoomOccupancy, devicefleet.RoomOccupancy{
			Name: room.Name, GroupName: room.GroupName, CategoryName: room.CategoryName,
			StudentCount: room.StudentCount, Capacity: room.Capacity, IsOccupied: room.IsOccupied,
		})
	}
	for _, running := range value.RunningActivities {
		dashboard.RunningActivities = append(dashboard.RunningActivities, devicefleet.RunningActivity{
			ID: running.ID, Name: running.Name, Category: running.Category, RoomName: running.RoomName,
			Participants: running.Participants, MaxCapacity: running.MaxCapacity,
		})
	}
	for _, upcoming := range value.UpcomingActivities {
		dashboard.UpcomingActivities = append(dashboard.UpcomingActivities, devicefleet.UpcomingActivity{
			ID: upcoming.ID, Name: upcoming.Name, Category: upcoming.Category,
			StartTime: upcoming.StartTime, RoomName: upcoming.RoomName,
		})
	}
	for _, bucket := range value.PickupTimes {
		dashboard.PickupTimes = append(dashboard.PickupTimes, devicefleet.PickupBucket{
			Time: bucket.Time, Count: bucket.Count,
		})
	}
	return dashboard
}

// mapError translates internal sentinels to the stable errors callers match.
func MapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrDeviceNotFound):
		return devicefleet.ErrDeviceNotFound
	case errors.Is(err, domain.ErrDeviceDuplicate):
		return devicefleet.ErrDuplicateDeviceID
	case errors.Is(err, domain.ErrDeviceProtected):
		return devicefleet.ErrDeviceProtected
	case errors.Is(err, domain.ErrDeviceInvalid):
		return devicefleet.ErrInvalidDevice
	case errors.Is(err, domain.ErrDisplayNotFound):
		return devicefleet.ErrDisplayNotFound
	case errors.Is(err, domain.ErrDisplayInactive):
		return devicefleet.ErrDisplayInactive
	case errors.Is(err, domain.ErrDisplayInvalid):
		invalid := &domain.InvalidDisplayError{}
		if errors.As(err, &invalid) {
			return &devicefleet.InvalidDisplayError{Reason: invalid.Reason}
		}
		return devicefleet.ErrInvalidDisplayInput
	case errors.Is(err, domain.ErrUnregisteredTagScanNotFound):
		return devicefleet.ErrUnregisteredTagScanNotFound
	case errors.Is(err, domain.ErrUnregisteredTagScanResolved):
		return devicefleet.ErrUnregisteredTagScanResolved
	case errors.Is(err, domain.ErrUnregisteredTagScanInvalid):
		return devicefleet.ErrInvalidUnregisteredTagScan
	case errors.Is(err, domain.ErrTenantRequired):
		return devicefleet.ErrTenantRequired
	default:
		return err
	}
}
