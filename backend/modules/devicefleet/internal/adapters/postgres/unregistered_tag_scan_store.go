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

const unregisteredTagScanTableExpr = `audit.unregistered_tag_scans AS "scan"`

// unregisteredTagScanRow is this owner's private mapping of
// audit.unregistered_tag_scans. It never leaves the adapter.
type unregisteredTagScanRow struct {
	bun.BaseModel `bun:"table:unregistered_tag_scans,alias:scan"`

	ID                   int64      `bun:"id,pk,autoincrement"`
	TenantID             int64      `bun:"tenant_id,notnull"`
	CreatedAt            time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TagUID               string     `bun:"tag_uid,notnull"`
	DeviceID             *int64     `bun:"device_id"`
	ScannedAt            time.Time  `bun:"scanned_at,notnull"`
	ResolvedAt           *time.Time `bun:"resolved_at"`
	ResolvedByOperatorID *int64     `bun:"resolved_by_operator_id"`
	ResolutionNote       *string    `bun:"resolution_note"`
}

// UnregisteredTagScanStore reads and writes audit.unregistered_tag_scans for
// the Device Fleet owner. Operator reviews run without an ambient tenant and
// then see every tenant the admin transaction may read; device and scheduler
// callers carry a tenant and are bounded by it and by RLS.
type UnregisteredTagScanStore struct{ database Database }

// NewUnregisteredTagScanStore builds the scan adapter over the supplied runtime.
func NewUnregisteredTagScanStore(database Database) *UnregisteredTagScanStore {
	if database == nil {
		panic("devicefleet postgres: database runtime is required")
	}
	return &UnregisteredTagScanStore{database: database}
}

func (s *UnregisteredTagScanStore) selectScans(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := db.NewSelect().
		Model(model).
		ModelTableExpr(unregisteredTagScanTableExpr).
		ColumnExpr(`"scan".*`)
	if tenantID > 0 {
		query = query.Where(`"scan".tenant_id = ?`, tenantID)
	}
	return query
}

// Insert appends one scan for the caller's tenant and returns the stored row.
func (s *UnregisteredTagScanStore) Insert(ctx context.Context, input domain.RecordUnregisteredTagScan) (domain.UnregisteredTagScan, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.UnregisteredTagScan{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.UnregisteredTagScan{}, domain.OperationStats{}, domain.ErrTenantRequired
	}
	row := unregisteredTagScanRow{
		TenantID: tenantID, TagUID: input.TagUID, DeviceID: input.DeviceID, ScannedAt: input.ScannedAt,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`audit.unregistered_tag_scans`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.UnregisteredTagScan{}, stats, fmt.Errorf("devicefleet postgres: record unregistered tag scan: %w", err)
	}
	stats.Rows = 1
	return toUnregisteredTagScanDomain(row), stats, nil
}

// FindByID reads one scan visible to the caller.
func (s *UnregisteredTagScanStore) FindByID(ctx context.Context, id int64) (domain.UnregisteredTagScan, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.UnregisteredTagScan{}, false, domain.OperationStats{}, err
	}
	row := unregisteredTagScanRow{}
	query := s.selectScans(db, tenantID, &row).Where(`"scan".id = ?`, id)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UnregisteredTagScan{}, false, stats, nil
	}
	if err != nil {
		return domain.UnregisteredTagScan{}, false, stats, fmt.Errorf("devicefleet postgres: find unregistered tag scan: %w", err)
	}
	stats.Rows = 1
	return toUnregisteredTagScanDomain(row), true, stats, nil
}

// List reads the scans matching filter, newest first.
func (s *UnregisteredTagScanStore) List(ctx context.Context, filter domain.UnregisteredTagScanFilter) ([]domain.UnregisteredTagScan, domain.OperationStats, error) {
	if filter.TenantIDs != nil && len(filter.TenantIDs) == 0 {
		return []domain.UnregisteredTagScan{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []unregisteredTagScanRow
	query := s.selectScans(db, tenantID, &rows)
	if filter.TenantIDs != nil {
		query = query.Where(`"scan".tenant_id IN (?)`, bun.List(filter.TenantIDs))
	}
	if filter.UnresolvedOnly {
		query = query.Where(`"scan".resolved_at IS NULL`)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = domain.DefaultUnregisteredTagScanLimit
	}
	query = query.OrderExpr(`"scan".scanned_at DESC`).Limit(limit)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("devicefleet postgres: list unregistered tag scans: %w", err)
	}
	stats.Rows = int64(len(rows))
	scans := make([]domain.UnregisteredTagScan, 0, len(rows))
	for _, row := range rows {
		scans = append(scans, toUnregisteredTagScanDomain(row))
	}
	return scans, stats, nil
}

// Resolve stamps one still-open scan and reports how many rows matched.
func (s *UnregisteredTagScanStore) Resolve(ctx context.Context, input domain.ResolveUnregisteredTagScan) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		Model((*unregisteredTagScanRow)(nil)).
		ModelTableExpr(unregisteredTagScanTableExpr).
		Set("resolved_at = ?", input.ResolvedAt).
		Set("resolved_by_operator_id = ?", input.OperatorID).
		Set("resolution_note = ?", input.Note).
		Set("updated_at = ?", input.ResolvedAt).
		Where(`"scan".id = ?`, input.ID).
		Where(`"scan".resolved_at IS NULL`)
	if tenantID > 0 {
		query = query.Where(`"scan".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: resolve unregistered tag scan: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: count resolved unregistered tag scans: %w", err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// DeleteExpired removes the caller tenant's scans older than cutoff through
// audit.delete_expired_unregistered_tag_scans, which enforces the retention
// window for non-superuser roles.
func (s *UnregisteredTagScanStore) DeleteExpired(ctx context.Context, cutoff time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return 0, domain.OperationStats{}, domain.ErrTenantRequired
	}
	var deleted int64
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT audit.delete_expired_unregistered_tag_scans(?, ?)`, tenantID, cutoff).Scan(ctx, &deleted)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: delete expired unregistered tag scans: %w", err)
	}
	stats.Rows = deleted
	return deleted, stats, nil
}

func toUnregisteredTagScanDomain(row unregisteredTagScanRow) domain.UnregisteredTagScan {
	return domain.UnregisteredTagScan{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		TagUID: row.TagUID, DeviceID: row.DeviceID, ScannedAt: row.ScannedAt, ResolvedAt: row.ResolvedAt,
		ResolvedByOperatorID: row.ResolvedByOperatorID, ResolutionNote: row.ResolutionNote,
	}
}
