package timetable

import (
	"errors"
	"fmt"
	"time"
)

// Validate checks the storable exception shape, including single-field overrides.
func (e *ActivityException) Validate() error {
	if e.ActivityGroupID <= 0 {
		return errors.New("activity_group_id is required")
	}
	if e.ExceptionDate == "" {
		return errors.New("exception_date is required")
	}
	switch e.ExceptionType {
	case ActivityExceptionCancelled:
		if e.StartTime != nil || e.EndTime != nil || e.RoomID != nil {
			return errors.New("cancelled exception must not set start_time, end_time, or room_id")
		}
	case ActivityExceptionModified:
		if e.StartTime == nil && e.EndTime == nil && e.RoomID == nil {
			return errors.New("modified exception must override at least one of start_time, end_time, or room_id")
		}
		if err := validateExceptionClockOrder(e.StartTime, e.EndTime); err != nil {
			return err
		}
	default:
		return errors.New("invalid exception type")
	}
	if e.Reason != nil && len(*e.Reason) > ActivityExceptionReasonMaxLength {
		return fmt.Errorf("reason cannot exceed %d characters", ActivityExceptionReasonMaxLength)
	}
	return nil
}
func validateExceptionClockOrder(start, end *string) error {
	if start == nil || end == nil {
		return nil
	}
	from, err := time.Parse("15:04:05", *start)
	if err != nil {
		return err
	}
	to, err := time.Parse("15:04:05", *end)
	if err != nil {
		return err
	}
	if !to.After(from) {
		return errors.New("end_time must be after start_time")
	}
	return nil
}
