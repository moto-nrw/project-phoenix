package compose

import (
	"github.com/uptrace/bun"

	workforcePostgres "github.com/moto-nrw/project-phoenix/database/repositories/workforce"
)

// The absence type allowance stores, built for the root's test compositions
// that read the stored allowances back. Production reads them through the
// Workforce capability.

func NewStaffAbsenceTypeAllowanceRepository(db *bun.DB) *StaffAbsenceTypeAllowanceRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceRepository(workforcePostgres.Database(databaseRuntime(db)))
}

func NewStaffAbsenceTypeAllowanceChangeRepository(db *bun.DB) *StaffAbsenceTypeAllowanceChangeRepository {
	return workforcePostgres.NewStaffAbsenceTypeAllowanceChangeRepository(workforcePostgres.Database(databaseRuntime(db)))
}
