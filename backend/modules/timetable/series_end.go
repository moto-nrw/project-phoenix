package timetable

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrInvalidSeriesEnd rejects a last series day before the series start or
// outside its planning period.
var ErrInvalidSeriesEnd = errors.New("invalid series end date")

// ValidateSeriesLastDay checks the inclusive last day of a series (#3594), as
// the planner enters it: not before the series start (firstDay, zero = no
// lower bound) and not after the end of its planning period (periodEnd, nil =
// no period pin).
func ValidateSeriesLastDay(lastDay, firstDay calendar.Date, periodEnd *calendar.Date) error {
	if lastDay.IsZero() {
		return fmt.Errorf("%w: end_date is required", ErrInvalidSeriesEnd)
	}
	if !firstDay.IsZero() && lastDay.Before(firstDay) {
		return fmt.Errorf("%w: end_date must not be before the series start %s", ErrInvalidSeriesEnd, firstDay)
	}
	if periodEnd != nil && !periodEnd.IsZero() && lastDay.After(*periodEnd) {
		return fmt.Errorf("%w: end_date must lie within the planning period (until %s)", ErrInvalidSeriesEnd, *periodEnd)
	}
	return nil
}
