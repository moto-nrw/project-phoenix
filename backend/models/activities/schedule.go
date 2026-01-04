package activities

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
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

// tableActivitiesSchedules is the schema-qualified table name for schedules
const tableActivitiesSchedules = "activities.schedules"

// Schedule represents a scheduled time for an activity group
type Schedule struct {
	base.Model      `bun:"schema:activities,table:schedules"`
	Weekday         int    `bun:"weekday,notnull" json:"weekday"`
	TimeframeID     *int64 `bun:"timeframe_id" json:"timeframe_id,omitempty"`
	ActivityGroupID int64  `bun:"activity_group_id,notnull" json:"activity_group_id"`

	// Relations - these would be populated when using the ORM's relations
	// ActivityGroup *Group `bun:"rel:belongs-to,join:activity_group_id=id" json:"activity_group,omitempty"`
	// Timeframe *schedule.Timeframe `bun:"rel:belongs-to,join:timeframe_id=id" json:"timeframe,omitempty"`
}

func (s *Schedule) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableActivitiesSchedules)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActivitiesSchedules)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActivitiesSchedules)
	}
	return nil
}

// GetID returns the entity's ID
func (s *Schedule) GetID() interface{} {
	return s.ID
}

// GetCreatedAt returns the creation timestamp
func (s *Schedule) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (s *Schedule) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// TableName returns the database table name
func (s *Schedule) TableName() string {
	return tableActivitiesSchedules
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
