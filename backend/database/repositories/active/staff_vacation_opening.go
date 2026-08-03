package active

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableStaffVacationOpening = "active.staff_vacation_openings"
	// bun qualifies model columns with the struct-derived default alias
	// (singular). The plural table name does NOT match it, so every custom
	// query must alias explicitly — quoted, per the BUN alias rule.
	tableStaffVacationOpeningAliased = tableStaffVacationOpening + ` AS "staff_vacation_opening"`
)

type StaffVacationOpeningRepository struct {
	*base.Repository[*active.StaffVacationOpening]
	db *bun.DB
}

func NewStaffVacationOpeningRepository(db *bun.DB) active.StaffVacationOpeningRepository {
	repo := base.NewRepository[*active.StaffVacationOpening](db, tableStaffVacationOpening, "StaffVacationOpening")
	repo.TenantScoped = true
	return &StaffVacationOpeningRepository{Repository: repo, db: db}
}

// List delegates to the generic ListWithOptions, which builds the aliased
// table expression itself. That alias is load-bearing here: bun qualifies
// model columns with the struct-derived singular alias, which the plural
// table name does not match.
func (r *StaffVacationOpeningRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffVacationOpening, error) {
	return r.ListWithOptions(ctx, options)
}

func (r *StaffVacationOpeningRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*active.StaffVacationOpening, error) {
	opening := &active.StaffVacationOpening{}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(opening).
		ModelTableExpr(tableStaffVacationOpeningAliased).
		Where("staff_id = ?", staffID).
		Where("year = ?", year).
		Limit(1)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get vacation opening by staff+year", Err: err}
	}
	return opening, nil
}

func (r *StaffVacationOpeningRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*active.StaffVacationOpening, error) {
	result := make(map[int64]*active.StaffVacationOpening, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var openings []*active.StaffVacationOpening
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&openings).
		ModelTableExpr(tableStaffVacationOpeningAliased).
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Where("year = ?", year)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get vacation openings by staff IDs+year", Err: err}
	}
	for _, opening := range openings {
		result[opening.StaffID] = opening
	}
	return result, nil
}
