package care

import "github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"

// These keys and policy values are the guardian workflow's settings protocol.
// The root binds the port to the tenant registry; no defaults are supplied here.
const (
	KeyEnrollmentBookingsAuthoritative  = "enrollment.bookings_authoritative"
	KeyEnrollmentCareOfferingsEnabled   = "enrollment.care_offerings_enabled"
	KeyEnrollmentOfferingChangesEnabled = "enrollment.offering_changes_enabled"
	KeyGuardianParentCanRemove          = "guardians.parent_can_remove"
	KeyGuardianParentInviteMode         = "guardians.parent_invite_mode"
	KeyMealPlanEnabled                  = "operations.meal_plan_enabled"
	KeyParentCareArrivalRequestEnabled  = "operations.parent_care_arrival_request_enabled"
	KeyParentCareModeRequestEnabled     = "operations.parent_care_mode_request_enabled"
	KeyParentCarePickupRequestEnabled   = "operations.parent_care_pickup_request_enabled"
	KeyParentExcusedReportsEnabled      = "operations.parent_excused_reports_enabled"
	KeyParentExcusedRequiresApproval    = "operations.parent_excused_requires_approval"
	KeyParentGuardianManagementEnabled  = "operations.parent_guardian_management_enabled"
	KeyParentMasterDataEditEnabled      = "operations.parent_master_data_edit_enabled"
	KeyParentMasterDataRequestEnabled   = "operations.parent_master_data_request_enabled"
	KeyParentNewsEnabled                = "operations.parent_news_enabled"
	KeyParentNotesEnabled               = "operations.parent_notes_enabled"
	KeyParentPickupChangeCutoffTime     = "operations.parent_pickup_change_cutoff_time"
	KeyParentPickupChangeEnabled        = "operations.parent_pickup_change_enabled"
	KeyParentRequestReasonPolicy        = "operations.parent_request_reason_policy"
	KeyParentSickNoteEnabled            = "operations.parent_sick_note_enabled"
	KeyParentSickReportsEnabled         = "operations.parent_sick_reports_enabled"
	KeyParentSickRequiresApproval       = "operations.parent_sick_requires_approval"
	ParentInviteModeDirect              = "direct"
	ParentInviteModeDisabled            = "disabled"
	ParentInviteModeStaffApproval       = "staff_approval"
	ReasonPolicyBoth                    = parentrequests.ReasonPolicyBoth
	ReasonPolicyGuardians               = parentrequests.ReasonPolicyGuardians
	ReasonPolicyNobody                  = parentrequests.ReasonPolicyNobody
	ReasonPolicyStaff                   = parentrequests.ReasonPolicyStaff
)
