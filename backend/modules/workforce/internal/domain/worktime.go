// Package domain holds the Workforce work-time rules: the template shape a
// school may define, the per-staff schedule versions derived from it, and the
// validation both share. Dates are calendar days in DateLayout and clock
// values are wall-clock strings in ClockLayout; neither carries an instant a
// driver could shift across a timezone boundary.
package domain

import (
	"errors"
	"fmt"
	"time"
)

const (
	// MaxRotationWeeks caps a template at four rotation weeks (A/B/C/D).
	MaxRotationWeeks = 4
	// MaxDailyMinutes caps a single day's target working time at 12 hours.
	MaxDailyMinutes = 720
	// DateLayout is the calendar-date representation of every date field.
	DateLayout = "2006-01-02"
	// ClockLayout is the wall-clock representation of every time-of-day field.
	ClockLayout = "15:04:05"
)

var (
	ErrWorkTimeModelNotFound = errors.New("work time model not found")
	ErrWorkTimeModelAssigned = errors.New("work time model is assigned to staff")
	ErrInvalidWorkTime       = errors.New("invalid work time input")
)

// InvalidWorkTimeError carries the caller-facing validation reason. It unwraps
// to ErrInvalidWorkTime so the composition can classify it without string
// matching.
type InvalidWorkTimeError struct{ Reason string }

func (e *InvalidWorkTimeError) Error() string { return e.Reason }
func (e *InvalidWorkTimeError) Unwrap() error { return ErrInvalidWorkTime }

func invalid(format string, args ...any) error {
	return &InvalidWorkTimeError{Reason: fmt.Sprintf(format, args...)}
}

// OperationStats is the per-call runtime evidence the store reports.
type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}

// WorkTimeModelEntry is the target working time of one (week_index,
// day_of_week) slot inside a template.
type WorkTimeModelEntry struct {
	ID            int64
	ModelID       int64
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
	// StartTime is a wall clock in ClockLayout, empty when unset.
	StartTime string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WorkTimeModel is a tenant-scoped, named working-time template.
type WorkTimeModel struct {
	ID                 int64
	TenantID           int64
	Name               string
	RotationLength     int
	RotationAnchorDate string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Entries            []WorkTimeModelEntry
}

// WorkTimeModelEntryFields is the writable part of one template slot.
type WorkTimeModelEntryFields struct {
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
	StartTime     string
}

// WorkTimeModelFields is the writable part of a template.
type WorkTimeModelFields struct {
	Name               string
	RotationLength     int
	RotationAnchorDate string
	Entries            []WorkTimeModelEntryFields
}

// StaffWorkSchedule is one version of a staff member's target working time
// for a single weekday. A schedule change closes the running versions at
// today and inserts new ones, so past weeks keep the rotation parity they
// were computed with.
type StaffWorkSchedule struct {
	ID             int64
	TenantID       int64
	StaffID        int64
	WeekIndex      int
	RotationLength int
	DayOfWeek      int
	TargetMinutes  int
	StartTime      string
	// RotationAnchorDate is the anchor this version was written with,
	// empty when the row predates per-version anchors.
	RotationAnchorDate string
	ValidFrom          string
	// ValidUntil is exclusive, empty while the version is current.
	ValidUntil string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// StaffWorkScheduleFields is the writable part of one schedule version row.
type StaffWorkScheduleFields struct {
	WeekIndex      int
	RotationLength int
	DayOfWeek      int
	TargetMinutes  int
	StartTime      string
}

// ValidateWorkTimeModelFields enforces the template rules: a name, a rotation
// between one and MaxRotationWeeks, an anchor date, per-entry bounds, and one
// entry per (week_index, day_of_week) slot inside the rotation.
func ValidateWorkTimeModelFields(fields WorkTimeModelFields) error {
	if fields.Name == "" {
		return invalid("name is required")
	}
	if fields.RotationLength < 1 || fields.RotationLength > MaxRotationWeeks {
		return invalid("rotation_length must be between 1 and %d", MaxRotationWeeks)
	}
	if fields.RotationAnchorDate == "" {
		return invalid("rotation_anchor_date is required")
	}
	if err := ValidateDate(fields.RotationAnchorDate, "rotation_anchor_date"); err != nil {
		return err
	}
	seen := make(map[[2]int]struct{}, len(fields.Entries))
	for _, entry := range fields.Entries {
		if err := validateEntryBounds(entry); err != nil {
			return err
		}
		if entry.WeekIndex >= fields.RotationLength {
			return invalid("entry week_index outside rotation_length")
		}
		slot := [2]int{entry.WeekIndex, entry.DayOfWeek}
		if _, exists := seen[slot]; exists {
			return invalid("duplicate entry for week_index and day_of_week")
		}
		seen[slot] = struct{}{}
	}
	return nil
}

func validateEntryBounds(entry WorkTimeModelEntryFields) error {
	if entry.WeekIndex < 0 || entry.WeekIndex >= MaxRotationWeeks {
		return invalid("week_index must be between 0 and %d", MaxRotationWeeks-1)
	}
	if entry.DayOfWeek < 0 || entry.DayOfWeek > 6 {
		return invalid("day_of_week must be between 0 (Monday) and 6 (Sunday)")
	}
	if entry.TargetMinutes < 0 || entry.TargetMinutes > MaxDailyMinutes {
		return invalid("target_minutes must be between 0 and %d (12h)", MaxDailyMinutes)
	}
	return ValidateClock(entry.StartTime, "start_time")
}

// ValidateStaffScheduleFields enforces the per-staff schedule rules. It is the
// same bounds check the template entries pass, plus the rotation consistency
// a stored version needs.
func ValidateStaffScheduleFields(fields StaffWorkScheduleFields) error {
	if fields.DayOfWeek < 0 || fields.DayOfWeek > 6 {
		return invalid("day_of_week must be between 0 (Monday) and 6 (Sunday)")
	}
	if fields.TargetMinutes < 0 || fields.TargetMinutes > MaxDailyMinutes {
		return invalid("target_minutes must be between 0 and %d (12h)", MaxDailyMinutes)
	}
	if fields.RotationLength < 1 || fields.RotationLength > MaxRotationWeeks {
		return invalid("rotation_length must be between 1 and %d", MaxRotationWeeks)
	}
	if fields.WeekIndex < 0 || fields.WeekIndex >= fields.RotationLength {
		return invalid("week_index must be between 0 and rotation_length - 1")
	}
	return ValidateClock(fields.StartTime, "start_time")
}

// ValidateDate rejects anything that is not a strict calendar day.
func ValidateDate(value, field string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(DateLayout, value)
	if err != nil || parsed.Format(DateLayout) != value {
		return invalid("%s must be a %s date", field, DateLayout)
	}
	return nil
}

// ValidateClock rejects anything that is not a strict wall clock.
func ValidateClock(value, field string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(ClockLayout, value)
	if err != nil || parsed.Format(ClockLayout) != value {
		return invalid("%s must be a %s wall clock", field, ClockLayout)
	}
	return nil
}
