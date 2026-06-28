package authorize

import (
	"encoding/json"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/users"
)

const (
	GuardianPermissionPortalAccess     = "parent_portal.access"
	GuardianPermissionSickNoteSubmit   = "parent_portal.sick_note.submit"
	GuardianPermissionNotesWrite       = "parent_portal.notes.write"
	GuardianPermissionEnrollmentsView  = "parent_portal.enrollments.view"
	GuardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"
	// GuardianPermissionRequestSubmit allows a parent to submit (and withdraw)
	// structured change-requests — care-schedule and master-data changes that
	// OVERWRITE the child's permanent schedule / name / contact data once staff
	// confirm them. It is deliberately separate from notes.write (plain chat): a
	// guardian must not gain authority to mutate master data just because they
	// may send a message. See .claude/rules/guardian-parent-permissions.md.
	GuardianPermissionRequestSubmit = "parent_portal.request.submit"
	// GuardianPermissionGuardianEdit allows a parent to edit the contact data
	// (name, email, phone, address) of contact-only guardians of their child,
	// plus the per-child pickup note and emergency priority. It never permits
	// editing a guardian who holds their own portal account (#1667).
	GuardianPermissionGuardianEdit = "parent_portal.guardian.edit"
	// GuardianPermissionPickupManage allows a parent to set the per-child
	// can_pickup and is_emergency_contact flags on any guardian of their child.
	// Granting/revoking pickup authority is safety-relevant, so only the full
	// guardian presets (primary/legal/co) receive it (#1667).
	GuardianPermissionPickupManage = "parent_portal.pickup.manage"
)

const (
	GuardianRolePrimaryGuardian = "primary_guardian"
	GuardianRoleLegalGuardian   = "legal_guardian"
	GuardianRoleCoGuardian      = "co_guardian"
	GuardianRoleEmergency       = "emergency_contact"
	GuardianRolePickupOnly      = "pickup_only"
	GuardianRoleSocialWorker    = "social_worker"
	GuardianRoleCustom          = "custom"
)

var fullParentPortalPermissions = []string{
	GuardianPermissionPortalAccess,
	GuardianPermissionSickNoteSubmit,
	GuardianPermissionNotesWrite,
	GuardianPermissionRequestSubmit,
	GuardianPermissionEnrollmentsView,
	GuardianPermissionEnrollmentSubmit,
	GuardianPermissionGuardianEdit,
	GuardianPermissionPickupManage,
}

// Guardian permission checks and grants are authorization concerns, not data
// facts on the model entities. They lived on PersonGuardian / StudentGuardian
// (HasPermission / GrantPermission / RevokePermission); they move here per
// issue #586, Rule 12. The PersonGuardian variants operate on the model's JSON
// `Permissions` string; the StudentGuardian variant operates on its decoded map.

func NormalizeGuardianRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case GuardianRolePrimaryGuardian:
		return GuardianRolePrimaryGuardian
	case GuardianRoleLegalGuardian:
		return GuardianRoleLegalGuardian
	case GuardianRoleCoGuardian:
		return GuardianRoleCoGuardian
	case GuardianRoleEmergency:
		return GuardianRoleEmergency
	case GuardianRolePickupOnly:
		return GuardianRolePickupOnly
	case GuardianRoleSocialWorker:
		return GuardianRoleSocialWorker
	default:
		return GuardianRoleCustom
	}
}

// IsFullGuardianRole reports whether the role is one of the full guardian
// presets (primary/legal/co). These are real legal guardians, not helpers:
// their personal contact data and pickup/emergency authority are managed by
// themselves (once they hold a portal account) or the school — never by another
// parent, even when they have not yet registered a portal account (#1667). It
// is the role-level counterpart to the account-level HasAccount protection: a
// non-registered co-parent must not be editable just because they lack a login.
func IsFullGuardianRole(role string) bool {
	switch NormalizeGuardianRole(role) {
	case GuardianRolePrimaryGuardian, GuardianRoleLegalGuardian, GuardianRoleCoGuardian:
		return true
	default:
		return false
	}
}

// DefaultStudentGuardianRole returns the initial role preset for a relationship
// when a caller did not explicitly choose one. It is deliberately conservative:
// legal/parent relationships keep parent-portal access, while pickup and
// emergency contact flags do not imply portal access.
func DefaultStudentGuardianRole(relationshipType string, isPrimary, isEmergencyContact, canPickup bool) string {
	if isPrimary {
		return GuardianRolePrimaryGuardian
	}
	switch strings.ToLower(strings.TrimSpace(relationshipType)) {
	case string(users.RelationshipParent), string(users.RelationshipGuardian):
		return GuardianRoleLegalGuardian
	}
	if canPickup {
		return GuardianRolePickupOnly
	}
	if isEmergencyContact {
		return GuardianRoleEmergency
	}
	return GuardianRoleCustom
}

// StudentGuardianPermissionSet returns the concrete stored permissions for a
// role preset. V1 derives permissions server-side from the role; the UI never
// sends raw permission maps.
func StudentGuardianPermissionSet(role string) map[string]interface{} {
	perms := map[string]interface{}{}
	switch NormalizeGuardianRole(role) {
	case GuardianRolePrimaryGuardian, GuardianRoleLegalGuardian, GuardianRoleCoGuardian:
		for _, permission := range fullParentPortalPermissions {
			perms[permission] = true
		}
	}
	return perms
}

func StudentGuardianGrantPermissions(sg *users.StudentGuardian, permissions ...string) {
	if sg == nil {
		return
	}
	perms := sg.GetPermissions()
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		perms[permission] = true
	}
}

func ApplyStudentGuardianRole(sg *users.StudentGuardian, role string) {
	if sg == nil {
		return
	}
	normalized := NormalizeGuardianRole(role)
	sg.GuardianRole = normalized
	sg.Permissions = StudentGuardianPermissionSet(normalized)
}

func ApplyDefaultStudentGuardianRole(sg *users.StudentGuardian) {
	if sg == nil {
		return
	}
	role := DefaultStudentGuardianRole(sg.RelationshipType, sg.IsPrimary, sg.IsEmergencyContact, sg.CanPickup)
	ApplyStudentGuardianRole(sg, role)
}

// PersonGuardianHasPermission reports whether the guardian relationship grants
// the named permission. An empty or invalid permissions JSON yields false.
func PersonGuardianHasPermission(pg *users.PersonGuardian, permission string) bool {
	if pg == nil || pg.Permissions == "" {
		return false
	}
	perms := map[string]bool{}
	if err := json.Unmarshal([]byte(pg.Permissions), &perms); err != nil {
		return false
	}
	return perms[permission]
}

// PersonGuardianGrantPermission grants the named permission on the guardian
// relationship and re-serializes the JSON `Permissions` field. Invalid existing
// JSON is reset to a fresh map before the grant is applied.
func PersonGuardianGrantPermission(pg *users.PersonGuardian, permission string) error {
	if pg == nil {
		return nil
	}
	perms := map[string]bool{}
	if pg.Permissions != "" {
		if err := json.Unmarshal([]byte(pg.Permissions), &perms); err != nil {
			perms = map[string]bool{}
		}
	}
	perms[permission] = true
	bytes, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	pg.Permissions = string(bytes)
	return nil
}

// PersonGuardianRevokePermission removes the named permission from the guardian
// relationship and re-serializes the JSON `Permissions` field. A missing or
// empty permission set is a no-op.
func PersonGuardianRevokePermission(pg *users.PersonGuardian, permission string) error {
	if pg == nil || pg.Permissions == "" {
		return nil
	}
	perms := map[string]bool{}
	if err := json.Unmarshal([]byte(pg.Permissions), &perms); err != nil {
		return nil
	}
	delete(perms, permission)
	bytes, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	pg.Permissions = string(bytes)
	return nil
}

// StudentGuardianHasPermission reports whether the student-guardian relationship
// grants the named permission. A boolean value is returned directly; any other
// non-nil value is treated as granted, matching the prior model behavior.
func StudentGuardianHasPermission(sg *users.StudentGuardian, permission string) bool {
	if sg == nil {
		return false
	}
	value, exists := sg.GetPermissions()[permission]
	if !exists {
		return false
	}
	if boolValue, ok := value.(bool); ok {
		return boolValue
	}
	return value != nil
}
