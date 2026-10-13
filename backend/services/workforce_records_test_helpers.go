package services

import (
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
)

// The constructors below hand suites that may not import the Workforce
// adapters themselves the retained record contracts over the Workforce
// facade (#2690), built the way newFactory builds them.

// NewStaffWorkScheduleRepository serves the retained staff work schedules.
func NewStaffWorkScheduleRepository(workTime workforceModule.Capability) configModels.StaffWorkScheduleRepository {
	return workforceRepositoryAdapter.NewStaffWorkScheduleRepository(workTime)
}

// NewWorkTimeModelRepository serves the retained work-time templates.
func NewWorkTimeModelRepository(workTime workforceModule.Capability) configModels.WorkTimeModelRepository {
	return workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime)
}

// NewStaffMasterDataRepository serves the retained staff master data.
func NewStaffMasterDataRepository(workTime workforceModule.Capability) userModels.StaffMasterDataRepository {
	return workforceLegacy.NewStaffMasterDataRepository(workTime)
}

// NewStaffQualificationRepository serves the retained staff qualifications.
func NewStaffQualificationRepository(workTime workforceModule.Capability) userModels.StaffQualificationRepository {
	return workforceLegacy.NewStaffQualificationRepository(workTime)
}
