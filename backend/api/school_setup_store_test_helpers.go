//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
	schoolSetupCompose "github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

func NewSchoolSetupTestService(settings schoolsetup.Settings) (schoolsetup.Service, error) {
	return schoolSetupCompose.New(schoolSetupCompose.Dependencies{Settings: settings, Store: settingsCompose.NewSchoolSetupRepository[schoolsetup.State](schoolSetupCompose.AmbientRuntime{})})
}

func NewStaffOnboardingTestService() (schoolsetup.StaffOnboardingService, error) {
	return schoolSetupCompose.NewStaffOnboarding(settingsCompose.NewStaffOnboardingRepository[schoolsetup.StaffState](schoolSetupCompose.AmbientRuntime{}))
}
