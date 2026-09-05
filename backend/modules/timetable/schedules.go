package timetable

import "time"

const (
	WeekdayMonday = iota + 1
	WeekdayTuesday
	WeekdayWednesday
	WeekdayThursday
	WeekdayFriday
	WeekdaySaturday
	WeekdaySunday
)

// Schedule is the owner view of one activities.schedules row. Calendar dates
// stay as YYYY-MM-DD strings at the module boundary.
type Schedule struct {
	ID               int64     `json:"id"`
	TenantID         int64     `json:"tenant_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Weekday          int       `json:"weekday"`
	TimeframeID      *int64    `json:"timeframe_id,omitempty"`
	ActivityGroupID  int64     `json:"activity_group_id"`
	WeekPattern      int       `json:"week_pattern"`
	CalendarPeriodID *int64    `json:"calendar_period_id,omitempty"`
	ValidUntil       *string   `json:"valid_until,omitempty"`
	ValidFrom        *string   `json:"valid_from,omitempty"`
}

type ScheduleInput struct {
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
