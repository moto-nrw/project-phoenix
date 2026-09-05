package domain

import (
	"errors"
	"time"
)

var ErrScheduleNotFound = errors.New("schedule not found")

type Schedule struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Weekday          int
	TimeframeID      *int64
	ActivityGroupID  int64
	WeekPattern      int
	CalendarPeriodID *int64
	ValidUntil       *string
	ValidFrom        *string
}

type ScheduleFields struct {
	Weekday          int
	TimeframeID      *int64
	ActivityGroupID  int64
	WeekPattern      int
	CalendarPeriodID *int64
	ValidUntil       *string
	ValidFrom        *string
}

type ScheduleFilter struct {
	GroupIDs []int64
	Weekday  *int
}

type TemplateStartTime struct {
	ActivityGroupID int64
	Weekday         int
	StartTime       time.Time
}
