package platform

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Email outbox status values.
const (
	EmailOutboxStatusPending = "pending"
	EmailOutboxStatusSending = "sending"
	EmailOutboxStatusSent    = "sent"
	EmailOutboxStatusFailed  = "failed"
)

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
)

// Pre-defined related_entity_type values.
const (
	EmailRelatedTypeGuardianInvitation = "guardian_invitation"
	EmailRelatedTypeEnrollmentRequest  = "enrollment_request"
	EmailRelatedTypeAppointment        = "calendar_appointment"
)

// EmailOutbox is a row in platform.email_outbox — the shared outbox the
// worker drains. Tenant-scoped via base.TenantModel; the worker bypasses
// RLS using phoenix_admin to pick up rows across all tenants.
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

// Validate enforces the column-level CHECK constraints in app code so we
// fail fast before the round-trip.
func (e *EmailOutbox) Validate() error {
	e.Kind = strings.TrimSpace(e.Kind)
	if e.Kind == "" {
		return errors.New("email outbox kind is required")
	}
	switch e.Status {
	case "":
		e.Status = EmailOutboxStatusPending
	case EmailOutboxStatusPending, EmailOutboxStatusSending, EmailOutboxStatusSent, EmailOutboxStatusFailed:
		// ok
	default:
		return errors.New("email outbox status must be pending|sending|sent|failed")
	}
	if e.NextRetryAt.IsZero() {
		e.NextRetryAt = time.Now()
	}
	return nil
}

// EmailOutboxRepository describes the DB operations the outbox service +
// worker need. Defined as an interface so the worker can be tested with a
// stub that doesn't require a real DB.
type EmailOutboxRepository interface {
	// Create inserts a new pending outbox row. Tenant comes from context.
	Create(ctx context.Context, row *EmailOutbox) error

	// FindByID returns a single row by primary key. Used by the worker's
	// per-row tenant-scoped second pass and by admin UIs reading status.
	FindByID(ctx context.Context, id int64) (*EmailOutbox, error)

	// ClaimDuePending atomically reserves up to `limit` rows whose
	// next_retry_at <= now and status='pending', flipping them to
	// 'sending'. Uses FOR UPDATE SKIP LOCKED so multiple workers can run
	// in parallel without double-claiming. Caller must run as
	// phoenix_admin (cross-tenant pickup) — the function does NOT honor
	// app.current_tenant_id.
	ClaimDuePending(ctx context.Context, limit int, now time.Time) ([]*EmailOutbox, error)

	// LockSending locks the claimed row FOR UPDATE and reports whether it
	// still exists with status='sending'. The worker calls it inside the
	// same phoenix_admin transaction as the actual send: features cancel
	// in-flight emails by deleting their outbox rows (e.g. enrollment
	// deletion), and that delete either commits before this probe (send is
	// skipped) or blocks on the row lock until the send transaction
	// commits — never in between.
	LockSending(ctx context.Context, id int64) (bool, error)

	// MarkSent transitions a claimed row to 'sent' and records the
	// timestamp. Idempotent — re-marking a sent row is a no-op.
	MarkSent(ctx context.Context, id int64, sentAt time.Time) error

	// MarkRetry pushes a claimed row back to 'pending' with attempts+1,
	// the new last_error, and a delayed next_retry_at.
	MarkRetry(ctx context.Context, id int64, attempts int, lastErr string, nextRetryAt time.Time) error

	// MarkFailed transitions a claimed row to 'failed' (terminal).
	MarkFailed(ctx context.Context, id int64, attempts int, lastErr string) error

	// FindByRelatedEntity returns all rows for a feature's related entity
	// (e.g., "all email rows for enrollment_request 42"). Tenant-scoped.
	FindByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) ([]*EmailOutbox, error)

	// CancelPendingByRelatedEntity marks every still-pending row for a related
	// entity as 'failed' with the given reason, so the worker (which only claims
	// 'pending' rows) never sends them. Used when the triggering entity is
	// retracted before its async send. Rows already claimed ('sending') or
	// terminal ('sent'/'failed') are left untouched. Tenant-scoped. Returns the
	// number of rows cancelled.
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}

type EmailOutboxCleanupRepository interface {
	EmailOutboxRepository
	DeleteByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) (int64, error)
}
