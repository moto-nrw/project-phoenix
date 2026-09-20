package authmodels

// Approval-status values for GuardianInvitation.ApprovalStatus.
const (
	GuardianInvitationApprovalNotRequired = "not_required" // staff invites + parent direct mode
	GuardianInvitationApprovalPending     = "pending"      // parent invite awaiting staff approval
	GuardianInvitationApprovalApproved    = "approved"     // staff approved; email dispatched
	GuardianInvitationApprovalRejected    = "rejected"     // staff rejected; no access granted
)
