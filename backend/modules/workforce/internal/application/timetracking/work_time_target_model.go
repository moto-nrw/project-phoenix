package timetracking

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// WorkTimeTargetModel exposes the target calculation without a writable model.
type WorkTimeTargetModel struct {
	ID                 int64
	RotationAnchorDate calendar.Date
	DailyTarget        func(anchor, date calendar.Date) (int, bool)
}
