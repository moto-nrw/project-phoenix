package ports

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
