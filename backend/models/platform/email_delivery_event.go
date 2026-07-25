package platform

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Delivery event types accepted from providers. Anything outside this set is
// dropped at the adapter boundary rather than stored — an unknown future event
// type must not be able to move a message's status.
const (
	DeliveryEventQueued     = "queued"
	DeliveryEventDelivered  = "delivered"
	DeliveryEventDeferred   = "deferred"
	DeliveryEventBounced    = "bounced"
	DeliveryEventComplained = "complained"
	DeliveryEventSuppressed = "suppressed"
)

// Bounce classes. A soft bounce is a transient failure the provider gave up
// on; a hard bounce means the address itself is dead. Providers that omit the
// class are treated as hard — the conservative reading, because presenting a
// dead address as "vorübergehend" would keep staff waiting forever.
const (
	BounceClassHard = "hard"
	BounceClassSoft = "soft"
)

// EmailDeliveryEvent is one provider report about a message we sent. Rows are
// written exclusively by the webhook ingest path under phoenix_admin and read
// (never written) by tenant requests through RLS.
//
// The full event history is kept even when an event loses the precedence
// comparison and does not move platform.email_outbox.delivery_status — the
// summary column is monotone, the history is complete.
type EmailDeliveryEvent struct {
	base.Model `bun:"schema:platform,table:email_delivery_events"`
	base.TenantModel
	OutboxID          int64     `bun:"outbox_id,notnull" json:"outbox_id"`
	DispatchAttemptID int64     `bun:"dispatch_attempt_id,notnull" json:"-"`
	Provider          string    `bun:"provider,notnull" json:"provider"`
	ProviderEventID   string    `bun:"provider_event_id,notnull" json:"provider_event_id"`
	ProviderMessageID *string   `bun:"provider_message_id" json:"-"`
	EventType         string    `bun:"event_type,notnull" json:"event_type"`
	BounceClass       *string   `bun:"bounce_class" json:"bounce_class,omitempty"`
	OccurredAt        time.Time `bun:"occurred_at,notnull" json:"occurred_at"`
	ReceivedAt        time.Time `bun:"received_at,notnull" json:"received_at"`
	ReasonCode        *string   `bun:"reason_code" json:"reason_code,omitempty"`
	Reason            *string   `bun:"reason" json:"reason,omitempty"`
	Recipient         *string   `bun:"recipient" json:"-"`

	// Raw is the provider payload, kept for support triage. Never serialised
	// to a tenant API: it can contain arbitrary headers and PII.
	Raw map[string]any `bun:"raw,type:jsonb,notnull,default:'{}'" json:"-"`
}

// Validate enforces the column-level CHECK constraints in app code so we fail
// fast before the round-trip.
func (e *EmailDeliveryEvent) Validate() error {
	e.Provider = strings.TrimSpace(e.Provider)
	if e.Provider == "" {
		return errors.New("delivery event provider is required")
	}
	e.ProviderEventID = strings.TrimSpace(e.ProviderEventID)
	if e.ProviderEventID == "" {
		return errors.New("delivery event provider_event_id is required")
	}
	if e.OutboxID == 0 {
		return errors.New("delivery event outbox_id is required")
	}
	if e.DispatchAttemptID == 0 {
		return errors.New("delivery event dispatch_attempt_id is required")
	}
	switch e.EventType {
	case DeliveryEventQueued, DeliveryEventDelivered, DeliveryEventDeferred,
		DeliveryEventBounced, DeliveryEventComplained, DeliveryEventSuppressed:
		// ok
	default:
		return errors.New("delivery event type is not a known value")
	}
	if e.BounceClass != nil {
		switch *e.BounceClass {
		case BounceClassHard, BounceClassSoft:
			// ok
		default:
			return errors.New("delivery event bounce class must be hard|soft")
		}
	}
	if e.OccurredAt.IsZero() {
		return errors.New("delivery event occurred_at is required")
	}
	if e.ReceivedAt.IsZero() {
		e.ReceivedAt = time.Now()
	}
	if e.Raw == nil {
		e.Raw = map[string]any{}
	}
	return nil
}

// EmailDeliveryEventRepository is the data access the ingest path and the
// tenant read API need.
type EmailDeliveryEventRepository interface {
	// CreateIfNew inserts the event and reports false when
	// (provider, provider_event_id) already exists. This unique index is the
	// wire-level idempotency guard: a replayed webhook body is stored once
	// and applied once. Admin-scoped (webhook ingest).
	CreateIfNew(ctx context.Context, event *EmailDeliveryEvent) (bool, error)

	// ListByOutboxIDs returns the events for a set of outbox rows, newest
	// first. Tenant-scoped (read API).
	ListByOutboxIDs(ctx context.Context, outboxIDs []int64) ([]*EmailDeliveryEvent, error)
}

// EmailDeliveryEventCleanupRepository adds the retention sweep.
type EmailDeliveryEventCleanupRepository interface {
	EmailDeliveryEventRepository

	// DeleteOlderThan removes events received before cutoff for the current
	// tenant and returns the number of rows deleted.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
