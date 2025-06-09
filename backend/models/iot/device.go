package iot

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
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

// Device represents an IoT device in the system
type Device struct {
	base.Model     `bun:"schema:iot,table:devices"`
	DeviceID       string       `bun:"device_id,notnull,unique" json:"device_id"`
	DeviceType     string       `bun:"device_type,notnull" json:"device_type"`
	Name           *string      `bun:"name" json:"name,omitempty"`
	Status         DeviceStatus `bun:"status,notnull,default:'active'" json:"status"`
	APIKey         *string      `bun:"api_key,unique" json:"-"`              // Never expose API key in JSON
	LastSeen       *time.Time   `bun:"last_seen" json:"last_seen,omitempty"` // Used as last_activity for health monitoring
	RegisteredByID *int64       `bun:"registered_by_id" json:"registered_by_id,omitempty"`

	// Relations
	RegisteredBy *users.Person `bun:"-" json:"registered_by,omitempty"`
}

func (d *Device) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr("iot.devices")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr("iot.devices")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr("iot.devices")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr("iot.devices")
	}
	return nil
}

// TableName returns the database table name
func (d *Device) TableName() string {
	return "iot.devices"
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

// IsOnline checks if the device is considered online based on last seen time
// A device is considered online if it was seen in the last 5 minutes
func (d *Device) IsOnline() bool {
	if d.LastSeen == nil {
		return false
	}

	// Device is considered online if seen in the last 5 minutes
	return time.Since(*d.LastSeen) <= 5*time.Minute
}

// GetID returns the entity's ID
func (m *Device) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Device) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Device) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

// HasAPIKey returns true if the device has an API key set
func (d *Device) HasAPIKey() bool {
	return d.APIKey != nil && *d.APIKey != ""
}
