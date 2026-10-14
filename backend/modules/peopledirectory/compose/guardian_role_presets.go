package compose

import "github.com/moto-nrw/project-phoenix/models/users"

// GuardianRolePresetFuncs serves the role-preset port from two functions, so
// a composition binds the Security Runtime policy without an adapter type.
type GuardianRolePresetFuncs struct {
	Default func(relationshipType string, isPrimary, isEmergencyContact, canPickup bool) string
	Presets func(role string) (string, []string)
}

// DefaultRole is the preset a new relationship takes when the caller chose
// none.
func (f GuardianRolePresetFuncs) DefaultRole(relationshipType string, isPrimary, isEmergencyContact, canPickup bool) string {
	return f.Default(relationshipType, isPrimary, isEmergencyContact, canPickup)
}

// Preset returns the stored role of a preset and the permissions it grants.
func (f GuardianRolePresetFuncs) Preset(role string) (string, []string) {
	return f.Presets(role)
}

// ApplyGuardianRolePreset sets a relationship's role to the preset and its
// parent-portal permissions to exactly the ones the preset grants.
func ApplyGuardianRolePreset(presets GuardianRolePresets, link *users.StudentGuardian, role string) {
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
func ApplyDefaultGuardianRole(presets GuardianRolePresets, link *users.StudentGuardian) {
	relationshipType, _, primary, emergency, pickup, _ := link.GuardianAuthorizationData()
	ApplyGuardianRolePreset(presets, link, presets.DefaultRole(relationshipType, primary, emergency, pickup))
}
