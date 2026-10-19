//go:build testhooks

package compose

import (
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func studentPhotoPort(runtime StudentPhotoRuntime) ports.StudentPhotoRuntime {
	return application.StudentPhotoPort(runtime)
}
func studentPhotoRuntimeUnboundForTest() error { return application.ErrStudentPhotoRuntimeUnbound }

// ApplyDefaultGuardianRole gives a relationship the default preset its
// relationship type, primary, emergency and pickup facts select, with the
// permissions that preset grants.
func ApplyDefaultGuardianRole(presets peoplePorts.GuardianRolePresets, link *peopleValues.GuardianRelationshipSnapshot) {
	application.ApplyDefaultGuardianRole(presets, link)
}
