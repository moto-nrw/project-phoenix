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

// DefaultStudentGuardianRole is the role preset a new relationship takes when
// the caller chose none: the full guardian presets for parents and primary
// guardians, never portal access from the pickup or emergency flags alone.
func DefaultStudentGuardianRole(relationshipType string, isPrimary, isEmergencyContact, canPickup bool) string {
	return authorize.DefaultStudentGuardianRole(relationshipType, isPrimary, isEmergencyContact, canPickup)
}

// GuardianPermissionGranted reports whether a stored parent-portal permission
// value grants the permission: a boolean is taken as is, any other non-nil
// value counts as granted.
func GuardianPermissionGranted(value any) bool {
	return authorize.StudentGuardianHasPermission(storedGuardianPermission{value: value}, storedGuardianPermissionKey)
}

const storedGuardianPermissionKey = "permission"

// storedGuardianPermission carries one stored permission value through the
// relationship check, so the value is read by the shared rule.
type storedGuardianPermission struct{ value any }

func (p storedGuardianPermission) GuardianAuthorizationData() (string, string, bool, bool, bool, map[string]interface{}) {
	return "", "", false, false, false, map[string]interface{}{storedGuardianPermissionKey: p.value}
}

func (storedGuardianPermission) SetGuardianAuthorizationData(string, map[string]interface{}) {}
