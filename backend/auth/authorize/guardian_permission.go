package authorize

import (
	"strings"
)

type studentGuardianRecord interface {
	GuardianAuthorizationData() (relationshipType, role string, primary, emergency, pickup bool, permissions map[string]interface{})
	SetGuardianAuthorizationData(role string, permissions map[string]interface{})
}

const (
	GuardianPermissionPortalAccess     = "parent_portal.access"
	GuardianPermissionSickNoteSubmit   = "parent_portal.sick_note.submit"
	GuardianPermissionNotesWrite       = "parent_portal.notes.write"
	GuardianPermissionEnrollmentsView  = "parent_portal.enrollments.view"
	GuardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"
	// GuardianPermissionPollResponse allows a guardian to submit or replace a
	// per-child parent-announcement poll response. It is deliberately separate
	// from portal access because viewing a poll must not imply voting authority.
	GuardianPermissionPollResponse = "parent_portal.poll.response"
	// GuardianPermissionRequestSubmit allows a parent to submit (and withdraw)
	// structured change-requests — care-schedule changes that OVERWRITE the
	// child's permanent weekday schedule once staff confirm them. It is
	// deliberately separate from notes.write (plain chat): a guardian must not
	// gain authority to mutate the schedule just because they may send a
	// message. See .claude/rules/guardian-parent-permissions.md.
	GuardianPermissionRequestSubmit = "parent_portal.request.submit"
	// GuardianPermissionMasterDataEdit gates Track A direct Stammdaten edits
	// (health info, contact data) that apply immediately.
	GuardianPermissionMasterDataEdit = "parent_portal.master_data.edit"
	// GuardianPermissionMasterDataRequest gates Track B change requests for
	// high-stakes / OGS-coupled fields (child name, birthday, permanent
	// weekday Gehzeit) that require staff approval.
	GuardianPermissionMasterDataRequest = "parent_portal.master_data.request"
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
	GuardianPermissionPollResponse,
	GuardianPermissionMasterDataEdit,
	GuardianPermissionMasterDataRequest,
	GuardianPermissionGuardianEdit,
	GuardianPermissionPickupManage,
}

// Guardian permission checks and grants are authorization concerns, not data
// facts on the model entities. They lived on the guardian model entities
// (HasPermission / GrantPermission / RevokePermission); they move here per
// issue #586, Rule 12. The StudentGuardian variant operates on its decoded
// permissions map.

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
	case "parent", "guardian":
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

func ApplyStudentGuardianRole(sg studentGuardianRecord, role string) {
	if sg == nil {
		return
	}
	normalized := NormalizeGuardianRole(role)
	sg.SetGuardianAuthorizationData(normalized, StudentGuardianPermissionSet(normalized))
}

func ApplyDefaultStudentGuardianRole(sg studentGuardianRecord) {
	if sg == nil {
		return
	}
	relationshipType, _, primary, emergency, pickup, _ := sg.GuardianAuthorizationData()
	role := DefaultStudentGuardianRole(relationshipType, primary, emergency, pickup)
	ApplyStudentGuardianRole(sg, role)
}

// StudentGuardianHasPermission reports whether the student-guardian relationship
// grants the named permission. A boolean value is returned directly; any other
// non-nil value is treated as granted, matching the prior model behavior.
func StudentGuardianHasPermission(sg studentGuardianRecord, permission string) bool {
	if sg == nil {
		return false
	}
	_, _, _, _, _, permissions := sg.GuardianAuthorizationData()
	value, exists := permissions[permission]
	if !exists {
		return false
	}
	if boolValue, ok := value.(bool); ok {
		return boolValue
	}
	return value != nil
}
