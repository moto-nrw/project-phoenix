package activities

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// Schedule represents an activity schedule
type Schedule struct {
	base.Model
	Weekday         string `bun:"weekday,notnull" json:"weekday"`
	TimeframeID     *int64 `bun:"timeframe_id" json:"timeframe_id,omitempty"`
	ActivityGroupID int64  `bun:"activity_group_id,notnull" json:"activity_group_id"`

	// Relations
	Timeframe     *schedule.Timeframe `bun:"rel:belongs-to,join:timeframe_id=id" json:"timeframe,omitempty"`
	ActivityGroup *Group              `bun:"rel:belongs-to,join:activity_group_id=id" json:"activity_group,omitempty"`
}

// TableName returns the table name for the Schedule model
func (s *Schedule) TableName() string {
	return "activities.schedules"
}

// GetID returns the schedule ID
func (s *Schedule) GetID() interface{} {
	return s.ID
}

// GetCreatedAt returns the creation timestamp
func (s *Schedule) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (s *Schedule) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// Validate validates the schedule fields
func (s *Schedule) Validate() error {
	if strings.TrimSpace(s.Weekday) == "" {
		return errors.New("weekday is required")
	}

	// Validate the weekday value (must be a valid day of the week)
	validWeekdays := map[string]bool{
		"Monday":    true,
		"Tuesday":   true,
		"Wednesday": true,
		"Thursday":  true,
		"Friday":    true,
		"Saturday":  true,
		"Sunday":    true,
	}

	if !validWeekdays[s.Weekday] {
		return errors.New("invalid weekday value")
	}

	if s.ActivityGroupID <= 0 {
		return errors.New("activity group ID is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (s *Schedule) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := s.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	s.Weekday = strings.TrimSpace(s.Weekday)

	return nil
}

// ScheduleRepository defines operations for working with activity schedules
type ScheduleRepository interface {
	base.Repository[*Schedule]
	FindByWeekday(ctx context.Context, weekday string) ([]*Schedule, error)
	FindByTimeframe(ctx context.Context, timeframeID int64) ([]*Schedule, error)
	FindByActivityGroup(ctx context.Context, activityGroupID int64) ([]*Schedule, error)
	FindByTimeframeAndWeekday(ctx context.Context, timeframeID int64, weekday string) ([]*Schedule, error)
}

// DefaultScheduleRepository is the default implementation of ScheduleRepository
type DefaultScheduleRepository struct {
	db *bun.DB
}

// NewScheduleRepository creates a new schedule repository
func NewScheduleRepository(db *bun.DB) ScheduleRepository {
	return &DefaultScheduleRepository{db: db}
}

// Create inserts a new schedule into the database
func (r *DefaultScheduleRepository) Create(ctx context.Context, schedule *Schedule) error {
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
func (r *DefaultScheduleRepository) FindByID(ctx context.Context, id interface{}) (*Schedule, error) {
	schedule := new(Schedule)
	err := r.db.NewSelect().Model(schedule).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return schedule, nil
}

// FindByWeekday retrieves schedules by weekday
func (r *DefaultScheduleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*Schedule, error) {
	var schedules []*Schedule
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
func (r *DefaultScheduleRepository) FindByTimeframe(ctx context.Context, timeframeID int64) ([]*Schedule, error) {
	var schedules []*Schedule
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
func (r *DefaultScheduleRepository) FindByActivityGroup(ctx context.Context, activityGroupID int64) ([]*Schedule, error) {
	var schedules []*Schedule
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
func (r *DefaultScheduleRepository) FindByTimeframeAndWeekday(ctx context.Context, timeframeID int64, weekday string) ([]*Schedule, error) {
	var schedules []*Schedule
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
func (r *DefaultScheduleRepository) Update(ctx context.Context, schedule *Schedule) error {
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
func (r *DefaultScheduleRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Schedule)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves schedules matching the filters
func (r *DefaultScheduleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Schedule, error) {
	var schedules []*Schedule
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
