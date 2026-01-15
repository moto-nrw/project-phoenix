package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	"github.com/uptrace/bun"
)

// Constants to avoid duplicate string literals (S1192)
const (
	tableIoTDevices    = "iot.devices"
	whereDeviceIDEqual = "device_id = ?"
	whereStatusEqual   = "status = ?"
)

// DeviceRepository implements iot.DeviceRepository interface
type DeviceRepository struct {
	*base.Repository[*iot.Device]
	db *bun.DB
}

// NewDeviceRepository creates a new DeviceRepository
func NewDeviceRepository(db *bun.DB) iot.DeviceRepository {
	return &DeviceRepository{
		Repository: base.NewRepository[*iot.Device](db, tableIoTDevices, "Device"),
		db:         db,
	}
}

// FindByDeviceID retrieves a device by its deviceID
func (r *DeviceRepository) FindByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	device := new(iot.Device)
	err := r.db.NewSelect().
		Model(device).
		ModelTableExpr(`iot.devices AS "device"`).
		Where(whereDeviceIDEqual, deviceID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by device ID",
			Err: err,
		}
	}

	return device, nil
}

// FindByAPIKey retrieves a device by its API key
func (r *DeviceRepository) FindByAPIKey(ctx context.Context, apiKey string) (*iot.Device, error) {
	device := new(iot.Device)
	err := r.db.NewSelect().
		Model(device).
		ModelTableExpr(`iot.devices AS "device"`).
		Where("api_key = ?", apiKey).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by API key",
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
		ModelTableExpr(`iot.devices AS "device"`).
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
		ModelTableExpr(`iot.devices AS "device"`).
		Where(whereStatusEqual, status).
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
		ModelTableExpr(`iot.devices AS "device"`).
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
		ModelTableExpr(tableIoTDevices).
		Set("last_seen = ?", lastSeen).
		Where(whereDeviceIDEqual, deviceID).
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
		ModelTableExpr(tableIoTDevices).
		Set(whereStatusEqual, status).
		Where(whereDeviceIDEqual, deviceID).
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
		ModelTableExpr(`iot.devices AS "device"`).
		Where(whereStatusEqual, iot.DeviceStatusActive).
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
		ModelTableExpr(`iot.devices AS "device"`).
		Where(whereStatusEqual, iot.DeviceStatusMaintenance).
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
		ModelTableExpr(`iot.devices AS "device"`).
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
		ModelTableExpr(`iot.devices AS "device"`).
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
	query := r.db.NewSelect().Model(&devices).ModelTableExpr(`iot.devices AS "device"`)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = applyDeviceFilter(query, field, value)
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

// applyDeviceFilter applies a single filter to the query based on field name
func applyDeviceFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "device_id_like":
		return applyDeviceStringLikeFilter(query, "device_id", value)
	case "name_like":
		return applyDeviceStringLikeFilter(query, "name", value)
	case "status":
		return query.Where(whereStatusEqual, value)
	case "device_type":
		return query.Where("device_type = ?", value)
	case "seen_after":
		return applyDeviceTimeFilter(query, "last_seen", ">", value)
	case "seen_before":
		return applyDeviceTimeFilter(query, "last_seen", "<", value)
	case "has_name":
		return applyHasNameFilter(query, value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyDeviceStringLikeFilter applies LIKE filter for string fields
func applyDeviceStringLikeFilter(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where(column+" ILIKE ?", "%"+strValue+"%")
	}
	return query
}

// applyDeviceTimeFilter applies time comparison filter
func applyDeviceTimeFilter(query *bun.SelectQuery, column, operator string, value interface{}) *bun.SelectQuery {
	if timeValue, ok := value.(time.Time); ok {
		return query.Where(column+" "+operator+" ?", timeValue)
	}
	return query
}

// applyHasNameFilter applies NULL/NOT NULL filter for name field
func applyHasNameFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if boolValue, ok := value.(bool); ok {
		if boolValue {
			return query.Where("name IS NOT NULL")
		}
		return query.Where("name IS NULL")
	}
	return query
}
