package enrollment

import (
	"context"
	"errors"
	"fmt"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// The guards in this file are the owner-neutral faces of the care-offering
// change validation: the School Calendar and the Timetable owner describe
// the proposed period or timeframe in their own words, and this package
// renders it as the row shape its validation still reads.

// TimeframeReplacement is the proposed state of a timeframe an edit would
// leave behind. Clock values are HH:MM:SS.
type TimeframeReplacement struct {
	StartTime   string
	EndTime     *string
	IsActive    bool
	Description string
}

// IsCareOfferingInvalid reports whether err is a care-offering
// compatibility refusal rather than an infrastructure failure.
func IsCareOfferingInvalid(err error) bool {
	return errors.Is(err, ErrCareOfferingInvalid)
}

// IsCareOfferingCalendarPeriodConflict reports whether err is the refusal of
// a period change a linked care offering still needs.
func IsCareOfferingCalendarPeriodConflict(err error) bool {
	return errors.Is(err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
}

// ValidateCalendarPeriodFieldsChange is ValidateCalendarPeriodChange for the
// School Calendar's own period fields; a nil replacement is a removal.
func (s *careOfferingService) ValidateCalendarPeriodFieldsChange(
	ctx context.Context,
	periodID int64,
	replacement *schoolcalendar.CalendarPeriodFields,
) error {
	var period *scheduleModels.CalendarPeriod
	if replacement != nil {
		period = &scheduleModels.CalendarPeriod{
			Name:            replacement.Name,
			PeriodType:      replacement.PeriodType,
			StartDate:       scheduleModels.Date(replacement.StartDate),
			EndDate:         scheduleModels.Date(replacement.EndDate),
			WeekCycleLength: replacement.WeekCycleLength,
			IsActive:        replacement.IsActive,
		}
		period.ID = periodID
		if replacement.WeekCycleAnchor != "" {
			anchor := scheduleModels.Date(replacement.WeekCycleAnchor)
			period.WeekCycleAnchor = &anchor
		}
	}
	return s.ValidateCalendarPeriodChange(ctx, periodID, period)
}

// ValidateTimeframeReplacement is ValidateTimeframeChange for the Timetable
// owner's clock strings; a nil replacement is a deletion.
func (s *careOfferingService) ValidateTimeframeReplacement(
	ctx context.Context,
	timeframeID int64,
	replacement *TimeframeReplacement,
) error {
	var row *scheduleModels.Timeframe
	if replacement != nil {
		start, err := parseTimeframeClock(replacement.StartTime)
		if err != nil {
			return careOfferingInvalidf("timeframe start time %q is not a clock value", replacement.StartTime)
		}
		row = &scheduleModels.Timeframe{StartTime: start, IsActive: replacement.IsActive, Description: replacement.Description}
		row.ID = timeframeID
		if replacement.EndTime != nil {
			end, parseErr := parseTimeframeClock(*replacement.EndTime)
			if parseErr != nil {
				return careOfferingInvalidf("timeframe end time %q is not a clock value", *replacement.EndTime)
			}
			row.EndTime = &end
		}
	}
	if err := s.ValidateTimeframeChange(ctx, timeframeID, row); err != nil {
		return fmt.Errorf("validate timeframe %d change: %w", timeframeID, err)
	}
	return nil
}

// parseTimeframeClock reads the Timetable owner's HH:MM:SS clock string, with
// or without the fractional second a stored row may carry.
func parseTimeframeClock(value string) (time.Time, error) {
	clock, err := time.Parse("15:04:05", value)
	if err == nil {
		return clock, nil
	}
	return time.Parse("15:04:05.999999999", value)
}
