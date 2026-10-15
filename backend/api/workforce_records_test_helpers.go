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
// facade (#2690), built the way the service graph builds them. Workforce's
// own suites call its composition; the People Directory and Data Import
// suites build it through the root (ADR 0053 point 7.2, #4005).

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
