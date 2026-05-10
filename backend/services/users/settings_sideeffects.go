package users

import (
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// RegisterStudentPhotoSettingsSideEffects wires the student-photo feature flag
// to the photo lifecycle service.
func RegisterStudentPhotoSettingsSideEffects(registry *sideeffects.Registry, service StudentPhotoService) {
	registry.Register(configModel.KeyStudentPhotosEnabled, service.HandleFeatureToggle)
}
