package timetracking

import "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"

// WorkScheduleTargets keeps the existence of range rows separate from their
// target on a particular day. A valid schedule may deliberately yield zero.
type WorkScheduleTargets struct {
	HasEntries  bool
	DailyTarget func(staffAnchor *calendar.Date, date calendar.Date) (int, bool)
	// Covers reports whether a schedule version is valid on date, also when
	// it has no row for that weekday (#6889).
	Covers func(date calendar.Date) bool
}
