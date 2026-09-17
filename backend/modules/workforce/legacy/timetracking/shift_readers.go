package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// WorkSessionShifts supplies planned shift windows for session closure.
type WorkSessionShifts interface {
	FindByStaffIDsAndDate(context.Context, []int64, timezone.Date) ([]*TimeTrackingShift, error)
}

// StaffOverviewShifts supplies the batch reads used by the staff overview.
type StaffOverviewShifts interface {
	FindByStaffIDsAndDateRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64][]*TimeTrackingShift, error)
	FindByDateRange(context.Context, timezone.Date, timezone.Date) ([]*TimeTrackingShift, error)
}
