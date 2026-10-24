package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WorkSessionShifts supplies planned shift windows for session closure.
type WorkSessionShifts interface {
	FindByStaffIDsAndDate(context.Context, []int64, calendar.Date) ([]*TimeTrackingShift, error)
}

// StaffOverviewShifts supplies the batch reads used by the staff overview.
type StaffOverviewShifts interface {
	FindByStaffIDsAndDateRange(context.Context, []int64, calendar.Date, calendar.Date) (map[int64][]*TimeTrackingShift, error)
	FindByDateRange(context.Context, calendar.Date, calendar.Date) ([]*TimeTrackingShift, error)
}
