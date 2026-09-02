package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

type unregisteredTagScanRepository struct {
	runtime Runtime
}

func NewUnregisteredTagScanRepository(runtime Runtime) auditModels.UnregisteredTagScanRepository {
	return &unregisteredTagScanRepository{runtime: requireRuntime(runtime)}
}

func (r *unregisteredTagScanRepository) Create(ctx context.Context, scan *auditModels.UnregisteredTagScan) error {
	return NewAppender(r.runtime).Append(ctx, scan)
}

func (r *unregisteredTagScanRepository) FindByID(ctx context.Context, id int64) (*auditModels.UnregisteredTagScan, error) {
	var scan auditModels.UnregisteredTagScan
	query := r.operatorBaseQuery(ctx).
		Where(`"scan".id = ?`, id)

	if err := query.Scan(ctx, &scan); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapDatabase("find unregistered tag scan", err)
	}
	return &scan, nil
}

func (r *unregisteredTagScanRepository) ListForOperator(ctx context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	var scans []*auditModels.UnregisteredTagScan
	query := r.operatorBaseQuery(ctx)
	applyUnregisteredTagScanFilter(query, filter)
	query = query.OrderExpr(`"scan".scanned_at DESC`).Limit(500)

	if err := query.Scan(ctx, &scans); err != nil {
		return nil, wrapDatabase("list unregistered tag scans", err)
	}
	if scans == nil {
		scans = []*auditModels.UnregisteredTagScan{}
	}
	return scans, nil
}

func (r *unregisteredTagScanRepository) Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
	now := time.Now()
	result, err := runtimeDB(ctx, r.runtime).NewUpdate().
		Model((*auditModels.UnregisteredTagScan)(nil)).
		ModelTableExpr(`audit.unregistered_tag_scans AS "scan"`).
		Set("resolved_at = ?", now).
		Set("resolved_by_operator_id = ?", operatorID).
		Set("resolution_note = ?", note).
		Set("updated_at = ?", now).
		Where(`"scan".id = ?`, id).
		Where(`"scan".resolved_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return nil, wrapDatabase("resolve unregistered tag scan", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, wrapDatabase("resolve unregistered tag scan", err)
	}
	if affected != 1 {
		return nil, wrapDatabase("resolve unregistered tag scan", fmt.Errorf("expected 1 affected row, got %d", affected))
	}
	return r.FindByID(ctx, id)
}

func (r *unregisteredTagScanRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return 0, wrapDatabase("delete old unregistered tag scans", errors.New("tenant context is required"))
	}
	var deleted int64
	err := runtimeDB(ctx, r.runtime).NewRaw(
		`SELECT audit.delete_expired_unregistered_tag_scans(?, ?)`, tenantID, cutoff,
	).Scan(ctx, &deleted)
	if err != nil {
		return 0, wrapDatabase("delete old unregistered tag scans", err)
	}
	return int(deleted), err
}

func (r *unregisteredTagScanRepository) operatorBaseQuery(ctx context.Context) *bun.SelectQuery {
	return runtimeDB(ctx, r.runtime).NewSelect().
		Model((*auditModels.UnregisteredTagScan)(nil)).
		ModelTableExpr(`audit.unregistered_tag_scans AS "scan"`).
		ColumnExpr(`"scan".*`).
		ColumnExpr(`"scan".tenant_id AS school_id`).
		ColumnExpr(`"device".device_id AS device_identifier`).
		ColumnExpr(`"device".name AS device_name`).
		Join(`LEFT JOIN iot.devices AS "device" ON "device".id = "scan".device_id`)
}

func applyUnregisteredTagScanFilter(query *bun.SelectQuery, filter auditModels.UnregisteredTagScanFilter) {
	if filter.SchoolID != nil {
		query.Where(`"scan".tenant_id = ?`, *filter.SchoolID)
	}
	if filter.SchoolIDs != nil {
		query.Where(`"scan".tenant_id IN (?)`, bun.List(filter.SchoolIDs))
	}
	if filter.UnresolvedOnly {
		query.Where(`"scan".resolved_at IS NULL`)
	}
}
