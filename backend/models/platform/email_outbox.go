package platform

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const EmailOutboxStatusPending = "pending"

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

// EmailOutbox is the legacy renderer input DTO. Durable persistence belongs to
// the Delivery capability; renderers receive this compatibility projection.
type EmailOutbox struct {
	base.Model `bun:"schema:platform,table:email_outbox"`
	base.TenantModel
	Kind              string         `bun:"kind,notnull" json:"kind"`
	IdempotencyKey    *string        `bun:"idempotency_key" json:"idempotency_key,omitempty"`
	RelatedEntityType *string        `bun:"related_entity_type" json:"related_entity_type,omitempty"`
	RelatedEntityID   *int64         `bun:"related_entity_id" json:"related_entity_id,omitempty"`
	Payload           map[string]any `bun:"payload,type:jsonb,notnull,default:'{}'" json:"payload"`
	Status            string         `bun:"status,notnull,default:'pending'" json:"status"`
	Attempts          int            `bun:"attempts,notnull,default:0" json:"attempts"`
	LastError         *string        `bun:"last_error" json:"last_error,omitempty"`
	NextRetryAt       time.Time      `bun:"next_retry_at,notnull" json:"next_retry_at"`
	SentAt            *time.Time     `bun:"sent_at" json:"sent_at,omitempty"`
}
