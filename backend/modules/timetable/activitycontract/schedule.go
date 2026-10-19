package activitycontract

import (
	"errors"
	"time"
)

// Valid weekday values following ISO 8601 (Monday = 1, Sunday = 7)
const (
	WeekdayMonday    = 1
	WeekdayTuesday   = 2
	WeekdayWednesday = 3
	WeekdayThursday  = 4
	WeekdayFriday    = 5
	WeekdaySaturday  = 6
	WeekdaySunday    = 7
)

// Schedule represents a scheduled time for an activity group
type Schedule struct {
	ID               int64     `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	TenantID         int64     `json:"tenant_id"`
	Weekday          int       `json:"weekday"`
	TimeframeID      *int64    `json:"timeframe_id,omitempty"`
	ActivityGroupID  int64     `json:"activity_group_id"`
	WeekPattern      int       `json:"week_pattern"`
	CalendarPeriodID *int64    `json:"calendar_period_id,omitempty"`
	// ValidUntil is the exclusive end of this schedule's recurrence: the
	// schedule no longer produces instances ON or AFTER this date (same
	// convention as enrollment valid_until). NULL = open-ended. Set by the
	// template split ("Dieser und alle folgenden") to retire the old
	// template's recurrence from the effective date onward.
	ValidUntil *Date `json:"valid_until,omitempty"`
	// ValidFrom is the inclusive start of this schedule's recurrence: the
	// schedule produces no instances BEFORE this date. NULL = open start.
	// Set on successor schedules by the template split ("Dieser und alle
	// folgenden") so materialization never emits successor instances before
	// the effective date (which would duplicate the old template's rows).
	ValidFrom *Date `json:"valid_from,omitempty"`

	// Relations - these would be populated when using the ORM's relations
	// ActivityGroup *Group `json:"activity_group,omitempty"`
	// Timeframe *schedule.Timeframe `json:"timeframe,omitempty"`
}

// IsValidWeekday checks if the weekday is valid (ISO 8601: 1-7)
func IsValidWeekday(weekday int) bool {
	return weekday >= WeekdayMonday && weekday <= WeekdaySunday
}

// Validate ensures schedule data is valid
func (s *Schedule) Validate() error {
	if !IsValidWeekday(s.Weekday) {
		return errors.New("invalid weekday value")
	}

	if s.ActivityGroupID <= 0 {
		return errors.New("activity group ID is required")
	}

	return nil
}

// HasTimeframe checks if the schedule has a timeframe
func (s *Schedule) HasTimeframe() bool {
	return s.TimeframeID != nil && *s.TimeframeID > 0
}

func (s *Schedule) ValidityDateStrings() (*string, *string) {
	return scheduleDateString(s.ValidUntil), scheduleDateString(s.ValidFrom)
}

func (s *Schedule) SetValidityDateStrings(validUntil, validFrom *string) {
	s.ValidUntil, s.ValidFrom = scheduleDate(validUntil), scheduleDate(validFrom)
}

func scheduleDateString(value *Date) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func scheduleDate(value *string) *Date {
	if value == nil {
		return nil
	}
	result := Date(*value)
	return &result
}

func (v *Schedule) GetTenantID() int64   { return v.TenantID }
func (v *Schedule) SetTenantID(id int64) { v.TenantID = id }
