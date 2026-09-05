package schedule

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tableActivityExceptions   = "schedule.activity_exceptions"
	aliasActivityException    = "activity_exception"
	modelTblActivityException = `schedule.activity_exceptions AS "activity_exception"`
	orderExceptionDateASC     = "exception_date ASC"
)

// ActivityExceptionRepository implements schedule.ActivityExceptionRepository.
type ActivityExceptionRepository struct {
	*base.Repository[*schedule.ActivityException]
	db *bun.DB
}

// NewActivityExceptionRepository creates a new ActivityExceptionRepository.
func NewActivityExceptionRepository(db *bun.DB) schedule.ActivityExceptionRepository {
	repo := base.NewRepository[*schedule.ActivityException](db, tableActivityExceptions, "ActivityException")
	repo.TenantScoped = true
	return &ActivityExceptionRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *ActivityExceptionRepository) FindByID(ctx context.Context, id any) (*schedule.ActivityException, error) {
	var row schedule.ActivityException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblActivityException).
		Where(`"activity_exception".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, aliasActivityException)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: base.TranslateNotFound(err),
		}
	}
	return &row, nil
}

// List retrieves activity exceptions matching the provided query options.
func (r *ActivityExceptionRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.ActivityException, error) {
	return r.ListWithOptions(ctx, options)
}

// FindByActivityGroupID returns all exceptions for the given template.
func (r *ActivityExceptionRepository) FindByActivityGroupID(ctx context.Context, activityGroupID int64) ([]*schedule.ActivityException, error) {
	var rows []*schedule.ActivityException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblActivityException).
		Where(`"activity_exception".activity_group_id = ?`, activityGroupID).
		Order(orderExceptionDateASC)

	query = base.WithTenantFilter(ctx, query, aliasActivityException)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by activity group id",
			Err: base.TranslateNotFound(err),
		}
	}
	return rows, nil
}

// FindByActivityGroupAndDate returns the exception for a specific
// (template, date) pair, or nil if none exists.
func (r *ActivityExceptionRepository) FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date schedule.Date) (*schedule.ActivityException, error) {
	var row schedule.ActivityException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblActivityException).
		Where(`"activity_exception".activity_group_id = ?`, activityGroupID).
		Where(`"activity_exception".exception_date = ?`, date)

	query = base.WithTenantFilter(ctx, query, aliasActivityException)

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by activity group and date",
			Err: base.TranslateNotFound(err),
		}
	}
	return &row, nil
}

// FindByActivityGroupAndDateRange returns one template's exceptions within an
// inclusive range. This custom read is necessary because the generic exact-
// filter shape cannot express inclusive date bounds; both the group and tenant
// predicates stay in SQL.
func (r *ActivityExceptionRepository) FindByActivityGroupAndDateRange(
	ctx context.Context,
	activityGroupID int64,
	from, to schedule.Date,
) ([]*schedule.ActivityException, error) {
	var rows []*schedule.ActivityException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblActivityException).
		Where(`"activity_exception".activity_group_id = ?`, activityGroupID).
		Where(`"activity_exception".exception_date >= ?`, from).
		Where(`"activity_exception".exception_date <= ?`, to).
		Order(orderExceptionDateASC)

	query = base.WithTenantFilter(ctx, query, aliasActivityException)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by activity group and date range",
			Err: base.TranslateNotFound(err),
		}
	}
	return rows, nil
}

// FindByDateRange returns all exceptions within an inclusive date range.
func (r *ActivityExceptionRepository) FindByDateRange(ctx context.Context, from, to schedule.Date) ([]*schedule.ActivityException, error) {
	var rows []*schedule.ActivityException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblActivityException).
		Where(`"activity_exception".exception_date >= ?`, from).
		Where(`"activity_exception".exception_date <= ?`, to).
		Order(orderExceptionDateASC)

	query = base.WithTenantFilter(ctx, query, aliasActivityException)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by date range",
			Err: base.TranslateNotFound(err),
		}
	}
	return rows, nil
}
