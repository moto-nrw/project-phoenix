package timetable

import (
	"context"
	"time"
)

type RecurrenceRule struct {
	ID            int64      `json:"id"`
	TenantID      int64      `json:"tenant_id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Frequency     string     `json:"frequency"`
	IntervalCount int        `json:"interval_count"`
	Weekdays      []string   `json:"weekdays,omitempty"`
	MonthDays     []int      `json:"month_days,omitempty"`
	EndDate       *time.Time `json:"end_date,omitempty"`
	Count         *int       `json:"count,omitempty"`
}

type RecurrenceRuleInput struct {
	Frequency     string
	IntervalCount int
	Weekdays      []string
	MonthDays     []int
	EndDate       *time.Time
	Count         *int
}

type RecurrenceRuleFilter struct {
	Frequency      string
	Frequencies    []string
	Weekday        string
	ActiveAt       *time.Time
	SortBy         string
	SortDescending bool
	Limit          int
	Offset         int
}

type RecurrenceRuleQuery interface {
	FindRecurrenceRule(context.Context, int64) (RecurrenceRule, error)
	ListRecurrenceRules(context.Context, RecurrenceRuleFilter) ([]RecurrenceRule, error)
}

type RecurrenceRuleCommand interface {
	CreateRecurrenceRule(context.Context, RecurrenceRuleInput) (RecurrenceRule, error)
	UpdateRecurrenceRule(context.Context, int64, RecurrenceRuleInput) (RecurrenceRule, error)
	DeleteRecurrenceRule(context.Context, int64) error
}

type RecurrenceRuleCapability interface {
	RecurrenceRuleQuery
	RecurrenceRuleCommand
}
