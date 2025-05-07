package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// TimeframeRepository implements schedule.TimeframeRepository
type TimeframeRepository struct {
	db *bun.DB
}

// NewTimeframeRepository creates a new timeframe repository
func NewTimeframeRepository(db *bun.DB) schedule.TimeframeRepository {
	return &TimeframeRepository{db: db}
}

// Create inserts a new timeframe into the database
func (r *TimeframeRepository) Create(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(timeframe).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a timeframe by its ID
func (r *TimeframeRepository) FindByID(ctx context.Context, id interface{}) (*schedule.Timeframe, error) {
	timeframe := new(schedule.Timeframe)
	err := r.db.NewSelect().Model(timeframe).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return timeframe, nil
}

// FindActive retrieves all active timeframes
func (r *TimeframeRepository) FindActive(ctx context.Context) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	err := r.db.NewSelect().
		Model(&timeframes).
		Where("is_active = ?", true).
		Order("start_time ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return timeframes, nil
}

// FindByDateRange retrieves all timeframes within a date range
func (r *TimeframeRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	err := r.db.NewSelect().
		Model(&timeframes).
		Where("(start_time BETWEEN ? AND ?) OR (end_time BETWEEN ? AND ?) OR (start_time <= ? AND (end_time IS NULL OR end_time >= ?))",
			startDate, endDate, startDate, endDate, startDate, endDate).
		Order("start_time ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_date_range", Err: err}
	}
	return timeframes, nil
}

// FindByRecurrenceRule retrieves all timeframes with a specific recurrence rule
func (r *TimeframeRepository) FindByRecurrenceRule(ctx context.Context, recurrenceRuleID int64) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	err := r.db.NewSelect().
		Model(&timeframes).
		Where("recurrence_rule_id = ?", recurrenceRuleID).
		Order("start_time ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_recurrence_rule", Err: err}
	}
	return timeframes, nil
}

// UpdateStatus updates the active status of a timeframe
func (r *TimeframeRepository) UpdateStatus(ctx context.Context, id int64, isActive bool) error {
	_, err := r.db.NewUpdate().
		Model((*schedule.Timeframe)(nil)).
		Set("is_active = ?", isActive).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// FindCurrentlyActive retrieves all timeframes that are currently in effect
func (r *TimeframeRepository) FindCurrentlyActive(ctx context.Context) ([]*schedule.Timeframe, error) {
	now := time.Now()
	var timeframes []*schedule.Timeframe
	err := r.db.NewSelect().
		Model(&timeframes).
		Where("is_active = ?", true).
		Where("start_time <= ?", now).
		Where("end_time IS NULL OR end_time >= ?", now).
		Order("start_time ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_currently_active", Err: err}
	}
	return timeframes, nil
}

// Update updates an existing timeframe
func (r *TimeframeRepository) Update(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(timeframe).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a timeframe
func (r *TimeframeRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*schedule.Timeframe)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves timeframes matching the filters
func (r *TimeframeRepository) List(ctx context.Context, filters map[string]interface{}) ([]*schedule.Timeframe, error) {
	var timeframes []*schedule.Timeframe
	query := r.db.NewSelect().Model(&timeframes)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return timeframes, nil
}
