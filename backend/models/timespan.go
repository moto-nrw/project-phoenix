package models

import (
	"net/http"
	"time"
)

// Timespan represents a period of time with a start and optional end
type Timespan struct {
	ID        int64      `json:"id" bun:"id,pk,autoincrement"`
	StartTime time.Time  `json:"starttime" bun:"starttime,notnull"`
	EndTime   *time.Time `json:"endtime,omitempty" bun:"endtime"`
	CreatedAt time.Time  `json:"created_at" bun:"created_at,notnull"`
}

// Bind preprocesses a Timespan request
func (t *Timespan) Bind(req *http.Request) error {
	// Validation logic could be added here
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	return nil
}

// IsActive checks if the timespan is currently active
func (t *Timespan) IsActive() bool {
	// Timespan is active if it has no end time or the end time is in the future
	return t.EndTime == nil || t.EndTime.After(time.Now())
}

// Duration returns the duration of the timespan
// If the timespan has no end time, it returns the duration from start time until now
func (t *Timespan) Duration() time.Duration {
	if t.EndTime == nil {
		return time.Since(t.StartTime)
	}
	return t.EndTime.Sub(t.StartTime)
}
