package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/uptrace/bun"
)

const (
	deviceTableExpr  = `iot.devices AS "device"`
	deviceNotArchive = `"device".archived_at IS NULL`
)

// deviceRow is this owner's private mapping of iot.devices. It never leaves
// the adapter.
type deviceRow struct {
	bun.BaseModel `bun:"table:devices,alias:device"`

	ID                    int64      `bun:"id,pk,autoincrement"`
	TenantID              int64      `bun:"tenant_id,notnull"`
	CreatedAt             time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	DeviceID              string     `bun:"device_id,notnull"`
	DeviceType            string     `bun:"device_type,notnull"`
	Name                  *string    `bun:"name"`
	Status                string     `bun:"status,notnull,default:'active'"`
	APIKey                *string    `bun:"api_key"`
	LastSeen              *time.Time `bun:"last_seen"`
	RegisteredByID        *int64     `bun:"registered_by_id"`
	RoomID                *int64     `bun:"room_id"`
	ArchivedAt            *time.Time `bun:"archived_at"`
	TransferredToDeviceID *int64     `bun:"transferred_to_device_id"`
}

// DeviceStore reads and writes iot.devices for the Device Fleet owner.
type DeviceStore struct{ database Database }

// NewDeviceStore builds the device adapter over the supplied runtime.
func NewDeviceStore(database Database) *DeviceStore {
	if database == nil {
		panic("devicefleet postgres: database runtime is required")
	}
	return &DeviceStore{database: database}
}

func (s *DeviceStore) selectDevices(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := db.NewSelect().
		Model(model).
		ModelTableExpr(deviceTableExpr).
		ColumnExpr(`"device".*`).
		Where(deviceNotArchive)
	if tenantID > 0 {
		query = query.Where(`"device".tenant_id = ?`, tenantID)
	}
	return query
}

// Create inserts one device and returns the stored row.
func (s *DeviceStore) Create(ctx context.Context, input domain.CreateDevice) (domain.Device, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Device{}, domain.OperationStats{}, err
	}
	row := deviceRow{
		TenantID: tenantID, DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: string(input.Status), APIKey: input.APIKey, LastSeen: input.LastSeen,
		RegisteredByID: input.RegisteredByID, RoomID: input.RoomID,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`iot.devices`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Device{}, stats, fmt.Errorf("devicefleet postgres: create device: %w", err)
	}
	stats.Rows = 1
	return toDeviceDomain(row), stats, nil
}

// Update replaces every writable column of one device.
func (s *DeviceStore) Update(ctx context.Context, input domain.UpdateDevice) (domain.Device, domain.OperationStats, error) {
	columns := []string{
		"device_id", "device_type", "name", "status", "api_key", "last_seen",
		"registered_by_id", "room_id", "archived_at", "transferred_to_device_id", "updated_at",
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Device{}, domain.OperationStats{}, err
	}
	row := deviceRowFromUpdate(input)
	if row.UpdatedAt.IsZero() {
		row.UpdatedAt = time.Now()
	}
	query := db.NewUpdate().Model(&row).
		ModelTableExpr(deviceTableExpr).
		Column(columns...).
		Where(`"device".id = ?`, input.ID)
	if tenantID > 0 {
		query = query.Where(`"device".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Device{}, stats, domain.ErrDeviceNotFound
	}
	if err != nil {
		return domain.Device{}, stats, fmt.Errorf("devicefleet postgres: update device: %w", err)
	}
	stats.Rows = 1
	return toDeviceDomain(row), stats, nil
}

// UpdateColumns writes exactly the named columns of one device and reports
// how many rows matched.
func (s *DeviceStore) UpdateColumns(ctx context.Context, input domain.UpdateDevice, columns []string) (int64, domain.OperationStats, error) {
	if len(columns) == 0 {
		return 0, domain.OperationStats{}, errors.New("devicefleet postgres: no columns to update")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	row := deviceRowFromUpdate(input)
	query := db.NewUpdate().Model(&row).
		ModelTableExpr(deviceTableExpr).
		Column(columns...).
		Where(`"device".id = ?`, input.ID)
	if tenantID > 0 {
		query = query.Where(`"device".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: update device columns: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: count updated devices: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// UpdateStatusByDeviceID sets the status of one live device addressed by its
// tenant-unique device_id.
func (s *DeviceStore) UpdateStatusByDeviceID(ctx context.Context, deviceID string, status domain.DeviceStatus) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().Model((*deviceRow)(nil)).
		ModelTableExpr(`iot.devices`).
		Set("status = ?", string(status)).
		Where("device_id = ?", deviceID).
		Where("archived_at IS NULL")
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: update device status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: count status updates: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// Delete removes one device row and reports how many rows matched.
func (s *DeviceStore) Delete(ctx context.Context, id int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*deviceRow)(nil)).
		ModelTableExpr(deviceTableExpr).
		Where(`"device".id = ?`, id)
	if tenantID > 0 {
		query = query.Where(`"device".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: delete device: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: count deleted devices: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// FindByID reads one live device, optionally taking the supplied row lock.
func (s *DeviceStore) FindByID(ctx context.Context, id int64, lock string) (domain.Device, bool, domain.OperationStats, error) {
	return s.findOne(ctx, "find device", lock, func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"device".id = ?`, id)
	})
}

// FindByDeviceID reads one live device by its tenant-unique device_id.
func (s *DeviceStore) FindByDeviceID(ctx context.Context, deviceID string) (domain.Device, bool, domain.OperationStats, error) {
	return s.findOne(ctx, "find device by device id", "", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"device".device_id = ?`, deviceID)
	})
}

// FindByAPIKey reads one live device by its API key. Device authentication
// calls this before any tenant is known, so the tenant predicate is absent
// unless the caller already established one.
func (s *DeviceStore) FindByAPIKey(ctx context.Context, apiKey string) (domain.Device, bool, domain.OperationStats, error) {
	return s.findOne(ctx, "find device by api key", "", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"device".api_key = ?`, apiKey)
	})
}

func (s *DeviceStore) findOne(
	ctx context.Context,
	operation string,
	lock string,
	restrict func(*bun.SelectQuery) *bun.SelectQuery,
) (domain.Device, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Device{}, false, domain.OperationStats{}, err
	}
	row := deviceRow{}
	query := restrict(s.selectDevices(db, tenantID, &row))
	if lock != "" {
		query = query.For(lock)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Device{}, false, stats, nil
	}
	if err != nil {
		return domain.Device{}, false, stats, fmt.Errorf("devicefleet postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return toDeviceDomain(row), true, stats, nil
}

// List reads every live device matching filter.
func (s *DeviceStore) List(ctx context.Context, filter domain.DeviceFilter) ([]domain.Device, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []deviceRow
	query := applyDeviceFilter(s.selectDevices(db, tenantID, &rows), filter)
	return s.scanList(ctx, query, &rows, "list devices")
}

// ListByIDs reads the live devices visible for the given primary keys.
func (s *DeviceStore) ListByIDs(ctx context.Context, ids []int64) ([]domain.Device, domain.OperationStats, error) {
	if len(ids) == 0 {
		return []domain.Device{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []deviceRow
	query := s.selectDevices(db, tenantID, &rows).Where(`"device".id IN (?)`, bun.List(ids))
	return s.scanList(ctx, query, &rows, "list devices by id")
}

// ListOffline reads devices last seen before cutoff, including devices that
// were never seen and created before it.
func (s *DeviceStore) ListOffline(ctx context.Context, cutoff time.Time) ([]domain.Device, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []deviceRow
	query := s.selectDevices(db, tenantID, &rows).
		Where(`"device".last_seen < ? OR ("device".last_seen IS NULL AND "device".created_at < ?)`, cutoff, cutoff)
	return s.scanList(ctx, query, &rows, "list offline devices")
}

func (s *DeviceStore) scanList(
	ctx context.Context,
	query *bun.SelectQuery,
	rows *[]deviceRow,
	operation string,
) ([]domain.Device, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("devicefleet postgres: %s: %w", operation, err)
	}
	stats.Rows = int64(len(*rows))
	devices := make([]domain.Device, 0, len(*rows))
	for _, row := range *rows {
		devices = append(devices, toDeviceDomain(row))
	}
	return devices, stats, nil
}

// CountByType counts live devices grouped by device_type, most frequent first.
func (s *DeviceStore) CountByType(ctx context.Context) (map[string]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var counts []struct {
		DeviceType string `bun:"device_type"`
		Count      int    `bun:"count"`
	}
	query := db.NewSelect().
		Model((*deviceRow)(nil)).
		ModelTableExpr(deviceTableExpr).
		Column("device_type").
		ColumnExpr("COUNT(*) AS count").
		Where(deviceNotArchive)
	if tenantID > 0 {
		query = query.Where(`"device".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Group("device_type").Order("count DESC").Scan(ctx, &counts)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("devicefleet postgres: count devices by type: %w", err)
	}
	stats.Rows = int64(len(counts))
	result := make(map[string]int, len(counts))
	for _, count := range counts {
		result[count.DeviceType] = count.Count
	}
	return result, stats, nil
}

// applyDeviceFilter builds the predicates from fixed columns only, so every
// device query resolves to iot.devices statically.
func applyDeviceFilter(query *bun.SelectQuery, filter domain.DeviceFilter) *bun.SelectQuery {
	if filter.DeviceIDContains != nil {
		query = query.Where(`"device".device_id ILIKE ?`, "%"+*filter.DeviceIDContains+"%")
	}
	if filter.NameContains != nil {
		query = query.Where(`"device".name ILIKE ?`, "%"+*filter.NameContains+"%")
	}
	if filter.Status != nil {
		query = query.Where(`"device".status = ?`, string(*filter.Status))
	}
	if filter.DeviceType != nil {
		query = query.Where(`"device".device_type = ?`, *filter.DeviceType)
	}
	if filter.ExcludeDeviceType != nil {
		query = query.Where(`"device".device_type != ?`, *filter.ExcludeDeviceType)
	}
	if filter.ExcludeDeviceID != nil {
		query = query.Where(`"device".device_id != ?`, *filter.ExcludeDeviceID)
	}
	if filter.SeenAfter != nil {
		query = query.Where(`"device".last_seen > ?`, *filter.SeenAfter)
	}
	if filter.SeenBefore != nil {
		query = query.Where(`"device".last_seen < ?`, *filter.SeenBefore)
	}
	if filter.RoomID != nil {
		query = query.Where(`"device".room_id = ?`, *filter.RoomID)
	}
	if filter.RegisteredByID != nil {
		query = query.Where(`"device".registered_by_id = ?`, *filter.RegisteredByID)
	}
	if filter.HasName != nil {
		if *filter.HasName {
			query = query.Where(`"device".name IS NOT NULL`)
		} else {
			query = query.Where(`"device".name IS NULL`)
		}
	}
	return query
}

func deviceRowFromUpdate(input domain.UpdateDevice) deviceRow {
	return deviceRow{
		ID: input.ID, UpdatedAt: input.UpdatedAt, DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: string(input.Status), APIKey: input.APIKey, LastSeen: input.LastSeen,
		RegisteredByID: input.RegisteredByID, RoomID: input.RoomID, ArchivedAt: input.ArchivedAt,
		TransferredToDeviceID: input.TransferredToDeviceID,
	}
}

func toDeviceDomain(row deviceRow) domain.Device {
	return domain.Device{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		DeviceID: row.DeviceID, DeviceType: row.DeviceType, Name: row.Name,
		Status: domain.DeviceStatus(row.Status), APIKey: row.APIKey, LastSeen: row.LastSeen,
		RegisteredByID: row.RegisteredByID, RoomID: row.RoomID, ArchivedAt: row.ArchivedAt,
		TransferredToDeviceID: row.TransferredToDeviceID,
	}
}
