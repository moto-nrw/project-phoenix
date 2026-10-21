package timetable

import "errors"

// Schedule-service failures preserve the existing HTTP error messages.
var (
	// ErrCategoryNotAssignable rejects assigning missing or archived categories.
	ErrCategoryNotAssignable = errors.New("activity category is not available for new assignments")
	// ErrInvalidRecurrenceRange rejects a slot search whose start lies after its end.
	ErrInvalidRecurrenceRange = errors.New("invalid date range")
	ErrInvalidDuration        = errors.New("invalid duration")
)
