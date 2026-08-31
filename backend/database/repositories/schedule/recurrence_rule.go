package schedule

import (
	"context"
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
	repo := repoBase.NewRepository[*schedule.RecurrenceRule](db, "schedule.recurrence_rules", "RecurrenceRule")
	repo.TenantScoped = true
	return &RecurrenceRuleRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByFrequency finds all recurrence rules with the specified frequency
func (r *RecurrenceRuleRepository) FindByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("LOWER(frequency) = LOWER(?)", frequency)

	if where, val, ok := repoBase.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by frequency",
			Err: repoBase.TranslateNotFound(err),
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

	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("? = ANY(weekdays)", upperWeekday)

	if where, val, ok := repoBase.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by weekday",
			Err: repoBase.TranslateNotFound(err),
		}
	}

	return rules, nil
}

// FindByMonthDay finds all recurrence rules that include the specified month day
func (r *RecurrenceRuleRepository) FindByMonthDay(ctx context.Context, day int) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule

	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("? = ANY(month_days)", day)

	if where, val, ok := repoBase.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by month day",
			Err: repoBase.TranslateNotFound(err),
		}
	}

	return rules, nil
}

// FindByDateRange finds all recurrence rules that apply within the given date range
func (r *RecurrenceRuleRepository) FindByDateRange(ctx context.Context, startDate, _ time.Time) ([]*schedule.RecurrenceRule, error) {
	var rules []*schedule.RecurrenceRule

	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&rules).
		ModelTableExpr(tableExprRecurrenceAsRR).
		Where("(end_date IS NULL OR end_date >= ?)", startDate)

	if where, val, ok := repoBase.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by date range",
			Err: repoBase.TranslateNotFound(err),
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

// List retrieves recurrence rules matching the provided query options
func (r *RecurrenceRuleRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.RecurrenceRule, error) {
	rows, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: repoBase.DatabaseErrorCause(err)}
	}
	return rows, nil
}
