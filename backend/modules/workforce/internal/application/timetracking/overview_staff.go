package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type OverviewStaff struct {
	ID                 int64
	FirstName          string
	LastName           string
	EmploymentType     *string
	PersonnelNumber    *string
	WorkTimeModelID    *int64
	RotationAnchorDate *calendar.Date
}

// OverviewStaffQuery selects the tenant's non-offboarded staff for reports.
type OverviewStaffQuery interface {
	ListOverviewStaff(context.Context) ([]OverviewStaff, error)
}
