// Package iot is the retained IoT service shape. Every device read and write
// goes through the Device Fleet owner (#2676); this package keeps the error
// wrapping that the PyrePortal contract and the IoT handlers depend on.
package iot

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// Error message constants to avoid string duplication
const (
	errDeviceIDEmpty = "device ID cannot be empty"
)

func isProtectedSystemDevice(device *iot.Device) bool {
	return device != nil && device.DeviceID == iot.WebManualDeviceID
}

// service implements the Service interface on top of the owner capability.
type service struct {
	devices devicefleet.Capability
}

// NewService creates a new IoT service over the Device Fleet capability.
func NewService(devices devicefleet.Capability) Service {
	if devices == nil {
		panic("iot service: device fleet capability is required")
	}
	return &service{devices: devices}
}

// Fleet exposes the owner this service delegates to.
func (s *service) Fleet() devicefleet.Capability { return s.devices }

// IsDeviceOnline reports whether the device is currently online.
func (s *service) IsDeviceOnline(ctx context.Context, device *iot.Device) bool {
	return s.IsDeviceOnlineAt(ctx, device, time.Now())
}

// IsDeviceOnlineAt reports whether the device was online at the supplied
// observation time.
func (s *service) IsDeviceOnlineAt(ctx context.Context, device *iot.Device, now time.Time) bool {
	if device == nil {
		return false
	}
	return s.devices.IsDeviceOnlineAt(ctx, toCapabilityDevice(device), now)
}

// DeviceOnlineWindow resolves the per-tenant device-online window.
func (s *service) DeviceOnlineWindow(ctx context.Context) time.Duration {
	return s.devices.DeviceOnlineWindow(ctx)
}

// CreateDevice creates a new IoT device.
func (s *service) CreateDevice(ctx context.Context, device *iot.Device) error {
	if device == nil {
		return &IoTError{Op: "CreateDevice", Err: ErrInvalidDeviceData}
	}
	if err := device.Validate(); err != nil {
		return &IoTError{Op: "CreateDevice", Err: err}
	}

	// Reject a device_id already taken in this tenant before the insert, so
	// the caller sees the German duplicate message rather than a constraint
	// error. The table's unique index remains the real guard.
	existing, err := s.devices.FindDeviceByDeviceID(ctx, device.DeviceID)
	if err == nil && existing.ID > 0 {
		return &IoTError{Op: "CreateDevice", Err: &DuplicateDeviceIDError{DeviceID: device.DeviceID}}
	}

	if device.Status == "" {
		device.Status = iot.DeviceStatusActive
	}
	if device.LastSeen == nil {
		now := time.Now()
		device.LastSeen = &now
	}

	created, err := s.devices.CreateDevice(ctx, devicefleet.CreateDevice{
		DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name,
		Status: devicefleet.DeviceStatus(device.Status), APIKey: device.APIKey,
		LastSeen: device.LastSeen, RegisteredByID: device.RegisteredByID, RoomID: device.RoomID,
	})
	if err != nil {
		return &IoTError{Op: "CreateDevice", Err: err}
	}
	*device = *toModel(created)
	return nil
}

// GetDeviceByID retrieves a device by its ID.
func (s *service) GetDeviceByID(ctx context.Context, id int64) (*iot.Device, error) {
	if id <= 0 {
		return nil, &IoTError{Op: "GetDeviceByID", Err: errors.New("invalid ID")}
	}
	device, err := s.devices.FindDevice(ctx, id)
	if err != nil {
		if errors.Is(err, devicefleet.ErrDeviceNotFound) {
			return nil, &IoTError{Op: "GetDeviceByID", Err: ErrDeviceNotFound}
		}
		return nil, &IoTError{Op: "GetDeviceByID", Err: err}
	}
	return toModel(device), nil
}

// GetDeviceByDeviceID retrieves a device by its device ID.
func (s *service) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	if deviceID == "" {
		return nil, &IoTError{Op: "GetDeviceByDeviceID", Err: errors.New(errDeviceIDEmpty)}
	}
	device, err := s.devices.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &IoTError{Op: "GetDeviceByDeviceID", Err: err}
	}
	if device.ID <= 0 {
		return nil, &IoTError{Op: "GetDeviceByDeviceID", Err: &DeviceNotFoundError{DeviceID: deviceID}}
	}
	return toModel(device), nil
}

// UpdateDevice updates an existing IoT device.
func (s *service) UpdateDevice(ctx context.Context, device *iot.Device) error {
	if device == nil || device.ID <= 0 {
		return &IoTError{Op: "UpdateDevice", Err: ErrInvalidDeviceData}
	}
	if err := device.Validate(); err != nil {
		return &IoTError{Op: "UpdateDevice", Err: err}
	}

	existing, err := s.devices.FindDevice(ctx, device.ID)
	if err != nil {
		if errors.Is(err, devicefleet.ErrDeviceNotFound) {
			return &IoTError{Op: "UpdateDevice", Err: ErrDeviceNotFound}
		}
		return &IoTError{Op: "UpdateDevice", Err: err}
	}
	if existing.ID <= 0 {
		return &IoTError{Op: "UpdateDevice", Err: ErrDeviceNotFound}
	}
	if isProtectedSystemDevice(toModel(existing)) {
		return &IoTError{Op: "UpdateDevice", Err: ErrDeviceProtected}
	}

	if existing.DeviceID != device.DeviceID {
		duplicate, dupErr := s.devices.FindDeviceByDeviceID(ctx, device.DeviceID)
		if dupErr == nil && duplicate.ID > 0 && duplicate.ID != device.ID {
			return &IoTError{Op: "UpdateDevice", Err: &DuplicateDeviceIDError{DeviceID: device.DeviceID}}
		}
	}

	updated, err := s.devices.UpdateDevice(ctx, devicefleet.UpdateDevice{
		ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name,
		Status: devicefleet.DeviceStatus(device.Status), APIKey: device.APIKey, LastSeen: device.LastSeen,
		RegisteredByID: device.RegisteredByID, RoomID: device.RoomID, ArchivedAt: device.ArchivedAt,
		TransferredToDeviceID: device.TransferredToDeviceID,
	})
	if err != nil {
		return &IoTError{Op: "UpdateDevice", Err: err}
	}
	*device = *toModel(updated)
	return nil
}

// DeleteDevice deletes an IoT device by its ID.
func (s *service) DeleteDevice(ctx context.Context, id int64) error {
	if id <= 0 {
		return &IoTError{Op: "DeleteDevice", Err: errors.New("invalid ID")}
	}
	device, err := s.devices.FindDevice(ctx, id)
	if err != nil {
		if errors.Is(err, devicefleet.ErrDeviceNotFound) {
			return &IoTError{Op: "DeleteDevice", Err: ErrDeviceNotFound}
		}
		return &IoTError{Op: "DeleteDevice", Err: err}
	}
	if device.ID <= 0 {
		return &IoTError{Op: "DeleteDevice", Err: ErrDeviceNotFound}
	}
	if isProtectedSystemDevice(toModel(device)) {
		return &IoTError{Op: "DeleteDevice", Err: ErrDeviceProtected}
	}
	if err := s.devices.DeleteDevice(ctx, id); err != nil {
		return &IoTError{Op: "DeleteDevice", Err: err}
	}
	return nil
}

// ListDevices retrieves devices based on filters. The reserved web-manual
// system device is excluded by default; callers can still fetch it explicitly
// by requesting device_type=virtual.
func (s *service) ListDevices(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	filter, err := toFilter(filters)
	if err != nil {
		return nil, &IoTError{Op: "ListDevices", Err: err}
	}
	if filter.DeviceType == nil {
		excluded := iot.WebManualDeviceID
		filter.ExcludeDeviceID = &excluded
	}
	devices, err := s.devices.ListDevices(ctx, filter)
	if err != nil {
		return nil, &IoTError{Op: "ListDevices", Err: err}
	}
	return toModels(devices), nil
}

// UpdateDeviceStatus updates the status of a device.
func (s *service) UpdateDeviceStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error {
	if deviceID == "" {
		return &IoTError{Op: "UpdateDeviceStatus", Err: errors.New(errDeviceIDEmpty)}
	}
	if !devicefleet.ValidDeviceStatus(devicefleet.DeviceStatus(status)) {
		return &IoTError{Op: "UpdateDeviceStatus", Err: errors.New("invalid device status")}
	}

	existing, err := s.devices.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return &IoTError{Op: "UpdateDeviceStatus", Err: err}
	}
	if existing.ID <= 0 {
		return &IoTError{Op: "UpdateDeviceStatus", Err: &DeviceNotFoundError{DeviceID: deviceID}}
	}
	if isProtectedSystemDevice(toModel(existing)) {
		return &IoTError{Op: "UpdateDeviceStatus", Err: ErrDeviceProtected}
	}
	if err := s.devices.UpdateDeviceStatus(ctx, deviceID, devicefleet.DeviceStatus(status)); err != nil {
		return &IoTError{Op: "UpdateDeviceStatus", Err: err}
	}
	return nil
}

// PingDevice updates the last seen time for a device.
func (s *service) PingDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return &IoTError{Op: "PingDevice", Err: errors.New(errDeviceIDEmpty)}
	}
	existing, err := s.devices.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return &IoTError{Op: "PingDevice", Err: err}
	}
	if existing.ID <= 0 {
		return &IoTError{Op: "PingDevice", Err: &DeviceNotFoundError{DeviceID: deviceID}}
	}
	// Address the row by its globally unique primary key, so the ping stays
	// cross-tenant safe.
	if err := s.devices.UpdateDeviceLastSeen(ctx, existing.ID, time.Now()); err != nil {
		return &IoTError{Op: "PingDevice", Err: err}
	}
	return nil
}

// GetDevicesByType retrieves devices by their type.
func (s *service) GetDevicesByType(ctx context.Context, deviceType string) ([]*iot.Device, error) {
	if deviceType == "" {
		return nil, &IoTError{Op: "GetDevicesByType", Err: errors.New("device type cannot be empty")}
	}
	devices, err := s.devices.ListDevices(ctx, devicefleet.DeviceFilter{DeviceType: &deviceType})
	if err != nil {
		return nil, &IoTError{Op: "GetDevicesByType", Err: err}
	}
	return toModels(devices), nil
}

// GetDevicesByStatus retrieves devices by their status.
func (s *service) GetDevicesByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error) {
	if !devicefleet.ValidDeviceStatus(devicefleet.DeviceStatus(status)) {
		return nil, &IoTError{Op: "GetDevicesByStatus", Err: errors.New("invalid device status")}
	}
	return s.listByStatus(ctx, "GetDevicesByStatus", status)
}

// GetDevicesByRegisteredBy retrieves devices registered by a specific person.
func (s *service) GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error) {
	if personID <= 0 {
		return nil, &IoTError{Op: "GetDevicesByRegisteredBy", Err: errors.New("invalid person ID")}
	}
	devices, err := s.devices.ListDevices(ctx, devicefleet.DeviceFilter{RegisteredByID: &personID})
	if err != nil {
		return nil, &IoTError{Op: "GetDevicesByRegisteredBy", Err: err}
	}
	return toModels(devices), nil
}

// GetActiveDevices retrieves all active devices.
func (s *service) GetActiveDevices(ctx context.Context) ([]*iot.Device, error) {
	return s.listByStatus(ctx, "GetActiveDevices", iot.DeviceStatusActive)
}

// GetDevicesRequiringMaintenance retrieves all devices requiring maintenance.
func (s *service) GetDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error) {
	return s.listByStatus(ctx, "GetDevicesRequiringMaintenance", iot.DeviceStatusMaintenance)
}

func (s *service) listByStatus(ctx context.Context, operation string, status iot.DeviceStatus) ([]*iot.Device, error) {
	mapped := devicefleet.DeviceStatus(status)
	devices, err := s.devices.ListDevices(ctx, devicefleet.DeviceFilter{Status: &mapped})
	if err != nil {
		return nil, &IoTError{Op: operation, Err: err}
	}
	return toModels(devices), nil
}

// GetOfflineDevices retrieves devices offline for at least the given duration.
func (s *service) GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*iot.Device, error) {
	if offlineDuration <= 0 {
		return nil, &IoTError{Op: "GetOfflineDevices", Err: errors.New("invalid offline duration")}
	}
	devices, err := s.devices.ListOfflineDevices(ctx, offlineDuration)
	if err != nil {
		return nil, &IoTError{Op: "GetOfflineDevices", Err: err}
	}
	return toModels(devices), nil
}

// GetDeviceTypeStatistics retrieves a count of devices by type.
func (s *service) GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error) {
	stats, err := s.devices.CountDevicesByType(ctx)
	if err != nil {
		return nil, &IoTError{Op: "GetDeviceTypeStatistics", Err: err}
	}
	return stats, nil
}

// DetectNewDevices is a placeholder for network discovery.
func (s *service) DetectNewDevices(_ context.Context) ([]*iot.Device, error) {
	return nil, &IoTError{Op: "DetectNewDevices", Err: errors.New("device auto-discovery not implemented")}
}

// ScanNetwork is a placeholder for network scanning.
func (s *service) ScanNetwork(_ context.Context) (map[string]string, error) {
	return nil, &IoTError{Op: "ScanNetwork", Err: errors.New("network scanning not implemented")}
}

// UpdateDeviceLastSeenAt updates only the last_seen timestamp for a device,
// addressed by its globally unique primary key for cross-tenant safety.
func (s *service) UpdateDeviceLastSeenAt(ctx context.Context, id int64, lastSeen time.Time) error {
	if id <= 0 {
		return &IoTError{Op: "UpdateDeviceLastSeenAt", Err: errors.New("device ID must be positive")}
	}
	return s.devices.UpdateDeviceLastSeen(ctx, id, lastSeen)
}

// GetDeviceByAPIKey retrieves a device by its API key for authentication.
func (s *service) GetDeviceByAPIKey(ctx context.Context, apiKey string) (*iot.Device, error) {
	if apiKey == "" {
		return nil, &IoTError{Op: "GetDeviceByAPIKey", Err: errors.New("API key cannot be empty")}
	}
	device, err := s.devices.FindDeviceByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, &IoTError{Op: "GetDeviceByAPIKey", Err: err}
	}
	return toModel(device), nil
}
