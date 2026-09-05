package domain

import (
	"errors"
	"time"
)

var ErrRecurrenceRuleNotFound = errors.New("recurrence rule not found")

type RecurrenceRule struct {
	ID            int64
	TenantID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Frequency     string
	IntervalCount int
	Weekdays      []string
	MonthDays     []int
	EndDate       *time.Time
	Count         *int
}

type RecurrenceRuleFields struct {
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
