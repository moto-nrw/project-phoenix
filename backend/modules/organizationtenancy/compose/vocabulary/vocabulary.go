// Package vocabulary names the Organisation & Tenancy records, repositories and constants
// the legacy composition still binds (#2747). The owner's composition
// re-exports them under its own path, so the legacy composition depends on the
// composition instead of the domain package. Shrink it as consumers move to the
// owner's public contract; never add a name nothing uses.
package vocabulary

import upstream "github.com/moto-nrw/project-phoenix/models/platform"

type (
	OperatorAuditLog           = upstream.OperatorAuditLog
	OperatorAuditLogRepository = upstream.OperatorAuditLogRepository
	OutboxEnqueueRequest       = upstream.OutboxEnqueueRequest
	OutboxEnqueued             = upstream.OutboxEnqueued
	OutboxEnqueuer             = upstream.OutboxEnqueuer
	OutboxResultEnqueuer       = upstream.OutboxResultEnqueuer
)

const (
	ActionCreate                                = upstream.ActionCreate
	ActionDelete                                = upstream.ActionDelete
	ActionUpdate                                = upstream.ActionUpdate
	EmailKindAppointmentCancelled               = upstream.EmailKindAppointmentCancelled
	EmailKindAppointmentPublished               = upstream.EmailKindAppointmentPublished
	EmailKindAppointmentReminder                = upstream.EmailKindAppointmentReminder
	EmailKindAppointmentUpdated                 = upstream.EmailKindAppointmentUpdated
	EmailKindEnrollmentAdminNotify              = upstream.EmailKindEnrollmentAdminNotify
	EmailKindEnrollmentApproved                 = upstream.EmailKindEnrollmentApproved
	EmailKindEnrollmentChangeRequestApproved    = upstream.EmailKindEnrollmentChangeRequestApproved
	EmailKindEnrollmentChangeRequestParentReply = upstream.EmailKindEnrollmentChangeRequestParentReply
	EmailKindEnrollmentChangeRequestQuestion    = upstream.EmailKindEnrollmentChangeRequestQuestion
	EmailKindEnrollmentChangeRequestRejected    = upstream.EmailKindEnrollmentChangeRequestRejected
	EmailKindEnrollmentChangeRequestSubmitted   = upstream.EmailKindEnrollmentChangeRequestSubmitted
	EmailKindEnrollmentDecisionDigest           = upstream.EmailKindEnrollmentDecisionDigest
	EmailKindEnrollmentRejected                 = upstream.EmailKindEnrollmentRejected
	EmailKindEnrollmentRolloverOptIn            = upstream.EmailKindEnrollmentRolloverOptIn
	EmailKindEnrollmentRolloverOptOut           = upstream.EmailKindEnrollmentRolloverOptOut
	EmailKindEnrollmentSubmitted                = upstream.EmailKindEnrollmentSubmitted
	EmailKindEnrollmentWaitlisted               = upstream.EmailKindEnrollmentWaitlisted
	EmailKindGuardianInvitation                 = upstream.EmailKindGuardianInvitation
	EmailKindGuardianWelcome                    = upstream.EmailKindGuardianWelcome
	EmailKindParentAnnouncement                 = upstream.EmailKindParentAnnouncement
	EmailKindParentMessage                      = upstream.EmailKindParentMessage
	EmailKindStaffWelcome                       = upstream.EmailKindStaffWelcome
	EmailRelatedTypeGuardianInvitation          = upstream.EmailRelatedTypeGuardianInvitation
	EmailRelatedTypeGuardianProfile             = upstream.EmailRelatedTypeGuardianProfile
	EmailRelatedTypeSchoolInvitation            = upstream.EmailRelatedTypeSchoolInvitation
)
