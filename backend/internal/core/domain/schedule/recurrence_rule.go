package schedule

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// Frequency constants for recurrence rules
const (
	FrequencyDaily   = "daily"
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
	FrequencyYearly  = "yearly"
)

// tableScheduleRecurrenceRules is the schema-qualified table name for recurrence rules
const tableScheduleRecurrenceRules = "schedule.recurrence_rules"

// Valid weekday values
var ValidWeekdays = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}

// RecurrenceRule represents a rule for recurring events
type RecurrenceRule struct {
	base.Model    `bun:"schema:schedule,table:recurrence_rules"`
	Frequency     string     `bun:"frequency,notnull" json:"frequency"`
	IntervalCount int        `bun:"interval_count,notnull,default:1" json:"interval_count"`
	Weekdays      []string   `bun:"weekdays,array" json:"weekdays,omitempty"`
	MonthDays     []int      `bun:"month_days,array" json:"month_days,omitempty"`
	EndDate       *time.Time `bun:"end_date" json:"end_date,omitempty"`
	Count         *int       `bun:"count" json:"count,omitempty"`
}

func (r *RecurrenceRule) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableScheduleRecurrenceRules)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableScheduleRecurrenceRules)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableScheduleRecurrenceRules)
	}
	return nil
}

// TableName returns the database table name
func (r *RecurrenceRule) TableName() string {
	return tableScheduleRecurrenceRules
}

// Validate ensures recurrence rule data is valid
func (r *RecurrenceRule) Validate() error {
	if err := r.validateFrequency(); err != nil {
		return err
	}

	if r.IntervalCount < 1 {
		return errors.New("interval count must be at least 1")
	}

	if err := r.validateWeekdays(); err != nil {
		return err
	}

	if err := r.validateMonthDays(); err != nil {
		return err
	}

	return r.validateCountAndEndDate()
}

// validateFrequency validates and normalizes the frequency value
func (r *RecurrenceRule) validateFrequency() error {
	frequency := strings.ToLower(r.Frequency)
	if !isValidFrequency(frequency) {
		return errors.New("invalid frequency value")
	}
	r.Frequency = frequency
	return nil
}

// isValidFrequency checks if frequency is valid
func isValidFrequency(frequency string) bool {
	return frequency == FrequencyDaily ||
		frequency == FrequencyWeekly ||
		frequency == FrequencyMonthly ||
		frequency == FrequencyYearly
}

// validateWeekdays validates and normalizes weekdays
func (r *RecurrenceRule) validateWeekdays() error {
	if len(r.Weekdays) == 0 {
		return nil
	}

	validatedWeekdays := make([]string, 0, len(r.Weekdays))
	for _, day := range r.Weekdays {
		upperDay := strings.ToUpper(day)
		if !isValidWeekday(upperDay) {
			return errors.New("invalid weekday: " + day)
		}
		validatedWeekdays = append(validatedWeekdays, upperDay)
	}
	r.Weekdays = validatedWeekdays
	return nil
}

// isValidWeekday checks if a weekday is valid
func isValidWeekday(day string) bool {
	for _, validDay := range ValidWeekdays {
		if day == validDay {
			return true
		}
	}
	return false
}

// validateMonthDays validates month days are in valid range
func (r *RecurrenceRule) validateMonthDays() error {
	for _, day := range r.MonthDays {
		if day < 1 || day > 31 {
			return errors.New("month days must be between 1 and 31")
		}
	}
	return nil
}

// validateCountAndEndDate validates count and end date constraints
func (r *RecurrenceRule) validateCountAndEndDate() error {
	if r.Count != nil && *r.Count < 1 {
		return errors.New("count must be positive")
	}

	if r.EndDate != nil && r.Count != nil {
		return errors.New("cannot set both end date and count")
	}

	return nil
}

// IsFinite returns true if the recurrence has a defined end (either by count or end date)
func (r *RecurrenceRule) IsFinite() bool {
	return r.EndDate != nil || r.Count != nil
}

// IsWeekdayBased returns true if the recurrence is by weekdays
func (r *RecurrenceRule) IsWeekdayBased() bool {
	return r.Frequency == FrequencyWeekly && len(r.Weekdays) > 0
}

// IsMonthDayBased returns true if the recurrence is by days of month
func (r *RecurrenceRule) IsMonthDayBased() bool {
	return r.Frequency == FrequencyMonthly && len(r.MonthDays) > 0
}

// Clone creates a copy of the recurrence rule
func (r *RecurrenceRule) Clone() *RecurrenceRule {
	clone := &RecurrenceRule{
		Model: base.Model{
			ID:        r.ID,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		Frequency:     r.Frequency,
		IntervalCount: r.IntervalCount,
	}

	// Copy weekdays
	if len(r.Weekdays) > 0 {
		clone.Weekdays = make([]string, len(r.Weekdays))
		copy(clone.Weekdays, r.Weekdays)
	}

	// Copy month days
	if len(r.MonthDays) > 0 {
		clone.MonthDays = make([]int, len(r.MonthDays))
		copy(clone.MonthDays, r.MonthDays)
	}

	// Copy end date if present
	if r.EndDate != nil {
		endDate := *r.EndDate
		clone.EndDate = &endDate
	}

	// Copy count if present
	if r.Count != nil {
		count := *r.Count
		clone.Count = &count
	}

	return clone
}

// GetID implements the Entity interface
func (r *RecurrenceRule) GetID() interface{} {
	return r.ID
}

// GetCreatedAt implements the Entity interface
func (r *RecurrenceRule) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (r *RecurrenceRule) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}
