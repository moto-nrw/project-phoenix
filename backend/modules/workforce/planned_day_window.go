package workforce

import "time"

// PlannedShiftSpan is one planned shift of a staff member as instants.
type PlannedShiftSpan struct {
	Start     time.Time
	End       time.Time
	Cancelled bool
}

// PlannedDayWindow returns the earliest start and the latest end of the
// shifts that take place. A cancelled shift does not take place (#1841), so
// it never widens the window. ok is false when no shift takes place: both
// time-clock gates treat such a day as unplanned ("kein Plan, keine Sperre").
func PlannedDayWindow(shifts []PlannedShiftSpan) (start, end time.Time, ok bool) {
	for _, shift := range shifts {
		if shift.Cancelled {
			continue
		}
		if !ok || shift.Start.Before(start) {
			start = shift.Start
		}
		if !ok || shift.End.After(end) {
			end = shift.End
		}
		ok = true
	}
	return start, end, ok
}

// CheckInOpensAt is the first instant the planned-start lock (#3825) accepts
// a check-in: the planned start minus the tolerance. Arriving late is never
// locked, so the window has no closing instant.
func CheckInOpensAt(plannedStart time.Time, toleranceMinutes int) time.Time {
	return plannedStart.Add(-time.Duration(toleranceMinutes) * time.Minute)
}
