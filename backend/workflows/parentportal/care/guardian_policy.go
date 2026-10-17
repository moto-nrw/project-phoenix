package care

import usersModels "github.com/moto-nrw/project-phoenix/models/users"

// GuardianPolicy is the workflow's boundary to the canonical relationship
// role presets and stored permission-value semantics. Membership is not a grant.
type GuardianPolicy interface {
	IsFullRole(string) bool
	PermissionGranted(any) bool
	DefaultRole(relationship string, primary, emergency, pickup bool) (string, []string)
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
	GuardianPermissionRequestSubmit           = "parent_portal.request.submit"
	GuardianPermissionSickNoteSubmit          = "parent_portal.sick_note.submit"
	GuardianRoleSocialWorker                  = "social_worker"
)

func (s *Service) applyDefaultGuardianRole(link *usersModels.StudentGuardian) {
	if link == nil {
		return
	}
	relationship, _, primary, emergency, pickup, _ := link.GuardianAuthorizationData()
	role, granted := s.GuardianPolicy.DefaultRole(relationship, primary, emergency, pickup)
	permissions := make(map[string]interface{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	link.SetGuardianAuthorizationData(role, permissions)
}

func (s *Service) guardianHasPermission(link *usersModels.StudentGuardian, permission string) bool {
	if link == nil {
		return false
	}
	_, _, _, _, _, permissions := link.GuardianAuthorizationData()
	value, exists := permissions[permission]
	return exists && s.GuardianPolicy.PermissionGranted(value)
}
