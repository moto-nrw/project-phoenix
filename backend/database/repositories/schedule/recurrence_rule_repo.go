package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// RecurrenceRuleRepository implements schedule.RecurrenceRuleRepository
type RecurrenceRuleRepository struct {
	db *bun.DB
}

// NewRecurrenceRuleRepository creates a new recurrence rule repository
func NewRecurrenceRuleRepository(db *bun.DB) schedule.RecurrenceRuleRepository {
	return &RecurrenceRuleRepository{db: db}
}

// Create inserts a new recurrence rule into the database
func (r *RecurrenceRuleRepository) Create(ctx context.Context, rule *schedule.RecurrenceRule) error {
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
func (r *RecurrenceRuleRepository) FindByID(ctx context.Context, id interface{}) (*schedule.RecurrenceRule, error) {
	rule := new(schedule.RecurrenceRule)
	err := r.db.NewSelect().Model(rule).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return rule, nil
}

// FindByFrequency retrieves recurrence rules by frequency
func (r *RecurrenceRuleRepository) FindByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule
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
func (r *RecurrenceRuleRepository) FindActive(ctx context.Context) ([]*schedule.RecurrenceRule, error) {
	now := time.Now()
	var rules []*schedule.RecurrenceRule
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
func (r *RecurrenceRuleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule
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
func (r *RecurrenceRuleRepository) FindByMonthDay(ctx context.Context, day int) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule
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
func (r *RecurrenceRuleRepository) Update(ctx context.Context, rule *schedule.RecurrenceRule) error {
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
func (r *RecurrenceRuleRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*schedule.RecurrenceRule)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves recurrence rules matching the filters
func (r *RecurrenceRuleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule
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
