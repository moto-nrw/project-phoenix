package devicefleet

import (
	"context"
	"errors"
	"time"
)

// Error message constants to avoid string duplication
const (
	errDeviceIDEmpty = "device ID cannot be empty"
)

func isProtectedSystemDevice(device *Device) bool {
	return device != nil && device.DeviceID == WebManualDeviceID
}

// service implements the Service interface on top of the owner capability.
type administration struct {
	devices Capability
}

// NewAdministration creates a new IoT service over the Device Fleet capability.
func NewAdministration(devices Capability) Administration {
	if devices == nil {
		panic("iot service: device fleet capability is required")
	}
	return &administration{devices: devices}
}

// Fleet exposes the owner this service delegates to.
func (s *administration) Fleet() Capability { return s.devices }

// IsDeviceOnline reports whether the device is currently online.
func (s *administration) IsDeviceOnline(ctx context.Context, device *Device) bool {
	return s.IsDeviceOnlineAt(ctx, device, time.Now())
}

// IsLastSeenOnline reports whether a device last seen at lastSeen counts as
// online right now. Device-authenticated handlers hold the principal, not
// the row, so they ask by timestamp.
func (s *administration) IsLastSeenOnline(ctx context.Context, lastSeen *time.Time) bool {
	return s.devices.IsDeviceOnline(ctx, Device{LastSeen: lastSeen})
}

// IsDeviceOnlineAt reports whether the device was online at the supplied
// observation time.
func (s *administration) IsDeviceOnlineAt(ctx context.Context, device *Device, now time.Time) bool {
	if device == nil {
		return false
	}
	return s.devices.IsDeviceOnlineAt(ctx, *device, now)
}

// DeviceOnlineWindow resolves the per-tenant device-online window.
func (s *administration) DeviceOnlineWindow(ctx context.Context) time.Duration {
	return s.devices.DeviceOnlineWindow(ctx)
}

// CreateDevice creates a new IoT device.
func (s *administration) CreateDevice(ctx context.Context, device *Device) error {
	if device == nil {
		return &AdministrationError{Op: "CreateDevice", Err: ErrAdministrationInvalidDeviceData}
	}
	if err := validateAdministrationDevice(device); err != nil {
		return &AdministrationError{Op: "CreateDevice", Err: err}
	}

	// Reject a device_id already taken in this tenant before the insert, so
	// the caller sees the German duplicate message rather than a constraint
	// error. The table's unique index remains the real guard.
	existing, err := s.devices.FindDeviceByDeviceID(ctx, device.DeviceID)
	if err == nil && existing.ID > 0 {
		return &AdministrationError{Op: "CreateDevice", Err: &AdministrationDuplicateDeviceIDError{DeviceID: device.DeviceID}}
	}

	if device.Status == "" {
		device.Status = DeviceStatusActive
	}
	if device.LastSeen == nil {
		now := time.Now()
		device.LastSeen = &now
	}

	created, err := s.devices.CreateDevice(ctx, CreateDevice{
		DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name,
		Status: DeviceStatus(device.Status), APIKey: device.APIKey,
		LastSeen: device.LastSeen, RegisteredByID: device.RegisteredByID, RoomID: device.RoomID,
	})
	if err != nil {
		return &AdministrationError{Op: "CreateDevice", Err: err}
	}
	*device = *administrationDevicePointer(created)
	return nil
}

// GetDeviceByID retrieves a device by its ID.
func (s *administration) GetDeviceByID(ctx context.Context, id int64) (*Device, error) {
	if id <= 0 {
		return nil, &AdministrationError{Op: "GetDeviceByID", Err: errors.New("invalid ID")}
	}
	device, err := s.devices.FindDevice(ctx, id)
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			return nil, &AdministrationError{Op: "GetDeviceByID", Err: ErrAdministrationDeviceNotFound}
		}
		return nil, &AdministrationError{Op: "GetDeviceByID", Err: err}
	}
	return administrationDevicePointer(device), nil
}

// GetDeviceByDeviceID retrieves a device by its device ID.
func (s *administration) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*Device, error) {
	if deviceID == "" {
		return nil, &AdministrationError{Op: "GetDeviceByDeviceID", Err: errors.New(errDeviceIDEmpty)}
	}
	device, err := s.devices.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &AdministrationError{Op: "GetDeviceByDeviceID", Err: err}
	}
	if device.ID <= 0 {
		return nil, &AdministrationError{Op: "GetDeviceByDeviceID", Err: &AdministrationDeviceNotFoundError{DeviceID: deviceID}}
	}
	return administrationDevicePointer(device), nil
}

// UpdateDevice updates an existing IoT device.
func (s *administration) UpdateDevice(ctx context.Context, device *Device) error {
	if device == nil || device.ID <= 0 {
		return &AdministrationError{Op: "UpdateDevice", Err: ErrAdministrationInvalidDeviceData}
	}
	if err := validateAdministrationDevice(device); err != nil {
		return &AdministrationError{Op: "UpdateDevice", Err: err}
	}

	existing, err := s.devices.FindDevice(ctx, device.ID)
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			return &AdministrationError{Op: "UpdateDevice", Err: ErrAdministrationDeviceNotFound}
		}
		return &AdministrationError{Op: "UpdateDevice", Err: err}
	}
	if existing.ID <= 0 {
		return &AdministrationError{Op: "UpdateDevice", Err: ErrAdministrationDeviceNotFound}
	}
	if isProtectedSystemDevice(administrationDevicePointer(existing)) {
		return &AdministrationError{Op: "UpdateDevice", Err: ErrAdministrationDeviceProtected}
	}

	if existing.DeviceID != device.DeviceID {
		duplicate, dupErr := s.devices.FindDeviceByDeviceID(ctx, device.DeviceID)
		if dupErr == nil && duplicate.ID > 0 && duplicate.ID != device.ID {
			return &AdministrationError{Op: "UpdateDevice", Err: &AdministrationDuplicateDeviceIDError{DeviceID: device.DeviceID}}
		}
	}

	updated, err := s.devices.UpdateDevice(ctx, UpdateDevice{
		ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name,
		Status: DeviceStatus(device.Status), APIKey: device.APIKey, LastSeen: device.LastSeen,
		RegisteredByID: device.RegisteredByID, RoomID: device.RoomID, ArchivedAt: device.ArchivedAt,
		TransferredToDeviceID: device.TransferredToDeviceID,
	})
	if err != nil {
		return &AdministrationError{Op: "UpdateDevice", Err: err}
	}
	*device = *administrationDevicePointer(updated)
	return nil
}

// DeleteDevice deletes an IoT device by its ID.
func (s *administration) DeleteDevice(ctx context.Context, id int64) error {
	if id <= 0 {
		return &AdministrationError{Op: "DeleteDevice", Err: errors.New("invalid ID")}
	}
	device, err := s.devices.FindDevice(ctx, id)
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			return &AdministrationError{Op: "DeleteDevice", Err: ErrAdministrationDeviceNotFound}
		}
		return &AdministrationError{Op: "DeleteDevice", Err: err}
	}
	if device.ID <= 0 {
		return &AdministrationError{Op: "DeleteDevice", Err: ErrAdministrationDeviceNotFound}
	}
	if isProtectedSystemDevice(administrationDevicePointer(device)) {
		return &AdministrationError{Op: "DeleteDevice", Err: ErrAdministrationDeviceProtected}
	}
	if err := s.devices.DeleteDevice(ctx, id); err != nil {
		return &AdministrationError{Op: "DeleteDevice", Err: err}
	}
	return nil
}

// ListDevices retrieves devices based on filters. The reserved web-manual
// system device is excluded by default; callers can still fetch it explicitly
// by requesting device_type=virtual.
func (s *administration) ListDevices(ctx context.Context, filter DeviceFilter) ([]*Device, error) {
	if filter.DeviceType == nil {
		excluded := WebManualDeviceID
		filter.ExcludeDeviceID = &excluded
	}
	devices, err := s.devices.ListDevices(ctx, filter)
	if err != nil {
		return nil, &AdministrationError{Op: "ListDevices", Err: err}
	}
	return administrationDevicePointers(devices), nil
}

// UpdateDeviceStatus updates the status of a device.
func (s *administration) UpdateDeviceStatus(ctx context.Context, deviceID string, status DeviceStatus) error {
	if deviceID == "" {
		return &AdministrationError{Op: "UpdateDeviceStatus", Err: errors.New(errDeviceIDEmpty)}
	}
	if !ValidDeviceStatus(DeviceStatus(status)) {
		return &AdministrationError{Op: "UpdateDeviceStatus", Err: errors.New("invalid device status")}
	}

	existing, err := s.devices.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return &AdministrationError{Op: "UpdateDeviceStatus", Err: err}
	}
	if existing.ID <= 0 {
		return &AdministrationError{Op: "UpdateDeviceStatus", Err: &AdministrationDeviceNotFoundError{DeviceID: deviceID}}
	}
	if isProtectedSystemDevice(administrationDevicePointer(existing)) {
		return &AdministrationError{Op: "UpdateDeviceStatus", Err: ErrAdministrationDeviceProtected}
	}
	if err := s.devices.UpdateDeviceStatus(ctx, deviceID, DeviceStatus(status)); err != nil {
		return &AdministrationError{Op: "UpdateDeviceStatus", Err: err}
	}
	return nil
}

// PingDevice updates the last seen time for a device.
func (s *administration) PingDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return &AdministrationError{Op: "PingDevice", Err: errors.New(errDeviceIDEmpty)}
	}
	existing, err := s.devices.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return &AdministrationError{Op: "PingDevice", Err: err}
	}
	if existing.ID <= 0 {
		return &AdministrationError{Op: "PingDevice", Err: &AdministrationDeviceNotFoundError{DeviceID: deviceID}}
	}
	// Address the row by its globally unique primary key, so the ping stays
	// cross-tenant safe.
	if err := s.devices.UpdateDeviceLastSeen(ctx, existing.ID, time.Now()); err != nil {
		return &AdministrationError{Op: "PingDevice", Err: err}
	}
	return nil
}

// GetDevicesByType retrieves devices by their type.
func (s *administration) GetDevicesByType(ctx context.Context, deviceType string) ([]*Device, error) {
	if deviceType == "" {
		return nil, &AdministrationError{Op: "GetDevicesByType", Err: errors.New("device type cannot be empty")}
	}
	devices, err := s.devices.ListDevices(ctx, DeviceFilter{DeviceType: &deviceType})
	if err != nil {
		return nil, &AdministrationError{Op: "GetDevicesByType", Err: err}
	}
	return administrationDevicePointers(devices), nil
}

// GetDevicesByStatus retrieves devices by their status.
func (s *administration) GetDevicesByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error) {
	if !ValidDeviceStatus(DeviceStatus(status)) {
		return nil, &AdministrationError{Op: "GetDevicesByStatus", Err: errors.New("invalid device status")}
	}
	return s.listByStatus(ctx, "GetDevicesByStatus", status)
}

// GetDevicesByRegisteredBy retrieves devices registered by a specific person.
func (s *administration) GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*Device, error) {
	if personID <= 0 {
		return nil, &AdministrationError{Op: "GetDevicesByRegisteredBy", Err: errors.New("invalid person ID")}
	}
	devices, err := s.devices.ListDevices(ctx, DeviceFilter{RegisteredByID: &personID})
	if err != nil {
		return nil, &AdministrationError{Op: "GetDevicesByRegisteredBy", Err: err}
	}
	return administrationDevicePointers(devices), nil
}

// GetActiveDevices retrieves all active devices.
func (s *administration) GetActiveDevices(ctx context.Context) ([]*Device, error) {
	return s.listByStatus(ctx, "GetActiveDevices", DeviceStatusActive)
}

// GetDevicesRequiringMaintenance retrieves all devices requiring maintenance.
func (s *administration) GetDevicesRequiringMaintenance(ctx context.Context) ([]*Device, error) {
	return s.listByStatus(ctx, "GetDevicesRequiringMaintenance", DeviceStatusMaintenance)
}

func (s *administration) listByStatus(ctx context.Context, operation string, status DeviceStatus) ([]*Device, error) {
	mapped := DeviceStatus(status)
	devices, err := s.devices.ListDevices(ctx, DeviceFilter{Status: &mapped})
	if err != nil {
		return nil, &AdministrationError{Op: operation, Err: err}
	}
	return administrationDevicePointers(devices), nil
}

// GetOfflineDevices retrieves devices offline for at least the given duration.
func (s *administration) GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*Device, error) {
	if offlineDuration <= 0 {
		return nil, &AdministrationError{Op: "GetOfflineDevices", Err: errors.New("invalid offline duration")}
	}
	devices, err := s.devices.ListOfflineDevices(ctx, offlineDuration)
	if err != nil {
		return nil, &AdministrationError{Op: "GetOfflineDevices", Err: err}
	}
	return administrationDevicePointers(devices), nil
}

// GetDeviceTypeStatistics retrieves a count of devices by type.
func (s *administration) GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error) {
	stats, err := s.devices.CountDevicesByType(ctx)
	if err != nil {
		return nil, &AdministrationError{Op: "GetDeviceTypeStatistics", Err: err}
	}
	return stats, nil
}

// DetectNewDevices is a placeholder for network discovery.
func (s *administration) DetectNewDevices(_ context.Context) ([]*Device, error) {
	return nil, &AdministrationError{Op: "DetectNewDevices", Err: errors.New("device auto-discovery not implemented")}
}

// ScanNetwork is a placeholder for network scanning.
func (s *administration) ScanNetwork(_ context.Context) (map[string]string, error) {
	return nil, &AdministrationError{Op: "ScanNetwork", Err: errors.New("network scanning not implemented")}
}

type Administration interface {
	// Core device operations
	CreateDevice(ctx context.Context, device *Device) error
	GetDeviceByID(ctx context.Context, id int64) (*Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string) (*Device, error)
	UpdateDevice(ctx context.Context, device *Device) error
	DeleteDevice(ctx context.Context, id int64) error
	ListDevices(ctx context.Context, filter DeviceFilter) ([]*Device, error)

	// Status operations
	UpdateDeviceStatus(ctx context.Context, deviceID string, status DeviceStatus) error
	PingDevice(ctx context.Context, deviceID string) error

	// Filtered lookups
	GetDevicesByType(ctx context.Context, deviceType string) ([]*Device, error)
	GetDevicesByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error)
	GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*Device, error)

	// Monitoring and reporting
	GetActiveDevices(ctx context.Context) ([]*Device, error)
	GetDevicesRequiringMaintenance(ctx context.Context) ([]*Device, error)
	GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*Device, error)
	GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error)

	// Online/offline decision (issue #586 — Rule 12). The window is resolved
	// by the Device Fleet owner from the per-tenant setting
	// device_online_window_minutes.
	DeviceOnlineWindow(ctx context.Context) time.Duration
	IsDeviceOnline(ctx context.Context, device *Device) bool
	IsDeviceOnlineAt(ctx context.Context, device *Device, now time.Time) bool
	IsLastSeenOnline(ctx context.Context, lastSeen *time.Time) bool

	// Network operations
	DetectNewDevices(ctx context.Context) ([]*Device, error)
	ScanNetwork(ctx context.Context) (map[string]string, error)

	// Fleet exposes the Device Fleet owner this service delegates to, so the
	// composition root binds one instance for every entry point (#2676).
	Fleet() Capability
}

func administrationDevicePointer(device Device) *Device { return &device }
func administrationDevicePointers(devices []Device) []*Device {
	result := make([]*Device, 0, len(devices))
	for _, device := range devices {
		result = append(result, administrationDevicePointer(device))
	}
	return result
}
func validateAdministrationDevice(device *Device) error {
	if device.DeviceID == "" {
		return errors.New("device ID is required")
	}
	if device.DeviceType == "" {
		return errors.New("device type is required")
	}
	if device.Status == "" {
		device.Status = DeviceStatusActive
	} else if !ValidDeviceStatus(device.Status) {
		return errors.New("invalid device status")
	}
	return nil
}
