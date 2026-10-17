//go:build testhooks

package compose

import (
	workforcePostgres "github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/uptrace/bun"
)

type (
	StaffAbsenceTypeAllowanceRepository       = workforcePostgres.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChangeRepository = workforcePostgres.StaffAbsenceTypeAllowanceChangeRepository
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
	shapes := legacyRowErrors()
	return TimeTrackingTestRows{
		Sessions:         timetracking.NewWorkSessionRows(capability, shapes),
		Breaks:           timetracking.NewWorkSessionBreakRows(capability, shapes),
		Adjustments:      timetracking.NewBalanceAdjustmentRows(capability, shapes),
		VacationQuotas:   timetracking.NewVacationQuotaRows(capability, shapes),
		VacationOpenings: timetracking.NewVacationOpeningRows(capability, shapes),
	}
}

func NewStaffAbsenceTypeAllowanceRepository(db *bun.DB) *StaffAbsenceTypeAllowanceRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceRepository(workforcePostgres.Database(databaseRuntime(db)))
}

func NewStaffAbsenceTypeAllowanceChangeRepository(db *bun.DB) *StaffAbsenceTypeAllowanceChangeRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceChangeRepository(workforcePostgres.Database(databaseRuntime(db)))
}
