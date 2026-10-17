package application

import (
	"github.com/moto-nrw/project-phoenix/models/users"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// ApplyGuardianRolePreset sets a relationship's role to the preset and its
// parent-portal permissions to exactly the ones the preset grants.
func ApplyGuardianRolePreset(presets ports.GuardianRolePresets, link *users.StudentGuardian, role string) {
	normalized, granted := presets.Preset(role)
	permissions := make(map[string]interface{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	link.SetGuardianAuthorizationData(normalized, permissions)
}

// ApplyDefaultGuardianRole gives a relationship the default preset its
// relationship type, primary, emergency and pickup facts select, with the
// permissions that preset grants.
func ApplyDefaultGuardianRole(presets ports.GuardianRolePresets, link *users.StudentGuardian) {
	relationshipType, _, primary, emergency, pickup, _ := link.GuardianAuthorizationData()
	ApplyGuardianRolePreset(presets, link, presets.DefaultRole(relationshipType, primary, emergency, pickup))
}
