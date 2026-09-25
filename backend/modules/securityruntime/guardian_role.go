package securityruntime

import (
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
)

// Role presets of a student-guardian relationship.
const (
	GuardianRolePrimaryGuardian = authorize.GuardianRolePrimaryGuardian
	GuardianRoleEmergency       = authorize.GuardianRoleEmergency
	GuardianRolePickupOnly      = authorize.GuardianRolePickupOnly
	GuardianRoleCustom          = authorize.GuardianRoleCustom
)

// StudentGuardianRolePreset returns the stored role of a relationship role
// preset and the parent-portal permissions it grants, in sorted order: the
// values a relationship takes when the preset is applied.
func StudentGuardianRolePreset(role string) (string, []string) {
	normalized := authorize.NormalizeGuardianRole(role)
	granted := make([]string, 0)
	for permission, value := range authorize.StudentGuardianPermissionSet(normalized) {
		if enabled, ok := value.(bool); ok && enabled {
			granted = append(granted, permission)
		}
	}
	sort.Strings(granted)
	return normalized, granted
}

// IsFullGuardianRole reports whether a stored relationship role is one of the
// full guardian presets (primary, legal or co-guardian).
func IsFullGuardianRole(role string) bool {
	return authorize.IsFullGuardianRole(role)
}
