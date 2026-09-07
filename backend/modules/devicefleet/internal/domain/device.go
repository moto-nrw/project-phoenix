package domain

import (
	"errors"
	"time"
)

// Device fleet sentinel errors. The public package maps them to the stable
// errors its callers already match on.
var (
	ErrDeviceNotFound  = errors.New("device not found")
	ErrDeviceInvalid   = errors.New("invalid device data")
	ErrDeviceDuplicate = errors.New("duplicate device id")
	ErrDeviceProtected = errors.New("device is protected")
)

// DeviceStatus is the lifecycle state of one fleet device.
type DeviceStatus string

// The four device states the fleet recognises.
const (
	DeviceStatusActive      DeviceStatus = "active"
	DeviceStatusInactive    DeviceStatus = "inactive"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
	DeviceStatusOffline     DeviceStatus = "offline"
)

// DefaultDeviceOnlineWindow is the online/offline cutoff used when no tenant
// override (iot.device_online_window_minutes) is configured. A device counts
// as online when it was last seen within this window.
const DefaultDeviceOnlineWindow = 5 * time.Minute

// WebManualDeviceID names the reserved per-tenant device that records manual
// web check-ins. It is never listed by default and cannot be edited.
const WebManualDeviceID = "WEB-MANUAL-001"

// ValidDeviceStatus reports whether status is one of the four known states.
func ValidDeviceStatus(status DeviceStatus) bool {
	switch status {
	case DeviceStatusActive, DeviceStatusInactive, DeviceStatusMaintenance, DeviceStatusOffline:
		return true
	default:
		return false
	}
}

// Device is one row of iot.devices. RoomName is resolved through the
// Facilities owner, never through a join owned by this module.
type Device struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeviceID              string
	DeviceType            string
	Name                  *string
	Status                DeviceStatus
	APIKey                *string
	LastSeen              *time.Time
	RegisteredByID        *int64
	RoomID                *int64
	ArchivedAt            *time.Time
	TransferredToDeviceID *int64
	RoomName              *string
}

// IsProtected reports whether the device is the reserved web-manual device.
func (d Device) IsProtected() bool { return d.DeviceID == WebManualDeviceID }

// CreateDevice is the owner input for registering a device.
type CreateDevice struct {
	DeviceID       string
	DeviceType     string
	Name           *string
	Status         DeviceStatus
	APIKey         *string
	LastSeen       *time.Time
	RegisteredByID *int64
	RoomID         *int64
}

// Validate applies the invariants the retired device model carried.
func (input *CreateDevice) Validate() error {
	status, err := validatedStatus(input.DeviceID, input.DeviceType, input.Status)
	if err != nil {
		return err
	}
	input.Status = status
	return nil
}

// UpdateDevice replaces every writable column of one device.
type UpdateDevice struct {
	ID                    int64
	UpdatedAt             time.Time
	DeviceID              string
	DeviceType            string
	Name                  *string
	Status                DeviceStatus
	APIKey                *string
	LastSeen              *time.Time
	RegisteredByID        *int64
	RoomID                *int64
	ArchivedAt            *time.Time
	TransferredToDeviceID *int64
}

// Validate applies the same invariants to a full-row update.
func (input *UpdateDevice) Validate() error {
	status, err := validatedStatus(input.DeviceID, input.DeviceType, input.Status)
	if err != nil {
		return err
	}
	input.Status = status
	return nil
}

func validatedStatus(deviceID, deviceType string, status DeviceStatus) (DeviceStatus, error) {
	if deviceID == "" {
		return "", errors.New("device ID is required")
	}
	if deviceType == "" {
		return "", errors.New("device type is required")
	}
	if status == "" {
		return DeviceStatusActive, nil
	}
	if !ValidDeviceStatus(status) {
		return "", errors.New("invalid device status")
	}
	return status, nil
}

// DeviceFilter is the typed replacement for the retired filter map. Every
// field is optional; nil means "do not restrict".
type DeviceFilter struct {
	DeviceIDContains  *string
	NameContains      *string
	Status            *DeviceStatus
	DeviceType        *string
	ExcludeDeviceType *string
	ExcludeDeviceID   *string
	SeenAfter         *time.Time
	SeenBefore        *time.Time
	RoomID            *int64
	RegisteredByID    *int64
	HasName           *bool
}

// OperationStats is the per-operation runtime evidence the observer records.
type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

// Add folds another statement's counters into s.
func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
