package audit

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type unregisteredTagScanRepository struct {
	*base.Repository[*auditModels.UnregisteredTagScan]
	db *bun.DB
}

func NewUnregisteredTagScanRepository(db *bun.DB) auditModels.UnregisteredTagScanRepository {
	repo := base.NewRepository[*auditModels.UnregisteredTagScan](db, "audit.unregistered_tag_scans", "UnregisteredTagScan")
	repo.TenantScoped = true
	return &unregisteredTagScanRepository{
		Repository: repo,
		db:         db,
	}
}

func (r *unregisteredTagScanRepository) FindByID(ctx context.Context, id int64) (*auditModels.UnregisteredTagScan, error) {
	var scan auditModels.UnregisteredTagScan
	query := r.operatorBaseQuery(ctx).
		Where(`"scan".id = ?`, id)

	if err := query.Scan(ctx, &scan); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find unregistered tag scan", Err: base.TranslateNotFound(err)}
	}
	return &scan, nil
}

func (r *unregisteredTagScanRepository) ListForOperator(ctx context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	var scans []*auditModels.UnregisteredTagScan
	query := r.operatorBaseQuery(ctx)
	applyUnregisteredTagScanFilter(query, filter)
	query = query.OrderExpr(`"scan".scanned_at DESC`).Limit(500)

	if err := query.Scan(ctx, &scans); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list unregistered tag scans", Err: base.TranslateNotFound(err)}
	}
	if scans == nil {
		scans = []*auditModels.UnregisteredTagScan{}
	}
	return scans, nil
}

func (r *unregisteredTagScanRepository) Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
	now := time.Now()
	result, err := base.GetDB(ctx, r.db).NewUpdate().
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
		return nil, &modelBase.DatabaseError{Op: "resolve unregistered tag scan", Err: base.TranslateNotFound(err)}
	}
	if err := base.AssertRowsAffected(result, 1, "resolve unregistered tag scan"); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *unregisteredTagScanRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, &modelBase.DatabaseError{Op: "delete old unregistered tag scans", Err: errors.New("tenant context is required")}
	}
	var deleted int64
	err := base.GetDB(ctx, r.db).NewRaw(
		`SELECT audit.delete_expired_unregistered_tag_scans(?, ?)`, tenantID, cutoff,
	).Scan(ctx, &deleted)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete old unregistered tag scans", Err: err}
	}
	return int(deleted), err
}

func (r *unregisteredTagScanRepository) operatorBaseQuery(ctx context.Context) *bun.SelectQuery {
	return base.GetDB(ctx, r.db).NewSelect().
		Model((*auditModels.UnregisteredTagScan)(nil)).
		ModelTableExpr(`audit.unregistered_tag_scans AS "scan"`).
		ColumnExpr(`"scan".*`).
		ColumnExpr(`"school".id AS school_id`).
		ColumnExpr(`"school".name AS school_name`).
		ColumnExpr(`"org".id AS organization_id`).
		ColumnExpr(`"org".name AS organization_name`).
		ColumnExpr(`"device".device_id AS device_identifier`).
		ColumnExpr(`"device".name AS device_name`).
		Join(`INNER JOIN platform.schools AS "school" ON "school".id = "scan".tenant_id`).
		Join(`INNER JOIN platform.organizations AS "org" ON "org".id = "school".organization_id`).
		Join(`LEFT JOIN iot.devices AS "device" ON "device".id = "scan".device_id`)
}

func applyUnregisteredTagScanFilter(query *bun.SelectQuery, filter auditModels.UnregisteredTagScanFilter) {
	if filter.SchoolID != nil {
		query.Where(`"scan".tenant_id = ?`, *filter.SchoolID)
	}
	if filter.OrganizationID != nil {
		query.Where(`"school".organization_id = ?`, *filter.OrganizationID)
	}
	if filter.UnresolvedOnly {
		query.Where(`"scan".resolved_at IS NULL`)
	}
}
