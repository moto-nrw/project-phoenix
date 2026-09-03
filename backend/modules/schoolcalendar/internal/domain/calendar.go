package domain

import (
	"errors"
	"time"
)

var (
	ErrCalendarPeriodNotFound     = errors.New("calendar period not found")
	ErrClosingDayNotFound         = errors.New("closing day not found")
	ErrDateframeNotFound          = errors.New("dateframe not found")
	ErrCalendarPeriodNameConflict = errors.New("calendar period name already exists")
)

// Dateframe ranges are instants: the table stores TIMESTAMPTZ columns.
type Dateframe struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartDate   time.Time
	EndDate     time.Time
	Name        string
	Description string
}

type DateframeFields struct {
	StartDate   time.Time
	EndDate     time.Time
	Name        string
	Description string
}

type DateframeSort struct {
	Field      string
	Descending bool
}

type DateframeFilter struct {
	IDs             []int64
	Name            string
	NameFold        string
	NamePattern     string
	Contains        *time.Time
	OverlappingFrom *time.Time
	OverlappingTo   *time.Time
	Sort            []DateframeSort
	Limit           int
	Offset          int
}

// CalendarPeriod dates are calendar days in YYYY-MM-DD; WeekCycleAnchor is
// empty when unset.
type CalendarPeriod struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Name            string
	PeriodType      string
	StartDate       string
	EndDate         string
	WeekCycleLength int
	WeekCycleAnchor string
	IsActive        bool
}

type CalendarPeriodFields struct {
	Name            string
	PeriodType      string
	StartDate       string
	EndDate         string
	WeekCycleLength int
	WeekCycleAnchor string
	IsActive        bool
}

type CalendarPeriodFilter struct {
	IDs             []int64
	Name            string
	PeriodType      string
	ActiveOnly      bool
	OverlappingFrom string
	OverlappingTo   string
	ExcludeID       int64
}

type ClosingDay struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StartDate string
	EndDate   string
	Reason    string
}

type ClosingDayFields struct {
	StartDate string
	EndDate   string
	Reason    string
}

type ClosingDayFilter struct {
	IDs             []int64
	OverlappingFrom string
	OverlappingTo   string
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
