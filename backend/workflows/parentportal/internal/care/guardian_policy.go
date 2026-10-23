package care

import "encoding/json"

// GuardianPolicy is the workflow's boundary to the canonical relationship
// role presets and stored permission-value semantics. Membership is not a grant.
type GuardianPolicy interface {
	IsFullRole(string) bool
	PermissionGranted(any) bool
	DefaultRole(relationship string, primary, emergency, pickup bool) (string, []string)
	RelationshipTypeValid(string) bool
	NormalizeRelationship(*GuardianRelationship) error
	PermissionGrantedJSON(json.RawMessage, string) bool
	MissingProfileError() error
}

// Relationship permission names are the workflow's action protocol, not grants.
const (
	GuardianPermissionConsentManage           = "parent_portal.consent.manage"
	GuardianPermissionDeclarationSubmit       = "parent_portal.declarations.submit"
	GuardianPermissionEnrollmentSubmit        = "parent_portal.enrollment.submit"
	GuardianPermissionEnrollmentsView         = "parent_portal.enrollments.view"
	GuardianPermissionGuardianEdit            = "parent_portal.guardian.edit"
	GuardianPermissionMasterDataEdit          = "parent_portal.master_data.edit"
	GuardianPermissionMasterDataRequest       = "parent_portal.master_data.request"
	GuardianPermissionMealParticipationManage = "parent_portal.meal_participation.manage"
	GuardianPermissionNotesWrite              = "parent_portal.notes.write"
	GuardianPermissionPickupManage            = "parent_portal.pickup.manage"
	GuardianPermissionPortalAccess            = "parent_portal.access"
	GuardianPermissionRelatedAccountsManage   = "parent_portal.related_accounts.manage"
	GuardianPermissionRequestSubmit           = "parent_portal.request.submit"
	GuardianPermissionSickNoteSubmit          = "parent_portal.sick_note.submit"
	GuardianRoleSocialWorker                  = "social_worker"
)

func (s *Service) applyDefaultGuardianRole(link *GuardianRelationship) {
	if link == nil {
		return
	}
	role, granted := s.GuardianPolicy.DefaultRole(link.RelationshipType, link.IsPrimary, link.IsEmergencyContact, link.CanPickup)
	permissions := make(map[string]bool, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	link.GuardianRole = role
	link.Permissions, _ = json.Marshal(permissions)
}
func (s *Service) guardianHasPermission(link *GuardianRelationship, permission string) bool {
	if link == nil {
		return false
	}
	// The retained authorization getter materializes a nil permission object.
	if link.Permissions == nil {
		link.Permissions = json.RawMessage(`{}`)
	}
	return s.GuardianPolicy.PermissionGrantedJSON(link.Permissions, permission)
}
