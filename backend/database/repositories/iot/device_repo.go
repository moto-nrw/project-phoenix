// backend/database/repositories/iot/device_repo.go
package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/uptrace/bun"
)

// DeviceRepository implements iot.DeviceRepository
type DeviceRepository struct {
	db *bun.DB
}

// NewDeviceRepository creates a new device repository
func NewDeviceRepository(db *bun.DB) iot.DeviceRepository {
	return &DeviceRepository{db: db}
}

// Create inserts a new device into the database
func (r *DeviceRepository) Create(ctx context.Context, device *iot.Device) error {
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
func (r *DeviceRepository) FindByID(ctx context.Context, id interface{}) (*iot.Device, error) {
	device := new(iot.Device)
	err := r.db.NewSelect().Model(device).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return device, nil
}

// FindByDeviceID retrieves a device by its device ID
func (r *DeviceRepository) FindByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	device := new(iot.Device)
	err := r.db.NewSelect().Model(device).Where("device_id = ?", deviceID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_device_id", Err: err}
	}
	return device, nil
}

// FindByDeviceType retrieves devices by device type
func (r *DeviceRepository) FindByDeviceType(ctx context.Context, deviceType string) ([]*iot.Device, error) {
	var devices []*iot.Device
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
func (r *DeviceRepository) FindByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error) {
	var devices []*iot.Device
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
func (r *DeviceRepository) FindByRegisteredBy(ctx context.Context, registeredByID int64) ([]*iot.Device, error) {
	var devices []*iot.Device
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
func (r *DeviceRepository) UpdateStatus(ctx context.Context, id int64, status iot.DeviceStatus) error {
	_, err := r.db.NewUpdate().
		Model((*iot.Device)(nil)).
		Set("status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// UpdateLastSeen updates the last seen timestamp of a device
func (r *DeviceRepository) UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*iot.Device)(nil)).
		Set("last_seen = ?", lastSeen).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_last_seen", Err: err}
	}
	return nil
}

// FindActive retrieves all active devices
func (r *DeviceRepository) FindActive(ctx context.Context) ([]*iot.Device, error) {
	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("status = ?", iot.DeviceStatusActive).
		Order("last_seen DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return devices, nil
}

// Update updates an existing device
func (r *DeviceRepository) Update(ctx context.Context, device *iot.Device) error {
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
func (r *DeviceRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*iot.Device)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves devices matching the filters
func (r *DeviceRepository) List(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	var devices []*iot.Device
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
