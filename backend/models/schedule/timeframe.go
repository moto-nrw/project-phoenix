package schedule

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Timeframe represents a time period with start and end times
type Timeframe struct {
	base.Model  `bun:"schema:schedule,table:timeframes"`
	StartTime   time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime     *time.Time `bun:"end_time" json:"end_time,omitempty"`
	IsActive    bool       `bun:"is_active,notnull,default:false" json:"is_active"`
	Description string     `bun:"description" json:"description,omitempty"`
}

func (t *Timeframe) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr("schedule.timeframes")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr("schedule.timeframes")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr("schedule.timeframes")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr("schedule.timeframes")
	}
	return nil
}

// TableName returns the database table name
func (t *Timeframe) TableName() string {
	return "schedule.timeframes"
}

// Validate ensures timeframe data is valid
func (t *Timeframe) Validate() error {
	if t.StartTime.IsZero() {
		return errors.New("start time is required")
	}

	// If end time is provided, ensure it's after start time
	if t.EndTime != nil && !t.EndTime.IsZero() && !t.EndTime.After(t.StartTime) {
		return errors.New("end time must be after start time")
	}

	return nil
}

// Duration returns the duration of the timeframe
// If no end time is set, returns 0
func (t *Timeframe) Duration() time.Duration {
	if t.EndTime == nil {
		return 0
	}
	return t.EndTime.Sub(t.StartTime)
}

// IsOpen returns true if the timeframe has no end time
func (t *Timeframe) IsOpen() bool {
	return t.EndTime == nil || t.EndTime.IsZero()
}

// Contains checks if the given time is within the timeframe
func (t *Timeframe) Contains(checkTime time.Time) bool {
	if checkTime.Before(t.StartTime) {
		return false
	}

	if t.EndTime == nil {
		return true // Open-ended timeframe
	}

	return checkTime.Before(*t.EndTime) || checkTime.Equal(*t.EndTime)
}

// Overlaps checks if this timeframe overlaps with another timeframe
func (t *Timeframe) Overlaps(other *Timeframe) bool {
	// If this timeframe is open-ended (no end time)
	if t.EndTime == nil {
		// It overlaps if the other starts after or at the same time as this one
		return !other.StartTime.Before(t.StartTime)
	}

	// If other timeframe is open-ended
	if other.EndTime == nil {
		// It overlaps if other starts before this one ends
		return other.StartTime.Before(*t.EndTime)
	}

	// Both have start and end times
	// Overlap if one starts before the other ends
	return (other.StartTime.Before(*t.EndTime) && t.StartTime.Before(*other.EndTime))
}

// GetID implements the Entity interface
func (t *Timeframe) GetID() interface{} {
	return t.ID
}

// GetCreatedAt implements the Entity interface
func (t *Timeframe) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (t *Timeframe) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}
