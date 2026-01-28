package schedule

import (
	"context"
	"fmt"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// Table name constants for BUN ORM schema qualification
const (
	tableRecurrenceRules    = "schedule.recurrence_rules"
	tableExprRecurrenceAsRR = `schedule.recurrence_rules AS "recurrence_rule"`
)

// RecurrenceRuleRepository implements schedule.RecurrenceRuleRepository interface
type RecurrenceRuleRepository struct {
	*repoBase.Repository[*schedule.RecurrenceRule]
	db *bun.DB
}

// NewRecurrenceRuleRepository creates a new RecurrenceRuleRepository
func NewRecurrenceRuleRepository(db *bun.DB) schedule.RecurrenceRuleRepository {
	return &RecurrenceRuleRepository{
		Repository: repoBase.NewRepository[*schedule.RecurrenceRule](db, "schedule.recurrence_rules", "RecurrenceRule"),
		db:         db,
	}
}

// FindByFrequency finds all recurrence rules with the specified frequency
func (r *RecurrenceRuleRepository) FindByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule
	err := r.db.NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("LOWER(frequency) = LOWER(?)", frequency).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by frequency",
			Err: err,
		}
	}

	return rules, nil
}

// FindByWeekday finds all recurrence rules that include the specified weekday
func (r *RecurrenceRuleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule

	// Convert weekday to uppercase for consistency
	upperWeekday := weekday
	if weekday != "" {
		upperWeekday = weekday
	}

	err := r.db.NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("? = ANY(weekdays)", upperWeekday).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by weekday",
			Err: err,
		}
	}

	return rules, nil
}

// FindByMonthDay finds all recurrence rules that include the specified month day
func (r *RecurrenceRuleRepository) FindByMonthDay(ctx context.Context, day int) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule

	err := r.db.NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("? = ANY(month_days)", day).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by month day",
			Err: err,
		}
	}

	return rules, nil
}

// FindByDateRange finds all recurrence rules that apply within the given date range
func (r *RecurrenceRuleRepository) FindByDateRange(ctx context.Context, startDate, _ time.Time) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule

	err := r.db.NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("(end_date IS NULL OR end_date >= ?)", startDate).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by date range",
			Err: err,
		}
	}

	// We need to filter the results post-query to check if they actually apply
	// during the date range, since determining this purely with SQL is complex
	// and depends on the frequency, interval, weekdays, etc.

	// For a proper implementation, additional filtering logic would be needed here
	// based on the rule's frequency, interval, weekdays, month days, etc.
	// For now, we'll return all rules that either have no end date or end after the start date.

	return rules, nil
}

// Create overrides the base Create method to handle validation
func (r *RecurrenceRuleRepository) Create(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if rule == nil {
		return fmt.Errorf("recurrence rule cannot be nil")
	}

	// Validate rule
	if err := rule.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, rule)
}

// Update overrides the base Update method to handle validation
func (r *RecurrenceRuleRepository) Update(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if rule == nil {
		return fmt.Errorf("recurrence rule cannot be nil")
	}

	// Validate rule
	if err := rule.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, rule)
}

// List retrieves recurrence rules matching the provided query options
func (r *RecurrenceRuleRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.RecurrenceRule, error) {
	rules := make([]*schedule.RecurrenceRule, 0)
	query := r.db.NewSelect().Model(&rules).ModelTableExpr(tableExprRecurrenceAsRR)

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

	return rules, nil
}
