package timetracking

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// WorkScheduleTargets keeps the existence of range rows separate from their
// target on a particular day. A valid schedule may deliberately yield zero.
type WorkScheduleTargets struct {
	HasEntries  bool
	DailyTarget func(staffAnchor *calendar.Date, date calendar.Date) (int, bool)
}
