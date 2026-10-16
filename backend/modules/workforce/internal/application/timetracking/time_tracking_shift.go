package timetracking

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimeTrackingShift contains only planned-presence facts used by time tracking.
type TimeTrackingShift struct {
	StaffID      int64
	Date         calendar.Date
	StartTime    time.Time
	EndTime      time.Time
	BreakMinutes int
	Cancelled    bool
}

func (s *TimeTrackingShift) StartInstant() time.Time { return s.instant(s.StartTime) }
func (s *TimeTrackingShift) EndInstant() time.Time   { return s.instant(s.EndTime) }

func (s *TimeTrackingShift) instant(clock time.Time) time.Time {
	wc := calendar.NormalizeWallClock(clock)
	return time.Date(s.Date.Year(), s.Date.Month(), s.Date.Day(), wc.Hour(), wc.Minute(), wc.Second(), 0, calendar.Berlin)
}
