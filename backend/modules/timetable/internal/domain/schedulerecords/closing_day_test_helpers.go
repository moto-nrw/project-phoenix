//go:build testhooks

package schedulerecords

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Validate ensures closing day data is valid
func (c *ClosingDay) Validate() error {
	trimmedReason := strings.TrimSpace(c.Reason)
	if trimmedReason == "" {
		return errors.New("reason is required")
	}
	if utf8.RuneCountInString(trimmedReason) > ClosingDayReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if c.StartDate.IsZero() {
		return errors.New("start_date is required")
	}
	if c.EndDate.IsZero() {
		return errors.New("end_date is required")
	}
	if c.EndDate.Before(c.StartDate) {
		return errors.New("end_date must not be before start_date")
	}
	return nil
}
