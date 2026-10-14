package compose

import (
	workforcePostgres "github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
)

// The retained Workforce records the typed root's test graphs read through:
// the staff master data, qualifications and financial data over the Workforce
// facade, and the absence type allowance stores. The root may not import the
// adapters, so the composition names them (ADR 0053 point 2).

type (
	StaffAbsenceTypeAllowanceRepository       = workforcePostgres.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChangeRepository = workforcePostgres.StaffAbsenceTypeAllowanceChangeRepository
)

var (
	NewStaffAbsenceTypeAllowanceRepository       = workforcePostgres.NewStaffAbsenceTypeAllowanceRepository
	NewStaffAbsenceTypeAllowanceChangeRepository = workforcePostgres.NewStaffAbsenceTypeAllowanceChangeRepository
	NewStaffMasterDataRepository                 = workforceLegacy.NewStaffMasterDataRepository
	NewStaffQualificationRepository              = workforceLegacy.NewStaffQualificationRepository
	NewStaffFinancialDataRepository              = workforceLegacy.NewStaffFinancialDataRepository
)
