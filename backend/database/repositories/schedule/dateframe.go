package schedule

import (
	"context"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// Table name constants for BUN ORM schema qualification
const (
	tableDateframes         = "schedule.dateframes"
	tableExprDateframesAsDF = `schedule.dateframes AS "dateframe"`
)

// DateframeRepository implements schedule.DateframeRepository interface
type DateframeRepository struct {
	*repoBase.Repository[*schedule.Dateframe]
	db *bun.DB
}

// NewDateframeRepository creates a new DateframeRepository
func NewDateframeRepository(db *bun.DB) schedule.DateframeRepository {
	repo := repoBase.NewRepository[*schedule.Dateframe](db, "schedule.dateframes", "Dateframe")
	repo.TenantScoped = true
	return &DateframeRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByName finds a dateframe by its name
func (r *DateframeRepository) FindByName(ctx context.Context, name string) (*schedule.Dateframe, error) {
	dateframe := new(schedule.Dateframe)
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(dateframe).
		ModelTableExpr(tableExprDateframesAsDF).
		Where("LOWER(name) = LOWER(?)", name)

	if where, val, ok := repoBase.TenantWhere(ctx, "dateframe"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return dateframe, nil
}

// FindByDate finds all dateframes that include the given date
func (r *DateframeRepository) FindByDate(ctx context.Context, date time.Time) ([]*schedule.Dateframe, error) {
	var dateframes []*schedule.Dateframe

	// Normalize the date to ignore time component
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&dateframes).
		ModelTableExpr(tableExprDateframesAsDF).
		Where("start_date <= ? AND end_date >= ?", normalizedDate, normalizedDate)

	if where, val, ok := repoBase.TenantWhere(ctx, "dateframe"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by date",
			Err: err,
		}
	}

	return dateframes, nil
}

// FindOverlapping finds all dateframes that overlap with the given date range
func (r *DateframeRepository) FindOverlapping(ctx context.Context, startDate, endDate time.Time) ([]*schedule.Dateframe, error) {
	var dateframes []*schedule.Dateframe

	// Normalize dates to ignore time component
	normalizedStartDate := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	normalizedEndDate := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())

	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&dateframes).
		ModelTableExpr(tableExprDateframesAsDF).
		Where("start_date <= ? AND end_date >= ?", normalizedEndDate, normalizedStartDate)

	if where, val, ok := repoBase.TenantWhere(ctx, "dateframe"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find overlapping",
			Err: err,
		}
	}

	return dateframes, nil
}

// List retrieves dateframes matching the provided query options
func (r *DateframeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.Dateframe, error) {
	dateframes := make([]*schedule.Dateframe, 0)
	query := repoBase.GetDB(ctx, r.db).NewSelect().Model(&dateframes).ModelTableExpr(tableExprDateframesAsDF)

	if where, val, ok := repoBase.TenantWhere(ctx, "dateframe"); ok {
		query = query.Where(where, val)
	}

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return dateframes, nil
}
