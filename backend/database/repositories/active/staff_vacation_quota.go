package active

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	tableStaffVacationQuota          = "active.staff_vacation_quota"
	tableExprStaffVacationQuotaAlias = `active.staff_vacation_quota AS "quota"`
)

type StaffVacationQuotaRepository struct {
	*base.Repository[*active.StaffVacationQuota]
	db *bun.DB
}

func NewStaffVacationQuotaRepository(db *bun.DB) active.StaffVacationQuotaRepository {
	repo := base.NewRepository[*active.StaffVacationQuota](db, tableStaffVacationQuota, "StaffVacationQuota")
	repo.TenantScoped = true
	return &StaffVacationQuotaRepository{Repository: repo, db: db}
}

// List overrides base.List to use the QueryOptions signature the project's
// interface expects (vs the map[string]any default of generic Repository).
func (r *StaffVacationQuotaRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffVacationQuota, error) {
	var rows []*active.StaffVacationQuota
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprStaffVacationQuotaAlias)

	if where, val, ok := base.TenantWhere(ctx, "quota"); ok {
		query = query.Where(where, val)
	}
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list vacation quotas", Err: err}
	}
	return rows, nil
}

func (r *StaffVacationQuotaRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*active.StaffVacationQuota, error) {
	quota := &active.StaffVacationQuota{}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(quota).
		ModelTableExpr(tableExprStaffVacationQuotaAlias).
		Where(`"quota".staff_id = ?`, staffID).
		Where(`"quota".year = ?`, year).
		Limit(1)

	if where, val, ok := base.TenantWhere(ctx, "quota"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		// Surface "no row" as nil, nil so the service can fall back to the
		// tenant default rather than a 404.
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
