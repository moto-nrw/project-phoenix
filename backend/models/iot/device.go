package iot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// DeviceStatus represents the possible statuses of an IoT device
type DeviceStatus string

// Device status constants
const (
	DeviceStatusActive      DeviceStatus = "active"
	DeviceStatusInactive    DeviceStatus = "inactive"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
	DeviceStatusOffline     DeviceStatus = "offline"
)

// Device represents an IoT device in the system
type Device struct {
	base.Model
	DeviceID       string       `bun:"device_id,notnull,unique" json:"device_id"`
	DeviceType     string       `bun:"device_type,notnull" json:"device_type"`
	Name           string       `bun:"name" json:"name,omitempty"`
	Status         DeviceStatus `bun:"status,notnull,default:'active'" json:"status"`
	LastSeen       *time.Time   `bun:"last_seen" json:"last_seen,omitempty"`
	RegisteredByID *int64       `bun:"registered_by_id" json:"registered_by_id,omitempty"`

	// Relations
	RegisteredBy *users.Person `bun:"rel:belongs-to,join:registered_by_id=id" json:"registered_by,omitempty"`
}

// TableName returns the table name for the Device model
func (d *Device) TableName() string {
	return "iot.devices"
}

// GetID returns the device ID
func (d *Device) GetID() interface{} {
	return d.ID
}

// GetCreatedAt returns the creation timestamp
func (d *Device) GetCreatedAt() time.Time {
	return d.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (d *Device) GetUpdatedAt() time.Time {
	return d.UpdatedAt
}

// Validate validates the device fields
func (d *Device) Validate() error {
	if strings.TrimSpace(d.DeviceID) == "" {
		return errors.New("device ID is required")
	}

	if strings.TrimSpace(d.DeviceType) == "" {
		return errors.New("device type is required")
	}

	// Validate status if provided
	if d.Status != "" {
		validStatuses := map[DeviceStatus]bool{
			DeviceStatusActive:      true,
			DeviceStatusInactive:    true,
			DeviceStatusMaintenance: true,
			DeviceStatusOffline:     true,
		}

		if !validStatuses[d.Status] {
			return errors.New("invalid device status")
		}
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (d *Device) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := d.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	d.DeviceID = strings.TrimSpace(d.DeviceID)
	d.DeviceType = strings.TrimSpace(d.DeviceType)
	d.Name = strings.TrimSpace(d.Name)

	// Set default status if not provided
	if d.Status == "" {
		d.Status = DeviceStatusActive
	}

	return nil
}

// IsActive checks if the device is active
func (d *Device) IsActive() bool {
	return d.Status == DeviceStatusActive
}

// DeviceRepository defines operations for working with IoT devices
type DeviceRepository interface {
	base.Repository[*Device]
	FindByDeviceID(ctx context.Context, deviceID string) (*Device, error)
	FindByDeviceType(ctx context.Context, deviceType string) ([]*Device, error)
	FindByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error)
	FindByRegisteredBy(ctx context.Context, registeredByID int64) ([]*Device, error)
	UpdateStatus(ctx context.Context, id int64, status DeviceStatus) error
	UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error
	FindActive(ctx context.Context) ([]*Device, error)
}

// DefaultDeviceRepository is the default implementation of DeviceRepository
type DefaultDeviceRepository struct {
	db *bun.DB
}

// NewDeviceRepository creates a new device repository
func NewDeviceRepository(db *bun.DB) DeviceRepository {
	return &DefaultDeviceRepository{db: db}
}

// Create inserts a new device into the database
func (r *DefaultDeviceRepository) Create(ctx context.Context, device *Device) error {
	if err := device.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(device).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a device by its ID
func (r *DefaultDeviceRepository) FindByID(ctx context.Context, id interface{}) (*Device, error) {
	device := new(Device)
	err := r.db.NewSelect().Model(device).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return device, nil
}

// FindByDeviceID retrieves a device by its device ID
func (r *DefaultDeviceRepository) FindByDeviceID(ctx context.Context, deviceID string) (*Device, error) {
	device := new(Device)
	err := r.db.NewSelect().Model(device).Where("device_id = ?", deviceID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_device_id", Err: err}
	}
	return device, nil
}

// FindByDeviceType retrieves devices by device type
func (r *DefaultDeviceRepository) FindByDeviceType(ctx context.Context, deviceType string) ([]*Device, error) {
	var devices []*Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("device_type = ?", deviceType).
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_device_type", Err: err}
	}
	return devices, nil
}

// FindByStatus retrieves devices by status
func (r *DefaultDeviceRepository) FindByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error) {
	var devices []*Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("status = ?", status).
		Order("last_seen DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_status", Err: err}
	}
	return devices, nil
}

// FindByRegisteredBy retrieves devices by the person who registered them
func (r *DefaultDeviceRepository) FindByRegisteredBy(ctx context.Context, registeredByID int64) ([]*Device, error) {
	var devices []*Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("registered_by_id = ?", registeredByID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_registered_by", Err: err}
	}
	return devices, nil
}

// UpdateStatus updates the status of a device
func (r *DefaultDeviceRepository) UpdateStatus(ctx context.Context, id int64, status DeviceStatus) error {
	_, err := r.db.NewUpdate().
		Model((*Device)(nil)).
		Set("status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// UpdateLastSeen updates the last seen timestamp of a device
func (r *DefaultDeviceRepository) UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*Device)(nil)).
		Set("last_seen = ?", lastSeen).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_last_seen", Err: err}
	}
	return nil
}

// FindActive retrieves all active devices
func (r *DefaultDeviceRepository) FindActive(ctx context.Context) ([]*Device, error) {
	var devices []*Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("status = ?", DeviceStatusActive).
		Order("last_seen DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return devices, nil
}

// Update updates an existing device
func (r *DefaultDeviceRepository) Update(ctx context.Context, device *Device) error {
	if err := device.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(device).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a device
func (r *DefaultDeviceRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Device)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves devices matching the filters
func (r *DefaultDeviceRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Device, error) {
	var devices []*Device
	query := r.db.NewSelect().Model(&devices)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return devices, nil
}
