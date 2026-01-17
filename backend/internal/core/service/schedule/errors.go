package schedule

import (
	"errors"
	"fmt"
)

// Common schedule errors
var (
	ErrDateframeNotFound      = errors.New("dateframe not found")
	ErrTimeframeNotFound      = errors.New("timeframe not found")
	ErrRecurrenceRuleNotFound = errors.New("recurrence rule not found")
	ErrInvalidDateRange       = errors.New("invalid date range")
	ErrInvalidTimeRange       = errors.New("invalid time range")
	ErrInvalidDuration        = errors.New("invalid duration")
)

// ScheduleError represents a schedule-related error
type ScheduleError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

// Error returns the error message
func (e *ScheduleError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("schedule error during %s", e.Op)
	}
	return fmt.Sprintf("schedule error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *ScheduleError) Unwrap() error {
	return e.Err
}
