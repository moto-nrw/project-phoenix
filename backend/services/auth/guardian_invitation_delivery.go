package auth

import (
	"context"
	"errors"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// GuardianInvitationDelivery is what the tenant UI needs to tell a school
// whether an invitation actually reached the family.
//
// It is a LIST of attempts, not a single status: every resend enqueues another
// outbox row, and a school looking at "why hasn't this parent registered" needs
// to see that the first attempt bounced and the second is still on its way.
type GuardianInvitationDelivery struct {
	InvitationID int64                     `json:"invitation_id"`
	Attempts     []GuardianDeliveryAttempt `json:"attempts"` // newest first
}

// GuardianDeliveryAttempt is one dispatch of the invitation email.
//
// DispatchStatus and DeliveryStatus are deliberately separate fields with
// separate vocabularies. DispatchStatus says whether we handed the message to
// the mail server; DeliveryStatus says what the receiving side did with it.
// Collapsing them is exactly the bug this feature exists to fix.
type GuardianDeliveryAttempt struct {
	OutboxID       int64                   `json:"outbox_id"`
	DispatchStatus string                  `json:"dispatch_status"` // pending|sending|sent|failed
	DeliveryStatus string                  `json:"delivery_status"` // unknown|queued|deferred|delivered|soft_bounced|suppressed|complained|bounced
	QueuedAt       time.Time               `json:"queued_at"`
	SentAt         *time.Time              `json:"sent_at,omitempty"`
	DeliveryAt     *time.Time              `json:"delivery_status_at,omitempty"`
	Attempts       int                     `json:"attempts"`
	NextRetryAt    *time.Time              `json:"next_retry_at,omitempty"`
	LastError      *string                 `json:"last_error,omitempty"`      // our own dispatch failure
	DeliveryDetail *string                 `json:"delivery_detail,omitempty"` // provider explanation
	Events         []GuardianDeliveryEvent `json:"events"`
}

// GuardianDeliveryEvent is one provider report, trimmed to what a school may
// see. The raw payload, recipient address and correlation identifiers stay
// server-side: they carry arbitrary provider data and add nothing for staff.
type GuardianDeliveryEvent struct {
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	ReasonCode *string   `json:"reason_code,omitempty"`
	Reason     *string   `json:"reason,omitempty"`
}

// DeliveryStatus returns every dispatch attempt for an invitation together
// with its provider delivery outcome.
//
// Tenant scoping is entirely RLS: both tables are tenant-isolated and the
// caller runs inside the request's tenant transaction, so an invitation from
// another school simply yields no rows.
//
// German labels are rendered by the frontend from these codes. The intended
// mapping, pinned here so the two sides do not drift:
//
//	dispatch: pending → "eingeplant", sending → "wird gesendet",
//	          sent → "übergeben", failed → "Versand fehlgeschlagen"
//	delivery: delivered → "zugestellt", deferred → "verzögert",
//	          bounced → "nicht zustellbar", soft_bounced → "vorübergehend nicht zustellbar",
//	          complained → "als Spam markiert", suppressed → "blockiert",
//	          unknown/queued → "keine Rückmeldung"
func (s *guardianInvitationService) DeliveryStatus(ctx context.Context, invitationID int64) (*GuardianInvitationDelivery, error) {
	if s.OutboxReader == nil {
		return nil, errors.New("guardian invitation service not wired for delivery status")
	}

	allRows, err := s.OutboxReader.FindByRelatedEntity(ctx, platformModels.EmailRelatedTypeGuardianInvitation, invitationID)
	if err != nil {
		return nil, err
	}

	// Keep only actual invitation emails. Other kinds legitimately hang off the
	// same related entity — the ops bounce notice does, so support can find it
	// from the invitation — and counting one as a dispatch attempt would let a
	// freshly queued notice mask the very bounce it reports.
	rows := make([]*platformModels.EmailOutbox, 0, len(allRows))
	for _, row := range allRows {
		if row.Kind == platformModels.EmailKindGuardianInvitation {
			rows = append(rows, row)
		}
	}

	result := &GuardianInvitationDelivery{
		InvitationID: invitationID,
		Attempts:     make([]GuardianDeliveryAttempt, 0, len(rows)),
	}
	if len(rows) == 0 {
		return result, nil
	}

	eventsByOutbox := map[int64][]GuardianDeliveryEvent{}
	if s.DeliveryEvents != nil {
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		events, evErr := s.DeliveryEvents.ListByOutboxIDs(ctx, ids)
		if evErr != nil {
			return nil, evErr
		}
		for _, ev := range events {
			eventsByOutbox[ev.OutboxID] = append(eventsByOutbox[ev.OutboxID], GuardianDeliveryEvent{
				Type:       ev.EventType,
				OccurredAt: ev.OccurredAt,
				ReasonCode: ev.ReasonCode,
				Reason:     ev.Reason,
			})
		}
	}

	for _, row := range rows {
		attempt := GuardianDeliveryAttempt{
			OutboxID:       row.ID,
			DispatchStatus: row.Status,
			DeliveryStatus: row.DeliveryStatus,
			QueuedAt:       row.CreatedAt,
			SentAt:         row.SentAt,
			DeliveryAt:     row.DeliveryStatusAt,
			Attempts:       row.Attempts,
			LastError:      row.LastError,
			DeliveryDetail: row.DeliveryDetail,
			Events:         eventsByOutbox[row.ID],
		}
		// next_retry_at is only meaningful while a retry is actually pending;
		// on a sent or failed row it is a stale artefact of the last attempt.
		if row.Status == platformModels.EmailOutboxStatusPending {
			next := row.NextRetryAt
			attempt.NextRetryAt = &next
		}
		if attempt.Events == nil {
			attempt.Events = []GuardianDeliveryEvent{}
		}
		result.Attempts = append(result.Attempts, attempt)
	}

	return result, nil
}
