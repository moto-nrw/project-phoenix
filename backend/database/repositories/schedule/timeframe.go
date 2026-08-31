package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// TimeframeRepository implements schedule.TimeframeRepository interface
type TimeframeRepository struct {
	*base.Repository[*schedule.Timeframe]
	db *bun.DB
}

// NewTimeframeRepository creates a new TimeframeRepository
func NewTimeframeRepository(db *bun.DB) schedule.TimeframeRepository {
	repo := base.NewRepository[*schedule.Timeframe](db, "schedule.timeframes", "Timeframe")
	repo.TenantScoped = true
	return &TimeframeRepository{
		Repository: repo,
		db:         db,
	}
}

// FindActive finds all active timeframes
func (r *TimeframeRepository) FindActive(ctx context.Context) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where("is_active = ?", true)

	query = base.WithTenantFilter(ctx, query, "timeframe")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: base.TranslateNotFound(err),
		}
	}

	return timeframes, nil
}

// FindByTimeRange finds all timeframes that overlap with the given time range
func (r *TimeframeRepository) FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where("start_time <= ?", endTime)

	query = base.WithTenantFilter(ctx, query, "timeframe")

	// Handle open-ended timeframes (no end_time) differently
	query = query.Where("(end_time IS NULL OR end_time >= ?)", startTime)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: base.TranslateNotFound(err),
		}
	}

	return timeframes, nil
}

// FindByDescription finds timeframes with matching description
func (r *TimeframeRepository) FindByDescription(ctx context.Context, description string) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where("LOWER(description) LIKE LOWER(?)", "%"+description+"%")

	query = base.WithTenantFilter(ctx, query, "timeframe")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by description",
			Err: base.TranslateNotFound(err),
		}
	}

	return timeframes, nil
}

// List retrieves timeframes matching the provided query options
func (r *TimeframeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.Timeframe, error) {
	return r.ListWithOptions(ctx, options)
}
