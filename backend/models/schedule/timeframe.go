package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Timeframe represents a time period with a start and optional end time
type Timeframe struct {
	base.Model
	StartTime        time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime          *time.Time `bun:"end_time" json:"end_time,omitempty"`
	IsActive         bool       `bun:"is_active,notnull,default:false" json:"is_active"`
	Description      string     `bun:"description" json:"description,omitempty"`
	RecurrenceRuleID *int64     `bun:"recurrence_rule_id" json:"recurrence_rule_id,omitempty"`

	// Relations are defined at query time, not here
}

// TableName returns the table name for the Timeframe model
func (t *Timeframe) TableName() string {
	return "schedule.timeframes"
}

// GetID returns the timeframe ID
func (t *Timeframe) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp
func (t *Timeframe) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (t *Timeframe) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}

// Validate validates the timeframe fields
func (t *Timeframe) Validate() error {
	if t.StartTime.IsZero() {
		return errors.New("start time is required")
	}

	if t.EndTime != nil && !t.EndTime.IsZero() && t.EndTime.Before(t.StartTime) {
		return errors.New("end time must be after start time")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (t *Timeframe) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := t.Model.BeforeAppend(); err != nil {
		return err
	}

	return nil
}

// CheckActive checks if the timeframe is currently active based on its time range
func (t *Timeframe) CheckActive() bool {
	now := time.Now()

	// Check if the timeframe is before the current time
	if t.StartTime.After(now) {
		return false
	}

	// Check if the timeframe has ended
	if t.EndTime != nil && !t.EndTime.IsZero() && t.EndTime.Before(now) {
		return false
	}

	return t.IsActive
}

// TimeframeRepository defines operations for working with timeframes
type TimeframeRepository interface {
	base.Repository[*Timeframe]
	FindActive(ctx context.Context) ([]*Timeframe, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Timeframe, error)
	FindByRecurrenceRule(ctx context.Context, recurrenceRuleID int64) ([]*Timeframe, error)
	UpdateStatus(ctx context.Context, id int64, isActive bool) error
	FindCurrentlyActive(ctx context.Context) ([]*Timeframe, error)
}

// DefaultTimeframeRepository is the default implementation of TimeframeRepository
type DefaultTimeframeRepository struct {
	db *bun.DB
}

// NewTimeframeRepository creates a new timeframe repository
func NewTimeframeRepository(db *bun.DB) TimeframeRepository {
	return &DefaultTimeframeRepository{db: db}
}

// Create inserts a new timeframe into the database
func (r *DefaultTimeframeRepository) Create(ctx context.Context, timeframe *Timeframe) error {
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
func (r *DefaultTimeframeRepository) FindByID(ctx context.Context, id interface{}) (*Timeframe, error) {
	timeframe := new(Timeframe)
	err := r.db.NewSelect().Model(timeframe).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return timeframe, nil
}

// FindActive retrieves all active timeframes
func (r *DefaultTimeframeRepository) FindActive(ctx context.Context) ([]*Timeframe, error) {
	var timeframes []*Timeframe
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
func (r *DefaultTimeframeRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Timeframe, error) {
	var timeframes []*Timeframe
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
func (r *DefaultTimeframeRepository) FindByRecurrenceRule(ctx context.Context, recurrenceRuleID int64) ([]*Timeframe, error) {
	var timeframes []*Timeframe
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
func (r *DefaultTimeframeRepository) UpdateStatus(ctx context.Context, id int64, isActive bool) error {
	_, err := r.db.NewUpdate().
		Model((*Timeframe)(nil)).
		Set("is_active = ?", isActive).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// FindCurrentlyActive retrieves all timeframes that are currently in effect
func (r *DefaultTimeframeRepository) FindCurrentlyActive(ctx context.Context) ([]*Timeframe, error) {
	now := time.Now()
	var timeframes []*Timeframe
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
func (r *DefaultTimeframeRepository) Update(ctx context.Context, timeframe *Timeframe) error {
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
func (r *DefaultTimeframeRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Timeframe)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves timeframes matching the filters
func (r *DefaultTimeframeRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Timeframe, error) {
	var timeframes []*Timeframe
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
