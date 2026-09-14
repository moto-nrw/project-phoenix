package active

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// WorkScheduleTargets keeps the existence of range rows separate from their
// target on a particular day. A valid schedule may deliberately yield zero.
type WorkScheduleTargets struct {
	HasEntries  bool
	DailyTarget func(staffAnchor *timezone.Date, date timezone.Date) (int, bool)
}
