package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	"github.com/uptrace/bun"
)

// TimeframeRepository implements schedule.TimeframeRepository interface
type TimeframeRepository struct {
	*base.Repository[*schedule.Timeframe]
	db *bun.DB
}

// NewTimeframeRepository creates a new TimeframeRepository
func NewTimeframeRepository(db *bun.DB) schedule.TimeframeRepository {
	return &TimeframeRepository{
		Repository: base.NewRepository[*schedule.Timeframe](db, "schedule.timeframes", "Timeframe"),
		db:         db,
	}
}

// FindActive finds all active timeframes
func (r *TimeframeRepository) FindActive(ctx context.Context) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	err := r.db.NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where("is_active = ?", true).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: err,
		}
	}

	return timeframes, nil
}

// FindByTimeRange finds all timeframes that overlap with the given time range
func (r *TimeframeRepository) FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe

	query := r.db.NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where("start_time <= ?", endTime)

	// Handle open-ended timeframes (no end_time) differently
	query = query.Where("(end_time IS NULL OR end_time >= ?)", startTime)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: err,
		}
	}

	return timeframes, nil
}

// FindByDescription finds timeframes with matching description
func (r *TimeframeRepository) FindByDescription(ctx context.Context, description string) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	err := r.db.NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where("LOWER(description) LIKE LOWER(?)", "%"+description+"%").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by description",
			Err: err,
		}
	}

	return timeframes, nil
}

// Create overrides the base Create method to handle validation
func (r *TimeframeRepository) Create(ctx context.Context, timeframe *schedule.Timeframe) error {
	if timeframe == nil {
		return fmt.Errorf("timeframe cannot be nil")
	}

	// Validate timeframe
	if err := timeframe.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, timeframe)
}

// Update overrides the base Update method to handle validation
func (r *TimeframeRepository) Update(ctx context.Context, timeframe *schedule.Timeframe) error {
	if timeframe == nil {
		return fmt.Errorf("timeframe cannot be nil")
	}

	// Validate timeframe
	if err := timeframe.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, timeframe)
}

// List retrieves timeframes matching the provided query options
func (r *TimeframeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	query := r.db.NewSelect().
		Model(&timeframes).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`)

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

	return timeframes, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *TimeframeRepository) FindByID(ctx context.Context, id interface{}) (*schedule.Timeframe, error) {
	var timeframe schedule.Timeframe

	err := r.db.NewSelect().
		Model(&timeframe).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Where(`"timeframe".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return &timeframe, nil
}
