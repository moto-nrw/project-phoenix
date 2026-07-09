package iot

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// DeviceStatus represents the status of an IoT device
type DeviceStatus string

// DeviceStatus enum values
const (
	DeviceStatusActive      DeviceStatus = "active"
	DeviceStatusInactive    DeviceStatus = "inactive"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
	DeviceStatusOffline     DeviceStatus = "offline"
)

// Device type constants
const (
	DeviceTypeVirtual = "virtual"
	WebManualDeviceID = "WEB-MANUAL-001"
)

// Device represents an IoT device in the system
type Device struct {
	base.Model `bun:"schema:iot,table:devices"`
	base.TenantModel
	DeviceID       string       `bun:"device_id,notnull" json:"device_id"`
	DeviceType     string       `bun:"device_type,notnull" json:"device_type"`
	Name           *string      `bun:"name" json:"name,omitempty"`
	Status         DeviceStatus `bun:"status,notnull,default:'active'" json:"status"`
	APIKey         *string      `bun:"api_key,unique" json:"-"`              // Never expose API key in JSON
	LastSeen       *time.Time   `bun:"last_seen" json:"last_seen,omitempty"` // Used as last_activity for health monitoring
	RegisteredByID *int64       `bun:"registered_by_id" json:"registered_by_id,omitempty"`
	RoomID         *int64       `bun:"room_id" json:"room_id,omitempty"`

	// Relations

	// Transient fields populated by JOINs (ignored by INSERT/UPDATE)
	RoomName *string `bun:"room_name,scanonly" json:"room_name,omitempty"`
}

// Validate ensures device data is valid
func (d *Device) Validate() error {
	if d.DeviceID == "" {
		return errors.New("device ID is required")
	}

	if d.DeviceType == "" {
		return errors.New("device type is required")
	}

	// Validate the status
	if d.Status == "" {
		d.Status = DeviceStatusActive
	} else if !isValidDeviceStatus(d.Status) {
		return errors.New("invalid device status")
	}

	return nil
}

// isValidDeviceStatus checks if the given status is a valid DeviceStatus
func isValidDeviceStatus(status DeviceStatus) bool {
	switch status {
	case DeviceStatusActive, DeviceStatusInactive, DeviceStatusMaintenance, DeviceStatusOffline:
		return true
	}
	return false
}

// IsActive checks if the device is currently active
func (d *Device) IsActive() bool {
	return d.Status == DeviceStatusActive
}

// IsOffline checks if the device is currently offline
func (d *Device) IsOffline() bool {
	return d.Status == DeviceStatusOffline
}

// UpdateLastSeen updates the last seen timestamp to the current time
func (d *Device) UpdateLastSeen() {
	now := time.Now()
	d.LastSeen = &now
}

// SetStatus sets the device status
func (d *Device) SetStatus(status DeviceStatus) error {
	if !isValidDeviceStatus(status) {
		return errors.New("invalid device status")
	}
	d.Status = status
	return nil
}

// GetLastSeenDuration returns the duration since the device was last seen
// Returns nil if the device has never been seen
func (d *Device) GetLastSeenDuration() *time.Duration {
	if d.LastSeen == nil {
		return nil
	}

	duration := time.Since(*d.LastSeen)
	return &duration
}

// HasAPIKey returns true if the device has an API key set
func (d *Device) HasAPIKey() bool {
	return d.APIKey != nil && *d.APIKey != ""
}
