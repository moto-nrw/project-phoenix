package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/uptrace/bun"
)

// DeviceRepository implements iot.DeviceRepository interface
type DeviceRepository struct {
	*base.Repository[*iot.Device]
	db *bun.DB
}

// NewDeviceRepository creates a new DeviceRepository
func NewDeviceRepository(db *bun.DB) iot.DeviceRepository {
	return &DeviceRepository{
		Repository: base.NewRepository[*iot.Device](db, "iot.devices", "Device"),
		db:         db,
	}
}

// FindByDeviceID retrieves a device by its deviceID
func (r *DeviceRepository) FindByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	device := new(iot.Device)
	err := r.db.NewSelect().
		Model(device).
		Where("device_id = ?", deviceID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by device ID",
			Err: err,
		}
	}

	return device, nil
}

// FindByType retrieves devices by their type
func (r *DeviceRepository) FindByType(ctx context.Context, deviceType string) ([]*iot.Device, error) {
	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("device_type = ?", deviceType).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by type",
			Err: err,
		}
	}

	return devices, nil
}

// FindByStatus retrieves devices by their status
func (r *DeviceRepository) FindByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error) {
	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("status = ?", status).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by status",
			Err: err,
		}
	}

	return devices, nil
}

// FindByRegisteredBy retrieves devices registered by a specific person
func (r *DeviceRepository) FindByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error) {
	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("registered_by_id = ?", personID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by registered by",
			Err: err,
		}
	}

	return devices, nil
}

// UpdateLastSeen updates the last seen timestamp for a device
func (r *DeviceRepository) UpdateLastSeen(ctx context.Context, deviceID string, lastSeen time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*iot.Device)(nil)).
		Set("last_seen = ?", lastSeen).
		Where("device_id = ?", deviceID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last seen",
			Err: err,
		}
	}

	return nil
}

// UpdateStatus updates the status for a device
func (r *DeviceRepository) UpdateStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error {
	_, err := r.db.NewUpdate().
		Model((*iot.Device)(nil)).
		Set("status = ?", status).
		Where("device_id = ?", deviceID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update status",
			Err: err,
		}
	}

	return nil
}

// FindActiveDevices retrieves all active devices
func (r *DeviceRepository) FindActiveDevices(ctx context.Context) ([]*iot.Device, error) {
	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("status = ?", iot.DeviceStatusActive).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active devices",
			Err: err,
		}
	}

	return devices, nil
}

// FindDevicesRequiringMaintenance retrieves all devices requiring maintenance
func (r *DeviceRepository) FindDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error) {
	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("status = ?", iot.DeviceStatusMaintenance).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find devices requiring maintenance",
			Err: err,
		}
	}

	return devices, nil
}

// FindOfflineDevices retrieves devices that have been offline for at least the specified duration
func (r *DeviceRepository) FindOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]*iot.Device, error) {
	cutoffTime := time.Now().Add(-offlineSince)

	var devices []*iot.Device
	err := r.db.NewSelect().
		Model(&devices).
		Where("last_seen < ? OR (last_seen IS NULL AND created_at < ?)", cutoffTime, cutoffTime).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find offline devices",
			Err: err,
		}
	}

	return devices, nil
}

// CountDevicesByType counts devices grouped by their type
func (r *DeviceRepository) CountDevicesByType(ctx context.Context) (map[string]int, error) {
	type countResult struct {
		DeviceType string `bun:"device_type"`
		Count      int    `bun:"count"`
	}

	var counts []countResult
	err := r.db.NewSelect().
		Model((*iot.Device)(nil)).
		Column("device_type").
		ColumnExpr("COUNT(*) AS count").
		Group("device_type").
		Order("count DESC").
		Scan(ctx, &counts)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count devices by type",
			Err: err,
		}
	}

	// Convert to map
	countMap := make(map[string]int)
	for _, count := range counts {
		countMap[count.DeviceType] = count.Count
	}

	return countMap, nil
}

// Create overrides the base Create method to handle validation
func (r *DeviceRepository) Create(ctx context.Context, device *iot.Device) error {
	if device == nil {
		return fmt.Errorf("device cannot be nil")
	}

	// Validate device
	if err := device.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, device)
}

// Update overrides the base Update method to handle validation
func (r *DeviceRepository) Update(ctx context.Context, device *iot.Device) error {
	if device == nil {
		return fmt.Errorf("device cannot be nil")
	}

	// Validate device
	if err := device.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, device)
}

// List retrieves devices matching the provided filters
func (r *DeviceRepository) List(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	var devices []*iot.Device
	query := r.db.NewSelect().Model(&devices)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "device_id_like":
				if strValue, ok := value.(string); ok {
					query = query.Where("device_id ILIKE ?", "%"+strValue+"%")
				}
			case "name_like":
				if strValue, ok := value.(string); ok {
					query = query.Where("name ILIKE ?", "%"+strValue+"%")
				}
			case "status":
				query = query.Where("status = ?", value)
			case "device_type":
				query = query.Where("device_type = ?", value)
			case "seen_after":
				if timeValue, ok := value.(time.Time); ok {
					query = query.Where("last_seen > ?", timeValue)
				}
			case "seen_before":
				if timeValue, ok := value.(time.Time); ok {
					query = query.Where("last_seen < ?", timeValue)
				}
			case "has_name":
				if boolValue, ok := value.(bool); ok && boolValue {
					query = query.Where("name IS NOT NULL")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					query = query.Where("name IS NULL")
				}
			default:
				// Default to exact match for other fields
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return devices, nil
}
