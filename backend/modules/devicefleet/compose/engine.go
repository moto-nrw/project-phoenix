package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

type engine struct {
	service *application.Service
	observe ports.Observer
	now     func() time.Time
}

func (e engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e engine) Now() time.Time { return e.now() }

func (e engine) DeviceOnlineWindow(ctx context.Context) time.Duration {
	return e.service.DeviceOnlineWindow(ctx)
}

func (e engine) IsDeviceOnlineAt(ctx context.Context, device devicefleet.Device, now time.Time) bool {
	return e.service.IsDeviceOnlineAt(ctx, toDomainDevice(device), now)
}

func toDomainDevice(device devicefleet.Device) domain.Device {
	return domain.Device{
		ID: device.ID, TenantID: device.TenantID, DeviceID: device.DeviceID,
		Status: domain.DeviceStatus(device.Status), LastSeen: device.LastSeen,
	}
}

func (e engine) FindDevice(ctx context.Context, id int64) (devicefleet.Device, error) {
	value, err := e.service.FindDevice(ctx, id)
	return toPublicDevice(value), mapError(err)
}

func (e engine) FindDeviceForUpdate(ctx context.Context, id int64) (devicefleet.Device, error) {
	value, err := e.service.FindDeviceForUpdate(ctx, id)
	return toPublicDevice(value), mapError(err)
}

func (e engine) FindDeviceByDeviceID(ctx context.Context, deviceID string) (devicefleet.Device, error) {
	value, err := e.service.FindDeviceByDeviceID(ctx, deviceID)
	return toPublicDevice(value), mapError(err)
}

func (e engine) FindDeviceByAPIKey(ctx context.Context, apiKey string) (devicefleet.Device, error) {
	value, err := e.service.FindDeviceByAPIKey(ctx, apiKey)
	return toPublicDevice(value), mapError(err)
}

func (e engine) ListDevices(ctx context.Context, filter devicefleet.DeviceFilter) ([]devicefleet.Device, error) {
	values, err := e.service.ListDevices(ctx, toDomainFilter(filter))
	return toPublicDevices(values), mapError(err)
}

func (e engine) ListDevicesByID(ctx context.Context, ids []int64) ([]devicefleet.Device, error) {
	values, err := e.service.ListDevicesByIDs(ctx, ids)
	return toPublicDevices(values), mapError(err)
}

func (e engine) ListOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]devicefleet.Device, error) {
	values, err := e.service.ListOfflineDevices(ctx, offlineSince)
	return toPublicDevices(values), mapError(err)
}

func (e engine) CountDevicesByType(ctx context.Context) (map[string]int, error) {
	counts, err := e.service.CountDevicesByType(ctx)
	return counts, mapError(err)
}

func (e engine) CountDevicesByTenant(ctx context.Context) (map[int64]int, error) {
	counts, err := e.service.CountDevicesByTenant(ctx)
	return counts, mapError(err)
}

func (e engine) ListDevicesByTenant(ctx context.Context, tenantIDs []int64) ([]devicefleet.Device, error) {
	values, err := e.service.ListDevicesByTenants(ctx, tenantIDs)
	return toPublicDevices(values), mapError(err)
}

func (e engine) CreateDevice(ctx context.Context, input devicefleet.CreateDevice) (devicefleet.Device, error) {
	value, err := e.service.CreateDevice(ctx, domain.CreateDevice{
		DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: domain.DeviceStatus(input.Status), APIKey: input.APIKey, LastSeen: input.LastSeen,
		RegisteredByID: input.RegisteredByID, RoomID: input.RoomID,
	})
	return toPublicDevice(value), mapError(err)
}

func (e engine) UpdateDevice(ctx context.Context, input devicefleet.UpdateDevice) (devicefleet.Device, error) {
	value, err := e.service.UpdateDevice(ctx, toDomainUpdate(input))
	return toPublicDevice(value), mapError(err)
}

func (e engine) UpdateDeviceColumns(ctx context.Context, input devicefleet.UpdateDevice, columns []string) (int64, error) {
	affected, err := e.service.UpdateDeviceColumns(ctx, toDomainUpdate(input), columns)
	return affected, mapError(err)
}

func (e engine) DeleteDevice(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteDevice(ctx, id))
}

func (e engine) UpdateDeviceStatus(ctx context.Context, deviceID string, status devicefleet.DeviceStatus) error {
	return mapError(e.service.UpdateDeviceStatus(ctx, deviceID, domain.DeviceStatus(status)))
}

func (e engine) UpdateDeviceLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	return mapError(e.service.UpdateDeviceLastSeen(ctx, id, lastSeen))
}

func (e engine) UpdateDeviceRoom(ctx context.Context, id, roomID int64) error {
	return mapError(e.service.UpdateDeviceRoom(ctx, id, roomID))
}

func (e engine) ListDisplays(ctx context.Context) ([]devicefleet.Display, error) {
	values, err := e.service.ListDisplays(ctx)
	result := make([]devicefleet.Display, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicDisplay(value))
	}
	return result, mapError(err)
}

func (e engine) CreateDisplay(ctx context.Context, name string) (devicefleet.Display, string, error) {
	value, rawToken, err := e.service.CreateDisplay(ctx, name)
	return toPublicDisplay(value), rawToken, mapError(err)
}

func (e engine) UpdateDisplay(ctx context.Context, id int64, name *string, isActive *bool) (devicefleet.Display, error) {
	value, err := e.service.UpdateDisplay(ctx, id, name, isActive)
	return toPublicDisplay(value), mapError(err)
}

func (e engine) RegenerateDisplayToken(ctx context.Context, id int64) (string, error) {
	_, rawToken, err := e.service.RegenerateDisplayToken(ctx, id)
	return rawToken, mapError(err)
}

func (e engine) DeleteDisplay(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteDisplay(ctx, id))
}

func (e engine) Dashboard(ctx context.Context, rawToken string) (devicefleet.Dashboard, error) {
	value, err := e.service.ResolveDashboard(ctx, rawToken)
	if err != nil {
		return devicefleet.Dashboard{}, mapError(err)
	}
	return toPublicDashboard(value), nil
}

func (e engine) FindUnregisteredTagScan(ctx context.Context, id int64) (devicefleet.UnregisteredTagScan, error) {
	value, err := e.service.FindUnregisteredTagScan(ctx, id)
	return toPublicUnregisteredTagScan(value), mapError(err)
}

func (e engine) ListUnregisteredTagScans(ctx context.Context, filter devicefleet.UnregisteredTagScanFilter) ([]devicefleet.UnregisteredTagScan, error) {
	values, err := e.service.ListUnregisteredTagScans(ctx, domain.UnregisteredTagScanFilter{
		TenantIDs: filter.TenantIDs, UnresolvedOnly: filter.UnresolvedOnly, Limit: filter.Limit,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]devicefleet.UnregisteredTagScan, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicUnregisteredTagScan(value))
	}
	return result, nil
}

func (e engine) RecordUnregisteredTagScan(ctx context.Context, input devicefleet.RecordUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error) {
	value, err := e.service.RecordUnregisteredTagScan(ctx, domain.RecordUnregisteredTagScan{
		TagUID: input.TagUID, DeviceID: input.DeviceID, ScannedAt: input.ScannedAt,
	})
	return toPublicUnregisteredTagScan(value), mapError(err)
}

func (e engine) ResolveUnregisteredTagScan(ctx context.Context, input devicefleet.ResolveUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error) {
	value, err := e.service.ResolveUnregisteredTagScan(ctx, domain.ResolveUnregisteredTagScan{
		ID: input.ID, OperatorID: input.OperatorID, Note: input.Note,
	})
	return toPublicUnregisteredTagScan(value), mapError(err)
}

func (e engine) DeleteExpiredUnregisteredTagScans(ctx context.Context, cutoff time.Time) (int64, error) {
	deleted, err := e.service.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
	return deleted, mapError(err)
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

func toPublicDashboard(value application.Dashboard) devicefleet.Dashboard {
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
func mapError(err error) error {
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
