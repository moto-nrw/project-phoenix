package activities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// ScheduleRepository implements activities.ScheduleRepository
type ScheduleRepository struct {
	db *bun.DB
}

// NewScheduleRepository creates a new schedule repository
func NewScheduleRepository(db *bun.DB) activities.ScheduleRepository {
	return &ScheduleRepository{db: db}
}

// Create inserts a new schedule into the database
func (r *ScheduleRepository) Create(ctx context.Context, schedule *activities.Schedule) error {
	if err := schedule.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(schedule).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a schedule by its ID
func (r *ScheduleRepository) FindByID(ctx context.Context, id interface{}) (*activities.Schedule, error) {
	schedule := new(activities.Schedule)
	err := r.db.NewSelect().Model(schedule).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return schedule, nil
}

// FindByWeekday retrieves schedules by weekday
func (r *ScheduleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	err := r.db.NewSelect().
		Model(&schedules).
		Where("weekday = ?", weekday).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_weekday", Err: err}
	}
	return schedules, nil
}

// FindByTimeframe retrieves schedules by timeframe
func (r *ScheduleRepository) FindByTimeframe(ctx context.Context, timeframeID int64) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	err := r.db.NewSelect().
		Model(&schedules).
		Where("timeframe_id = ?", timeframeID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_timeframe", Err: err}
	}
	return schedules, nil
}

// FindByActivityGroup retrieves schedules by activity group
func (r *ScheduleRepository) FindByActivityGroup(ctx context.Context, activityGroupID int64) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	err := r.db.NewSelect().
		Model(&schedules).
		Where("activity_group_id = ?", activityGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_activity_group", Err: err}
	}
	return schedules, nil
}

// FindByTimeframeAndWeekday retrieves schedules by timeframe and weekday
func (r *ScheduleRepository) FindByTimeframeAndWeekday(ctx context.Context, timeframeID int64, weekday string) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	err := r.db.NewSelect().
		Model(&schedules).
		Where("timeframe_id = ?", timeframeID).
		Where("weekday = ?", weekday).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_timeframe_and_weekday", Err: err}
	}
	return schedules, nil
}

// Update updates an existing schedule
func (r *ScheduleRepository) Update(ctx context.Context, schedule *activities.Schedule) error {
	if err := schedule.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(schedule).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a schedule
func (r *ScheduleRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*activities.Schedule)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves schedules matching the filters
func (r *ScheduleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	query := r.db.NewSelect().Model(&schedules)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return schedules, nil
}
