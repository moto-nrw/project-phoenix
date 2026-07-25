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

// Email delivery status values — what the RECEIVING side did with a message
// we already handed to the provider. Orthogonal to the dispatch status above,
// which only reports whether our own SMTP submission succeeded. `sent` +
// `unknown` is the normal state right after dispatch and must never be
// presented to a user as "zugestellt".
const (
	EmailDeliveryStatusUnknown     = "unknown"      // no provider feedback yet
	EmailDeliveryStatusQueued      = "queued"       // provider accepted the submission
	EmailDeliveryStatusDeferred    = "deferred"     // transient 4xx, provider still retrying
	EmailDeliveryStatusDelivered   = "delivered"    // recipient MTA accepted it
	EmailDeliveryStatusSoftBounced = "soft_bounced" // provider gave up after transient failures
	EmailDeliveryStatusSuppressed  = "suppressed"   // suppression list, never attempted
	EmailDeliveryStatusComplained  = "complained"   // recipient marked it as spam
	EmailDeliveryStatusBounced     = "bounced"      // hard bounce, the address is dead
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

	// EmailKindDeliveryFailureNotice is the ops notice sent to the school's
	// configured alert addresses when a message hard-bounces or is reported
	// as spam. Rows of this kind never generate a notice of their own — see
	// the loop guard in the delivery service.
	EmailKindDeliveryFailureNotice = "email_delivery_failure_notice"
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

	// Delivery tracking. MessageID is the RFC 5322 Message-ID we mint at
	// dispatch time; ProviderMessageID is the transport's own identifier when
	// it reports one. Both are correlation keys for incoming provider events
	// and are never exposed through a tenant API.
	MessageID          *string    `bun:"message_id" json:"-"`
	ProviderMessageID  *string    `bun:"provider_message_id" json:"-"`
	DeliveryStatus     string     `bun:"delivery_status,notnull,default:'unknown'" json:"delivery_status"`
	DeliveryStatusRank int        `bun:"delivery_status_rank,notnull,default:0" json:"-"`
	DeliveryStatusAt   *time.Time `bun:"delivery_status_at" json:"delivery_status_at,omitempty"`
	DeliveryDetail     *string    `bun:"delivery_detail" json:"delivery_detail,omitempty"`
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
	switch e.DeliveryStatus {
	case "":
		e.DeliveryStatus = EmailDeliveryStatusUnknown
	case EmailDeliveryStatusUnknown, EmailDeliveryStatusQueued, EmailDeliveryStatusDeferred,
		EmailDeliveryStatusDelivered, EmailDeliveryStatusSoftBounced, EmailDeliveryStatusSuppressed,
		EmailDeliveryStatusComplained, EmailDeliveryStatusBounced:
		// ok
	default:
		return errors.New("email outbox delivery status is not a known value")
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

	// FindByRelatedEntities is the batched form used by list UIs. Tenant-scoped.
	FindByRelatedEntities(ctx context.Context, relatedType string, relatedIDs []int64) ([]*EmailOutbox, error)

	// CancelPendingByRelatedEntity marks every still-pending row for a related
	// entity as 'failed' with the given reason, so the worker (which only claims
	// 'pending' rows) never sends them. Used when the triggering entity is
	// retracted before its async send. Rows already claimed ('sending') or
	// terminal ('sent'/'failed') are left untouched. Tenant-scoped. Returns the
	// number of rows cancelled.
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)

	// FindByMessageID resolves the outbox row a provider delivery event
	// refers to. Caller must run as phoenix_admin: the webhook is
	// unauthenticated and cross-tenant, so this deliberately does NOT honor
	// app.current_tenant_id — the returned row's tenant_id is what
	// establishes the tenant for everything downstream. Returns (nil, nil)
	// when nothing matches; an unmatched event is a normal, expected outcome.
	//
	// Not expressible via List(filters): the whole point is the missing
	// tenant predicate, which the generic path always applies.
	FindByMessageID(ctx context.Context, messageID string) (*EmailOutbox, error)

	// FindByProviderMessageID is the same lookup keyed on the transport's own
	// identifier, for providers that rewrite our Message-ID.
	FindByProviderMessageID(ctx context.Context, providerMessageID string) (*EmailOutbox, error)

	// CreateDispatchAttempt persists a correlation record before the worker
	// submits the message to the transport.
	CreateDispatchAttempt(ctx context.Context, attempt *EmailDispatchAttempt) error

	// Dispatch-attempt lookups preserve correlation across retries. All are
	// admin-scoped because the webhook has no tenant before correlation.
	FindDispatchAttemptByMessageID(ctx context.Context, messageID string) (*EmailDispatchAttempt, error)
	FindDispatchAttemptByProviderMessageID(ctx context.Context, providerMessageID string) (*EmailDispatchAttempt, error)
	FindDispatchAttemptByCorrelationToken(ctx context.Context, correlationToken string) (*EmailDispatchAttempt, error)

	// SetDispatchIdentifiers stores every available identifier before transport
	// submission so immediate webhooks can correlate it.
	SetDispatchIdentifiers(ctx context.Context, id int64, messageID string, providerMessageID *string) error

	// ApplyDeliveryStatus performs the monotone, out-of-order-safe status
	// transition in a single guarded UPDATE and reports whether the row was
	// advanced. False means the incoming event lost the precedence
	// comparison (a duplicate or a late event) and must not change the row.
	//
	// Not reducible to UpdateColumns: the WHERE clause IS the concurrency
	// control — two webhook requests for the same message cannot interleave
	// into a wrong final state.
	ApplyDeliveryStatus(ctx context.Context, id int64, transition DeliveryTransition) (bool, error)
}

// DeliveryTransition is one candidate delivery-status change. Rank comes from
// the precedence lattice in services/platform; the model deliberately holds no
// policy of its own.
type DeliveryTransition struct {
	Status            string
	Rank              int
	OccurredAt        time.Time
	Detail            string
	ExpectedMessageID string
}

type EmailOutboxCleanupRepository interface {
	EmailOutboxRepository
	DeleteByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) (int64, error)
}
