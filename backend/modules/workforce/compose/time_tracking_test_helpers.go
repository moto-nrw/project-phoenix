package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
)

// TimeTrackingTestRows are the time-tracking row contracts over a Workforce
// capability, for the suites that seed rows and build a single service
// around fakes; production builds them inside NewTimeTracking.
type TimeTrackingTestRows struct {
	Sessions         timerecords.WorkSessionRepository
	Breaks           timerecords.WorkSessionBreakRepository
	Adjustments      timerecords.StaffBalanceAdjustmentRepository
	VacationQuotas   timerecords.StaffVacationQuotaRepository
	VacationOpenings timerecords.StaffVacationOpeningRepository
}

// NewTimeTrackingTestRows binds the row contracts to capability.
func NewTimeTrackingTestRows(capability workforce.Capability) TimeTrackingTestRows {
	return TimeTrackingTestRows{
		Sessions:         newWorkSessionRows(capability),
		Breaks:           newWorkSessionBreakRows(capability),
		Adjustments:      newBalanceAdjustmentRows(capability),
		VacationQuotas:   newVacationQuotaRows(capability),
		VacationOpenings: newVacationOpeningRows(capability),
	}
}
