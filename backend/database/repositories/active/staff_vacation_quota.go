package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableStaffVacationQuota = "active.staff_vacation_quota"

type StaffVacationQuotaRepository struct {
	*base.Repository[*active.StaffVacationQuota]
	db *bun.DB
}

func NewStaffVacationQuotaRepository(db *bun.DB) active.StaffVacationQuotaRepository {
	repo := base.NewRepository[*active.StaffVacationQuota](db, tableStaffVacationQuota, "StaffVacationQuota")
	repo.TenantScoped = true
	return &StaffVacationQuotaRepository{Repository: repo, db: db}
}

func (r *StaffVacationQuotaRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffVacationQuota, error) {
	rows, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list vacation quotas", Err: base.DatabaseErrorCause(err)}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows, nil
}

func (r *StaffVacationQuotaRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*active.StaffVacationQuota, error) {
	quota := &active.StaffVacationQuota{}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(quota).
		ModelTableExpr(tableStaffVacationQuota).
		Where("staff_id = ?", staffID).
		Where("year = ?", year).
		Limit(1)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx)
	if err != nil {
		// Surface "no row" as nil, nil so the service can fall back to the
		// tenant default rather than a 404.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		// String fallback for drivers that wrap the error: some bun configs
		// don't surface sql.ErrNoRows directly.
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get vacation quota by staff+year", Err: err}
	}
	return quota, nil
}

func (r *StaffVacationQuotaRepository) Upsert(ctx context.Context, quota *active.StaffVacationQuota) error {
	if quota == nil {
		return fmt.Errorf("quota cannot be nil")
	}
	if err := quota.Validate(); err != nil {
		return err
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(quota).
		ModelTableExpr(tableStaffVacationQuota).
		On("CONFLICT (staff_id, year) DO UPDATE").
		Set("entitled_days = EXCLUDED.entitled_days").
		Set("carryover_days = EXCLUDED.carryover_days").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert vacation quota", Err: err}
	}
	return nil
}

// GetByStaffIDsAndYear is GetByStaffAndYear for many staff members in one round
// trip, keyed by staff ID. A batched IN-lookup the generic filter API cannot
// express as a single query. Staff members without a quota row are absent from
// the map; callers fall back to the tenant default exactly as they would for
// the single-staff (nil, nil).
func (r *StaffVacationQuotaRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*active.StaffVacationQuota, error) {
	result := make(map[int64]*active.StaffVacationQuota, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var quotas []*active.StaffVacationQuota
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&quotas).
		ModelTableExpr(tableStaffVacationQuota).
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Where("year = ?", year)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get vacation quotas by staff IDs+year", Err: err}
	}
	for _, quota := range quotas {
		result[quota.StaffID] = quota
	}
	return result, nil
}
