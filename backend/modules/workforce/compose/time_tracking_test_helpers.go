//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforcePostgres "github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/workforcepostgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/uptrace/bun"
)

type (
	StaffAbsenceTypeAllowanceRepository       = workforcePostgres.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChangeRepository = workforcePostgres.StaffAbsenceTypeAllowanceChangeRepository
)

// TimeTrackingTestRows are the time-tracking row contracts over a Workforce
// capability, for the suites that seed rows and build a single service
// around fakes; production builds them inside NewWorkSessionSide.
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
		Sessions:         timetracking.NewWorkSessionRows(capability),
		Breaks:           timetracking.NewWorkSessionBreakRows(capability),
		Adjustments:      timetracking.NewBalanceAdjustmentRows(capability),
		VacationQuotas:   timetracking.NewVacationQuotaRows(capability),
		VacationOpenings: timetracking.NewVacationOpeningRows(capability),
	}
}

func NewStaffAbsenceTypeAllowanceRepository(db *bun.DB) *StaffAbsenceTypeAllowanceRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceRepository(workforcePostgres.Database(databaseRuntime(db)))
}

func NewStaffAbsenceTypeAllowanceChangeRepository(db *bun.DB) *StaffAbsenceTypeAllowanceChangeRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceChangeRepository(workforcePostgres.Database(databaseRuntime(db)))
}
