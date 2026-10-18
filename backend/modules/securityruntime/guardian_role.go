package securityruntime

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
)

// Role presets of a student-guardian relationship.
const (
	GuardianRolePrimaryGuardian = authorize.GuardianRolePrimaryGuardian
	GuardianRoleLegalGuardian   = authorize.GuardianRoleLegalGuardian
	GuardianRoleCoGuardian      = authorize.GuardianRoleCoGuardian
	GuardianRoleEmergency       = authorize.GuardianRoleEmergency
	GuardianRolePickupOnly      = authorize.GuardianRolePickupOnly
	GuardianRoleSocialWorker    = authorize.GuardianRoleSocialWorker
	GuardianRoleCustom          = authorize.GuardianRoleCustom
)

// Parent-portal permissions a relationship role preset grants.
const (
	GuardianPermissionPortalAccess     = authorize.GuardianPermissionPortalAccess
	GuardianPermissionSickNoteSubmit   = authorize.GuardianPermissionSickNoteSubmit
	GuardianPermissionNotesWrite       = authorize.GuardianPermissionNotesWrite
	GuardianPermissionEnrollmentSubmit = authorize.GuardianPermissionEnrollmentSubmit
	GuardianPermissionPollResponse     = authorize.GuardianPermissionPollResponse
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

// GuardianPermissionGrantedJSON reports whether the stored permissions
// object grants permission, read by GuardianPermissionGranted. It fails
// closed: an object that does not decode, a missing key and a value that
// does not decode all grant nothing. Numbers keep their exact text.
func GuardianPermissionGrantedJSON(raw json.RawMessage, permission string) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	value, exists := values[permission]
	if !exists {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var stored any
	if decoder.Decode(&stored) != nil {
		return false
	}
	return GuardianPermissionGranted(stored)
}

const storedGuardianPermissionKey = "permission"

// storedGuardianPermission carries one stored permission value through the
// relationship check, so the value is read by the shared rule.
type storedGuardianPermission struct{ value any }

func (p storedGuardianPermission) GuardianAuthorizationData() (string, string, bool, bool, bool, map[string]interface{}) {
	return "", "", false, false, false, map[string]interface{}{storedGuardianPermissionKey: p.value}
}

func (storedGuardianPermission) SetGuardianAuthorizationData(string, map[string]interface{}) {}

// GuardianRoleClass groups the stored relationship roles by the access a
// guardian invitation may build on them.
type GuardianRoleClass int

const (
	// GuardianRoleClassRestricted is every role without parents-portal
	// access that a confirmed invitation may upgrade to a legal guardian.
	GuardianRoleClassRestricted GuardianRoleClass = iota
	// GuardianRoleClassFull is a full guardian preset (primary, legal or
	// co-guardian).
	GuardianRoleClassFull
	// GuardianRoleClassSocialWorker is the school-managed social worker
	// preset, which no invitation may upgrade.
	GuardianRoleClassSocialWorker
)

// ClassifyGuardianRole reports the class of a stored relationship role.
func ClassifyGuardianRole(role string) GuardianRoleClass {
	switch {
	case IsFullGuardianRole(role):
		return GuardianRoleClassFull
	case authorize.NormalizeGuardianRole(role) == GuardianRoleSocialWorker:
		return GuardianRoleClassSocialWorker
	default:
		return GuardianRoleClassRestricted
	}
}
