package testutil

import "github.com/moto-nrw/project-phoenix/services"

// OperatorSchoolSettingsOptions varies the optional root dependencies of the
// operator's school settings a module from SetupOperatorSettingsModule
// composes with OperatorSchoolSettingsWith: the write hook, the broadcast,
// and whether the deployment has the Care Plan booking authority.
type OperatorSchoolSettingsOptions = services.OperatorSchoolSettingsTestOptions
