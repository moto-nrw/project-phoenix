package platform

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Reachability values (mirrors the chk_email_delivery_reachability constraint).
// This records WHY no mail was queued for a recipient — it is deliberately
// independent of the mail's own status, because the two answer different
// questions. With email_audience='all_contacts' a person can legitimately be
// ReachabilityNoPortal AND have a delivered mail; the staff matrix shows both
// columns side by side (#2384).
const (
	// ReachabilityOK — a mail was queued for this person.
	ReachabilityOK = "ok"
	// ReachabilityNoEmail — the guardian profile has no e-mail address. Not a
	// delivery failure: missing data the school can fix.
	ReachabilityNoEmail = "no_email"
	// ReachabilityNoPortal — the guardian has no parent-portal access, so they
	// can never acknowledge in moto. Whether they still receive the mail depends
	// on the announcement's email_audience.
	ReachabilityNoPortal = "no_portal"
	// ReachabilityExcluded — deliberately left out, e.g. the guardian opted out
	// of this notification type.
	ReachabilityExcluded = "excluded"
)

// EmailDelivery is one addressed person for one entity's mail — the row behind
// the staff-facing recipient matrix and, once #1937 lands, the anchor for
// provider delivery events.
//
// It is intentionally generic (platform schema, related_entity_* rather than
// announcement_id): every mail kind needs the same per-recipient record, so the
// delivery-status work fills this table in rather than replacing it.
type EmailDelivery struct {
	base.Model `bun:"schema:platform,table:email_delivery"`
	base.TenantModel

	RelatedEntityType string `bun:"related_entity_type,notnull" json:"related_entity_type"`
	RelatedEntityID   int64  `bun:"related_entity_id,notnull" json:"related_entity_id"`

	// OutboxID is nil when nothing was queued — no address, or excluded from the
	// e-mail audience. The row still exists so the school can SEE that person and
	// repair the data.
	OutboxID *int64 `bun:"outbox_id" json:"outbox_id,omitempty"`

	GuardianProfileID *int64 `bun:"guardian_profile_id" json:"guardian_profile_id,omitempty"`
	// AccountID is nullable: email_audience='all_contacts' reaches people who
	// have no portal account at all.
	AccountID *int64 `bun:"account_id" json:"account_id,omitempty"`

	// RecipientEmail is a SNAPSHOT of what was actually sent to, not what the
	// profile says today — correcting a typo later must not rewrite the history
	// of a failed delivery.
	RecipientEmail *string `bun:"recipient_email" json:"recipient_email,omitempty"`

	Reachability string `bun:"reachability,notnull,nullzero,default:'ok'" json:"reachability"`

	// ProviderMessageID stays empty until #1999 lands an API-based provider that
	// reports one.
	ProviderMessageID *string `bun:"provider_message_id" json:"provider_message_id,omitempty"`
}

// Queued reports whether a mail was actually enqueued for this recipient.
func (d *EmailDelivery) Queued() bool { return d.OutboxID != nil }

// EmailDeliveryRepository is the tenant-scoped data access for delivery rows.
// All methods run inside the caller's tenant transaction.
type EmailDeliveryRepository interface {
	// ReplaceForEntity swaps the whole recipient set of one entity in a single
	// statement pair: delete what is there, insert the new rows. Publishing
	// resolves the audience live, so a stale recipient (a guardian unlinked since
	// the last attempt) must not survive a republish.
	ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []*EmailDelivery) error

	// DeleteForEntity drops every delivery row of an entity — used when an
	// announcement is retracted or deleted, mirroring the pending-outbox cancel.
	DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error)

	// ListForEntity returns the recipients of an entity together with the mail's
	// current outbox state, ordered by name. The join is what turns a queued row
	// into "versendet" or "fehlgeschlagen" for the staff matrix.
	ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]*EmailDeliveryStatus, error)

	// AttachOutbox links an already-written delivery row to the outbox row that
	// was queued for it.
	AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error
}

// EmailDeliveryStatus is one recipient joined with the mail's outbox state — the
// projection behind the staff recipient matrix.
//
// EmailStatus deliberately reports what we actually know:
//
//	not_sent  — nothing was queued at all; Reachability says why (no address,
//	            no portal access, or deliberately excluded). Kept as its own
//	            value rather than reusing the reachability wording, so the two
//	            columns stay independent
//	pending   — queued, not handed over yet
//	sent      — handed to the mail server. NOT "delivered": without provider
//	            webhooks (#1937) we cannot know it reached the mailbox, and the
//	            UI must say "Versendet" rather than "Zugestellt".
//	failed    — the mail server refused it, or the send was cancelled
type EmailDeliveryStatus struct {
	DeliveryID        int64      `bun:"delivery_id" json:"delivery_id"`
	GuardianProfileID *int64     `bun:"guardian_profile_id" json:"guardian_profile_id,omitempty"`
	AccountID         *int64     `bun:"account_id" json:"account_id,omitempty"`
	FirstName         string     `bun:"first_name" json:"first_name"`
	LastName          string     `bun:"last_name" json:"last_name"`
	RecipientEmail    *string    `bun:"recipient_email" json:"recipient_email,omitempty"`
	Reachability      string     `bun:"reachability" json:"reachability"`
	EmailStatus       string     `bun:"email_status" json:"email_status"`
	LastError         *string    `bun:"last_error" json:"last_error,omitempty"`
	SentAt            *time.Time `bun:"sent_at" json:"sent_at,omitempty"`
	Attempts          int        `bun:"attempts" json:"attempts"`
}
