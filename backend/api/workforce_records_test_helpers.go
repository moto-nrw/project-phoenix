package api

import (
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
)

// The constructors below hand suites that may not import the Workforce
// adapters themselves the retained record contracts over the Workforce
// facade (#2690), built the way NewFactoryWithConfig builds them.

// NewWorkTimeModelRepository serves the retained work-time templates.
func NewWorkTimeModelRepository(workTime workforceModule.Capability) configModels.WorkTimeModelRepository {
	return workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime)
}

// NewStaffMasterDataRepository serves the retained staff master data.
func NewStaffMasterDataRepository(workTime workforceModule.Capability) userModels.StaffMasterDataRepository {
	return workforceCompose.NewStaffMasterDataRepository(workTime)
}

// NewStaffQualificationRepository serves the retained staff qualifications.
func NewStaffQualificationRepository(workTime workforceModule.Capability) userModels.StaffQualificationRepository {
	return workforceCompose.NewStaffQualificationRepository(workTime)
}
