package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Constants to avoid duplicate string literals (S1192)
const (
	tableIoTDevices    = "iot.devices"
	whereDeviceIDEqual = "device_id = ?"
	whereStatusEqual   = "status = ?"
	whereNotArchived   = `"device".archived_at IS NULL`
)

// DeviceRepository implements iot.DeviceRepository interface
type DeviceRepository struct {
	*base.Repository[*iot.Device]
	db *bun.DB
}

// NewDeviceRepository creates a new DeviceRepository
func NewDeviceRepository(db *bun.DB) iot.DeviceRepository {
	repo := base.NewRepository[*iot.Device](db, tableIoTDevices, "Device")
	repo.TenantScoped = true
	return &DeviceRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByID retrieves a device by its primary key, including room name via JOIN.
// Overrides base.Repository.FindByID which doesn't include the room JOIN.
func (r *DeviceRepository) FindByID(ctx context.Context, id interface{}) (*iot.Device, error) {
	device := new(iot.Device)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(device).
		ModelTableExpr(`iot.devices AS "device"`).
		ColumnExpr(`"device".*`).
		ColumnExpr(`"room".name AS room_name`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "device".room_id AND "room".tenant_id = "device".tenant_id`).
		Where(`"device".id = ?`, id).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return device, nil
}

// FindByIDForUpdate retrieves and locks a current device for a transfer.
// The archived predicate is re-evaluated after a concurrent row lock wait, so
// at most one transfer can archive a source device.
func (r *DeviceRepository) FindByIDForUpdate(ctx context.Context, id int64) (*iot.Device, error) {
	device := new(iot.Device)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(device).
		ModelTableExpr(`iot.devices AS "device"`).
		ColumnExpr(`"device".*`).
		Where(`"device".id = ?`, id).
		Where(whereNotArchived).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "device")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id for update", Err: err}
	}
	return device, nil
}

// FindByDeviceID retrieves a device by its deviceID
func (r *DeviceRepository) FindByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	device := new(iot.Device)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(device).
		ModelTableExpr(`iot.devices AS "device"`).
		ColumnExpr(`"device".*`).
		ColumnExpr(`"room".name AS room_name`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "device".room_id AND "room".tenant_id = "device".tenant_id`).
		Where(whereDeviceIDEqual, deviceID).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)

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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(device).
		ModelTableExpr(`iot.devices AS "device"`).
		Where("api_key = ?", apiKey).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)

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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(`iot.devices AS "device"`).
		Where("device_type = ?", deviceType).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)

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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(`iot.devices AS "device"`).
		Where(whereStatusEqual, status).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)

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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(`iot.devices AS "device"`).
		Where("registered_by_id = ?", personID).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by registered by",
			Err: err,
		}
	}

	return devices, nil
}

// UpdateLastSeen updates the last seen timestamp for a device by its primary key.
// Uses the integer PK (globally unique) rather than device_id (unique per tenant)
// to ensure cross-tenant safety when called from device auth without tenant context.
func (r *DeviceRepository) UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	device := &iot.Device{Model: modelBase.Model{ID: id}, LastSeen: &lastSeen}
	n, err := r.UpdateColumns(ctx, device, "last_seen")
	if err != nil {
		return err
	}
	if n != 1 {
		return &modelBase.DatabaseError{
			Op:  "update last seen",
			Err: fmt.Errorf("expected 1 rows affected, got %d", n),
		}
	}
	return nil
}

// UpdateRoomID updates the room_id for a device by its primary key.
func (r *DeviceRepository) UpdateRoomID(ctx context.Context, id int64, roomID int64) error {
	device := &iot.Device{Model: modelBase.Model{ID: id, UpdatedAt: time.Now()}, RoomID: &roomID}
	n, err := r.UpdateColumns(ctx, device, "room_id", "updated_at")
	if err != nil {
		return err
	}
	if n != 1 {
		return &modelBase.DatabaseError{
			Op:  "update room id",
			Err: fmt.Errorf("expected 1 rows affected, got %d", n),
		}
	}
	return nil
}

// UpdateStatus updates the status for a device
func (r *DeviceRepository) UpdateStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*iot.Device)(nil)).
		ModelTableExpr(tableIoTDevices).
		Set(whereStatusEqual, status).
		Where(whereDeviceIDEqual, deviceID).
		Where("archived_at IS NULL")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update status",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update status")
}

// FindOfflineDevices retrieves devices that have been offline for at least the specified duration
func (r *DeviceRepository) FindOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]*iot.Device, error) {
	cutoffTime := time.Now().Add(-offlineSince)

	var devices []*iot.Device
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(`iot.devices AS "device"`).
		Where("last_seen < ? OR (last_seen IS NULL AND created_at < ?)", cutoffTime, cutoffTime).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.Scan(ctx)

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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*iot.Device)(nil)).
		ModelTableExpr(`iot.devices AS "device"`).
		Column("device_type").
		ColumnExpr("COUNT(*) AS count").
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

	err := query.
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

// List retrieves devices matching the provided filters
func (r *DeviceRepository) List(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	var devices []*iot.Device
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(`iot.devices AS "device"`).
		ColumnExpr(`"device".*`).
		ColumnExpr(`"room".name AS room_name`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "device".room_id AND "room".tenant_id = "device".tenant_id`).
		Where(whereNotArchived)

	query = base.WithTenantFilter(ctx, query, "device")

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
		return applyDeviceStringLikeFilter(query, `"device".name`, value)
	case "status":
		return query.Where(whereStatusEqual, value)
	case "device_type":
		return query.Where("device_type = ?", value)
	case "exclude_device_type":
		return query.Where("device_type != ?", value)
	case "exclude_device_id":
		return query.Where("device_id != ?", value)
	case "seen_after":
		return applyDeviceTimeFilter(query, "last_seen", ">", value)
	case "seen_before":
		return applyDeviceTimeFilter(query, "last_seen", "<", value)
	case "room_id":
		return query.Where(`"device".room_id = ?`, value)
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
			return query.Where(`"device".name IS NOT NULL`)
		}
		return query.Where(`"device".name IS NULL`)
	}
	return query
}
