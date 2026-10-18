package application

import (
	"encoding/json"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// ApplyGuardianRolePreset sets a relationship's role to the preset and its
// parent-portal permissions to exactly the ones the preset grants.
func ApplyGuardianRolePreset(presets ports.GuardianRolePresets, link *peopleValues.GuardianRelationshipSnapshot, role string) {
	normalized, granted := presets.Preset(role)
	permissions := make(map[string]bool, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	link.GuardianRole = normalized
	link.Permissions, _ = json.Marshal(permissions)
}

// ApplyDefaultGuardianRole gives a relationship the default preset its
// relationship type, primary, emergency and pickup facts select, with the
// permissions that preset grants.
func ApplyDefaultGuardianRole(presets ports.GuardianRolePresets, link *peopleValues.GuardianRelationshipSnapshot) {
	relationshipType, primary, emergency, pickup := link.RelationshipType, link.IsPrimary, link.IsEmergencyContact, link.CanPickup
	ApplyGuardianRolePreset(presets, link, presets.DefaultRole(relationshipType, primary, emergency, pickup))
}
