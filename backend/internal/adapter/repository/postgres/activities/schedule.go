// backend/database/repositories/activities/schedule.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesSchedules          = "activities.schedules"
	tableExprActivitiesSchedulesAsSch = `activities.schedules AS "schedule"`
)

// ScheduleRepository implements activities.ScheduleRepository interface
type ScheduleRepository struct {
	*base.Repository[*activities.Schedule]
	db *bun.DB
}

// NewScheduleRepository creates a new ScheduleRepository
func NewScheduleRepository(db *bun.DB) activities.ScheduleRepository {
	return &ScheduleRepository{
		Repository: base.NewRepository[*activities.Schedule](db, tableActivitiesSchedules, "Schedule"),
		db:         db,
	}
}

// FindByGroupID finds all schedules for a specific group
func (r *ScheduleRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	err := r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		// Removed Timeframe relation since it's not properly defined in the model
		Where("activity_group_id = ?", groupID).
		Order("weekday").
		Order("timeframe_id").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return schedules, nil
}

// FindByWeekday finds all schedules for a specific weekday
func (r *ScheduleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	err := r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		// Note: ActivityGroup relation is commented out in model, so we can't use Relation()
		// The caller should load ActivityGroup separately if needed
		Where("weekday = ?", weekday).
		Order("timeframe_id").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by weekday",
			Err: err,
		}
	}

	return schedules, nil
}

// FindByTimeframeID finds all schedules for a specific timeframe
func (r *ScheduleRepository) FindByTimeframeID(ctx context.Context, timeframeID int64) ([]*activities.Schedule, error) {
	schedules := make([]*activities.Schedule, 0)
	err := r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		// Note: ActivityGroup relation is commented out in model, so we can't use Relation()
		// The caller should load ActivityGroup separately if needed
		Where("timeframe_id = ?", timeframeID).
		Order("weekday").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by timeframe ID",
			Err: err,
		}
	}

	return schedules, nil
}

// Create overrides the base Create method to handle validation
func (r *ScheduleRepository) Create(ctx context.Context, schedule *activities.Schedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule cannot be nil")
	}

	// Validate schedule
	if err := schedule.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, schedule)
}

// Update overrides the base Update method to handle validation
func (r *ScheduleRepository) Update(ctx context.Context, schedule *activities.Schedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule cannot be nil")
	}

	// Validate schedule
	if err := schedule.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(schedule).
		Where(whereIDEquals, schedule.ID).
		ModelTableExpr(tableActivitiesSchedules)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(schedule).
			Where(whereIDEquals, schedule.ID).
			ModelTableExpr(tableActivitiesSchedules)
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *ScheduleRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	query := r.db.NewSelect().Model(&schedules).ModelTableExpr(tableExprActivitiesSchedulesAsSch)

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

	return schedules, nil
}
