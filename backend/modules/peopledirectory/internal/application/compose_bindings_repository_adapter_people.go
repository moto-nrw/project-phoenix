package application

import (
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// defaultGuardianRole applies the default preset, or nothing without a
// policy.
func DefaultGuardianRole(presets peoplePorts.GuardianRolePresets) func(*peopleValues.GuardianRelationshipSnapshot) {
	if presets == nil {
		return nil
	}
	return func(link *peopleValues.GuardianRelationshipSnapshot) {
		ApplyDefaultGuardianRole(presets, link)
	}
}
