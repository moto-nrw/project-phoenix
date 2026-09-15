package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type OverviewStaff struct {
	ID                 int64
	FirstName          string
	LastName           string
	EmploymentType     *string
	PersonnelNumber    *string
	WorkTimeModelID    *int64
	RotationAnchorDate *timezone.Date
}

// OverviewStaffQuery selects the tenant's non-offboarded staff for reports.
type OverviewStaffQuery interface {
	ListOverviewStaff(context.Context) ([]OverviewStaff, error)
}
