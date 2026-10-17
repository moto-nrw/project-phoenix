package compose

import (
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// ApplyDefaultGuardianRole gives a relationship the default preset its
// relationship type, primary, emergency and pickup facts select, with the
// permissions that preset grants.
func ApplyDefaultGuardianRole(presets peoplePorts.GuardianRolePresets, link *users.StudentGuardian) {
	application.ApplyDefaultGuardianRole(presets, link)
}
