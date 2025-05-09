package schedule

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Frequency constants for recurrence rules
const (
	FrequencyDaily   = "daily"
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
	FrequencyYearly  = "yearly"
)

// Valid weekday values
var ValidWeekdays = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}

// RecurrenceRule represents a rule for recurring events
type RecurrenceRule struct {
	base.Model
	Frequency     string     `bun:"frequency,notnull" json:"frequency"`
	IntervalCount int        `bun:"interval_count,notnull,default:1" json:"interval_count"`
	Weekdays      []string   `bun:"weekdays,array" json:"weekdays,omitempty"`
	MonthDays     []int      `bun:"month_days,array" json:"month_days,omitempty"`
	EndDate       *time.Time `bun:"end_date" json:"end_date,omitempty"`
	Count         *int       `bun:"count" json:"count,omitempty"`
}

// TableName returns the database table name
func (r *RecurrenceRule) TableName() string {
	return "schedule.recurrence_rules"
}

// Validate ensures recurrence rule data is valid
func (r *RecurrenceRule) Validate() error {
	// Validate frequency
	frequency := strings.ToLower(r.Frequency)
	if frequency != FrequencyDaily &&
		frequency != FrequencyWeekly &&
		frequency != FrequencyMonthly &&
		frequency != FrequencyYearly {
		return errors.New("invalid frequency value")
	}
	r.Frequency = frequency

	// Interval count must be positive
	if r.IntervalCount < 1 {
		return errors.New("interval count must be at least 1")
	}

	// Validate weekdays
	if len(r.Weekdays) > 0 {
		validatedWeekdays := make([]string, 0, len(r.Weekdays))
		for _, day := range r.Weekdays {
			upperDay := strings.ToUpper(day)
			isValid := false
			for _, validDay := range ValidWeekdays {
				if upperDay == validDay {
					isValid = true
					break
				}
			}
			if !isValid {
				return errors.New("invalid weekday: " + day)
			}
			validatedWeekdays = append(validatedWeekdays, upperDay)
		}
		r.Weekdays = validatedWeekdays
	}

	// Validate month days
	if len(r.MonthDays) > 0 {
		for _, day := range r.MonthDays {
			if day < 1 || day > 31 {
				return errors.New("month days must be between 1 and 31")
			}
		}
	}

	// Validate count
	if r.Count != nil && *r.Count < 1 {
		return errors.New("count must be positive")
	}

	// Either EndDate or Count should be set, not both
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
