package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const tableClosingDays = "schedule.closing_days"

// ClosingDayRepository implements schedule.ClosingDayRepository
type ClosingDayRepository struct {
	*base.Repository[*schedule.ClosingDay]
	db *bun.DB
}

// NewClosingDayRepository creates a new ClosingDayRepository
func NewClosingDayRepository(db *bun.DB) schedule.ClosingDayRepository {
	repo := base.NewRepository[*schedule.ClosingDay](db, tableClosingDays, "ClosingDay")
	repo.TenantScoped = true
	return &ClosingDayRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByTenantID finds all closing days for the current tenant
func (r *ClosingDayRepository) FindByTenantID(ctx context.Context) ([]*schedule.ClosingDay, error) {
	var days []*schedule.ClosingDay
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&days).
		ModelTableExpr(`schedule.closing_days AS "closing_day"`).
		Order("start_date ASC")

	query = base.WithTenantFilter(ctx, query, "closing_day")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant id",
			Err: err,
		}
	}

	return days, nil
}

// FindOverlappingRange returns all closing days of the current tenant whose
// [start_date, end_date] range overlaps [from, to] (inclusive on both ends).
func (r *ClosingDayRepository) FindOverlappingRange(ctx context.Context, from, to timezone.Date) ([]*schedule.ClosingDay, error) {
	var days []*schedule.ClosingDay
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&days).
		ModelTableExpr(`schedule.closing_days AS "closing_day"`).
		Where(`"closing_day".start_date <= ?`, to).
		Where(`"closing_day".end_date >= ?`, from).
		Order("start_date ASC")

	query = base.WithTenantFilter(ctx, query, "closing_day")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find overlapping range",
			Err: err,
		}
	}

	return days, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *ClosingDayRepository) FindByID(ctx context.Context, id any) (*schedule.ClosingDay, error) {
	var day schedule.ClosingDay

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&day).
		ModelTableExpr(`schedule.closing_days AS "closing_day"`).
		Where(`"closing_day".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "closing_day")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}

	return &day, nil
}

// List retrieves closing days matching the provided query options
func (r *ClosingDayRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.ClosingDay, error) {
	return r.ListWithOptions(ctx, options)
}
