package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// RecurrenceRule represents a rule for recurring events
type RecurrenceRule struct {
	base.Model
	Frequency     string     `bun:"frequency,notnull" json:"frequency"`
	IntervalCount int        `bun:"interval_count,notnull,default:1" json:"interval_count"`
	Weekdays      []string   `bun:"weekdays,array" json:"weekdays,omitempty"`
	MonthDays     []int      `bun:"month_days,array" json:"month_days,omitempty"`
	EndDate       *time.Time `bun:"end_date" json:"end_date,omitempty"`
	Count         *int       `bun:"count" json:"count,omitempty"`

	// Relations - we don't define backward relations to avoid circular imports
}

// TableName returns the table name for the RecurrenceRule model
func (r *RecurrenceRule) TableName() string {
	return "schedule.recurrence_rules"
}

// GetID returns the recurrence rule ID
func (r *RecurrenceRule) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *RecurrenceRule) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *RecurrenceRule) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}

// Validate validates the recurrence rule fields
func (r *RecurrenceRule) Validate() error {
	if r.Frequency == "" {
		return errors.New("frequency is required")
	}

	// Validate frequency value
	validFrequencies := map[string]bool{
		"daily":   true,
		"weekly":  true,
		"monthly": true,
		"yearly":  true,
	}

	if !validFrequencies[r.Frequency] {
		return errors.New("invalid frequency value")
	}

	if r.IntervalCount <= 0 {
		return errors.New("interval count must be greater than zero")
	}

	// Validate weekdays if provided
	if len(r.Weekdays) > 0 && r.Frequency != "weekly" {
		return errors.New("weekdays are only valid for weekly frequency")
	}

	// Validate month days if provided
	if len(r.MonthDays) > 0 && r.Frequency != "monthly" {
		return errors.New("month days are only valid for monthly frequency")
	}

	// Validate month days values (1-31)
	for _, day := range r.MonthDays {
		if day < 1 || day > 31 {
			return errors.New("month days must be between 1 and 31")
		}
	}

	// Validate that either end date or count is set, not both
	if r.EndDate != nil && !r.EndDate.IsZero() && r.Count != nil {
		return errors.New("only one of end date or count should be set")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (r *RecurrenceRule) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := r.Model.BeforeAppend(); err != nil {
		return err
	}

	// Set default interval count if not provided
	if r.IntervalCount <= 0 {
		r.IntervalCount = 1
	}

	return nil
}

// IsActive checks if the recurrence rule is still active
func (r *RecurrenceRule) IsActive() bool {
	// Check if rule has an end date that has passed
	if r.EndDate != nil && !r.EndDate.IsZero() && r.EndDate.Before(time.Now()) {
		return false
	}

	return true
}

// RecurrenceRuleRepository defines operations for working with recurrence rules
type RecurrenceRuleRepository interface {
	base.Repository[*RecurrenceRule]
	FindByFrequency(ctx context.Context, frequency string) ([]*RecurrenceRule, error)
	FindActive(ctx context.Context) ([]*RecurrenceRule, error)
	FindByWeekday(ctx context.Context, weekday string) ([]*RecurrenceRule, error)
	FindByMonthDay(ctx context.Context, day int) ([]*RecurrenceRule, error)
}

// DefaultRecurrenceRuleRepository is the default implementation of RecurrenceRuleRepository
type DefaultRecurrenceRuleRepository struct {
	db *bun.DB
}

// NewRecurrenceRuleRepository creates a new recurrence rule repository
func NewRecurrenceRuleRepository(db *bun.DB) RecurrenceRuleRepository {
	return &DefaultRecurrenceRuleRepository{db: db}
}

// Create inserts a new recurrence rule into the database
func (r *DefaultRecurrenceRuleRepository) Create(ctx context.Context, rule *RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(rule).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a recurrence rule by its ID
func (r *DefaultRecurrenceRuleRepository) FindByID(ctx context.Context, id interface{}) (*RecurrenceRule, error) {
	rule := new(RecurrenceRule)
	err := r.db.NewSelect().Model(rule).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return rule, nil
}

// FindByFrequency retrieves recurrence rules by frequency
func (r *DefaultRecurrenceRuleRepository) FindByFrequency(ctx context.Context, frequency string) ([]*RecurrenceRule, error) {
	var rules []*RecurrenceRule
	err := r.db.NewSelect().
		Model(&rules).
		Where("frequency = ?", frequency).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_frequency", Err: err}
	}
	return rules, nil
}

// FindActive retrieves all active recurrence rules
func (r *DefaultRecurrenceRuleRepository) FindActive(ctx context.Context) ([]*RecurrenceRule, error) {
	now := time.Now()
	var rules []*RecurrenceRule
	err := r.db.NewSelect().
		Model(&rules).
		Where("end_date IS NULL OR end_date > ?", now).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return rules, nil
}

// FindByWeekday retrieves recurrence rules that include a specific weekday
func (r *DefaultRecurrenceRuleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*RecurrenceRule, error) {
	var rules []*RecurrenceRule
	err := r.db.NewSelect().
		Model(&rules).
		Where("? = ANY(weekdays)", weekday).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_weekday", Err: err}
	}
	return rules, nil
}

// FindByMonthDay retrieves recurrence rules that include a specific month day
func (r *DefaultRecurrenceRuleRepository) FindByMonthDay(ctx context.Context, day int) ([]*RecurrenceRule, error) {
	var rules []*RecurrenceRule
	err := r.db.NewSelect().
		Model(&rules).
		Where("? = ANY(month_days)", day).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_month_day", Err: err}
	}
	return rules, nil
}

// Update updates an existing recurrence rule
func (r *DefaultRecurrenceRuleRepository) Update(ctx context.Context, rule *RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(rule).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a recurrence rule
func (r *DefaultRecurrenceRuleRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*RecurrenceRule)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves recurrence rules matching the filters
func (r *DefaultRecurrenceRuleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*RecurrenceRule, error) {
	var rules []*RecurrenceRule
	query := r.db.NewSelect().Model(&rules)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return rules, nil
}
