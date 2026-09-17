package platform

// Pre-defined kinds. The column is text and the worker looks up renderers
// by kind, so new kinds can be added by registering a renderer at startup
// without a schema change. These constants exist so the in-tree call sites
// stay typo-safe.
const (
	EmailKindGuardianInvitation                 = "guardian_invitation"
	EmailKindParentAnnouncement                 = "parent_announcement"
	EmailKindEnrollmentSubmitted                = "enrollment_submitted"
	EmailKindEnrollmentAdminNotify              = "enrollment_admin_notification"
	EmailKindEnrollmentApproved                 = "enrollment_approved"
	EmailKindEnrollmentWaitlisted               = "enrollment_waitlisted"
	EmailKindEnrollmentRejected                 = "enrollment_rejected"
	EmailKindEnrollmentDecisionDigest           = "enrollment_decision_digest"
	EmailKindEnrollmentChangeRequestSubmitted   = "enrollment_change_request_submitted"
	EmailKindEnrollmentChangeRequestQuestion    = "enrollment_change_request_question"
	EmailKindEnrollmentChangeRequestParentReply = "enrollment_change_request_parent_reply"
	EmailKindEnrollmentChangeRequestApproved    = "enrollment_change_request_approved"
	EmailKindEnrollmentChangeRequestRejected    = "enrollment_change_request_rejected"

	// Rollover (phase renewal) email kinds. The renderers are
	// minimal-text placeholders in slice 1; proper branded templates
	// land in a follow-up.
	EmailKindEnrollmentRolloverOptIn  = "enrollment_rollover_opt_in"
	EmailKindEnrollmentRolloverOptOut = "enrollment_rollover_opt_out"

	// Calendar appointment (Termine) notification kinds. Sent to guardian
	// recipients of a parent-facing appointment.
	EmailKindAppointmentPublished = "appointment_published"
	EmailKindAppointmentUpdated   = "appointment_updated"
	EmailKindAppointmentCancelled = "appointment_cancelled"
	EmailKindAppointmentReminder  = "appointment_reminder"
	EmailKindParentMessage        = "parent_message"
)

// Pre-defined related_entity_type values.
const (
	EmailRelatedTypeGuardianInvitation = "guardian_invitation"
	EmailRelatedTypeEnrollmentRequest  = "enrollment_request"
	EmailRelatedTypeAppointment        = "calendar_appointment"
	EmailRelatedTypeParentMessage      = "parent_message"
)
