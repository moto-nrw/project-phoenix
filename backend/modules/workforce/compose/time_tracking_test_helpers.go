package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

// TimeTrackingTestRows are the time-tracking row contracts over a Workforce
// capability, for the suites that seed rows and build a single service
// around fakes; production builds them inside NewTimeTracking.
type TimeTrackingTestRows struct {
	Sessions         timetracking.WorkSessionRepository
	Breaks           timetracking.WorkSessionBreakRepository
	Adjustments      timetracking.StaffBalanceAdjustmentRepository
	VacationQuotas   timetracking.StaffVacationQuotaRepository
	VacationOpenings timetracking.StaffVacationOpeningRepository
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
