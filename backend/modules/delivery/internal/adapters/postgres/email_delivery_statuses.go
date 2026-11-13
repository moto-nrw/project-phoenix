package postgres

import (
	"context"
	"fmt"
	"time"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
)

// letterCurrentAddressSQL is where another mail of a delivery row would go.
const letterCurrentAddressSQL = `COALESCE((SELECT current_contact.address FROM platform.delivery_guardian_contacts AS current_contact
		WHERE current_contact.tenant_id = delivery.tenant_id AND current_contact.source = 'profile'
			AND current_contact.guardian_profile_id = delivery.guardian_profile_id), LOWER(TRIM(delivery.recipient_email)))`

// refusedElsewhereSQL is a mail someone unsubscribed from or complained about, at an
// address the person no longer has. The refusal stays with that address, so the mail
// counts as failed for the corrected one and never for the same one.
const refusedElsewhereSQL = `(outbox.status = 'sent' AND outbox.delivery_state IN ('unsubscribed', 'complained')
		AND COALESCE(LOWER(TRIM(outbox.recipient->>'address')), '') <> COALESCE(` + letterCurrentAddressSQL + `, ''))`

// emailDeliveryStatusesSQL reads a mail's result with the address it went to and,
// apart from it, the person's current address, where another mail would go.
const emailDeliveryStatusesSQL = `
		SELECT delivery.id AS delivery_id, delivery.guardian_profile_id, delivery.account_id,
			CASE WHEN contact.guardian_profile_id IS NOT NULL THEN contact.address ELSE LOWER(TRIM(delivery.recipient_email)) END AS recipient_email,
			LOWER(TRIM(outbox.recipient->>'address')) AS mailed_to,
			delivery.reachability,
			CASE
				WHEN delivery.outbox_id IS NULL THEN 'not_sent'
				WHEN outbox.status = 'sent' AND outbox.delivery_state IN ('failed', 'suppressed') THEN 'failed'
				WHEN ` + refusedElsewhereSQL + ` THEN 'failed'
				WHEN outbox.status = 'sent' THEN 'sent'
				WHEN outbox.status = 'dead_letter' OR ` + heldMailSQL + ` THEN 'failed'
				WHEN outbox.status = 'cancelled' THEN 'cancelled'
				ELSE 'pending'
			END AS email_status,
			outbox.last_error, outbox.sent_at, COALESCE(outbox.attempts, 0) AS attempts,
			outbox.status AS outbox_status, outbox.delivery_state, outbox.delivery_state_at,
			suppression.reason AS address_mark, contact.emails_stopped_at IS NOT NULL AS stopped,
			contact.emails_stopped_at IS NOT NULL AND EXISTS (
				SELECT 1 FROM platform.delivery_guardian_contacts AS other
				WHERE other.tenant_id = delivery.tenant_id AND other.address = contact.address
					AND other.emails_stopped_at IS NULL) AS stop_shared,
			EXISTS (
				SELECT 1 FROM platform.email_outbox AS earlier
				WHERE earlier.tenant_id = delivery.tenant_id
					AND earlier.related_entity_type = delivery.related_entity_type
					AND earlier.related_entity_id = delivery.related_entity_id
					AND (delivery.outbox_id IS NULL OR earlier.id <> delivery.outbox_id)
					AND earlier.status = 'sent' AND earlier.delivery_state IN ('unsubscribed', 'complained')
					AND LOWER(TRIM(earlier.recipient->>'address')) = ` + letterCurrentAddressSQL + `) AS current_address_refused
		FROM platform.delivery_email_deliveries AS delivery
		LEFT JOIN platform.email_outbox AS outbox ON outbox.id = delivery.outbox_id
		LEFT JOIN platform.delivery_guardian_contacts AS contact
			ON contact.tenant_id = delivery.tenant_id AND contact.source = 'profile'
			AND contact.guardian_profile_id = delivery.guardian_profile_id
		LEFT JOIN platform.email_suppressions AS suppression
			ON suppression.tenant_id = delivery.tenant_id
			AND suppression.address = CASE WHEN contact.guardian_profile_id IS NOT NULL THEN contact.address ELSE LOWER(TRIM(delivery.recipient_email)) END
		WHERE delivery.tenant_id = ? AND delivery.related_entity_type = ?
			AND delivery.related_entity_id = ?
		ORDER BY delivery.id`

type emailDeliveryStatusRow struct {
	DeliveryID        int64      `bun:"delivery_id"`
	GuardianProfileID *int64     `bun:"guardian_profile_id"`
	AccountID         *int64     `bun:"account_id"`
	RecipientEmail    *string    `bun:"recipient_email"`
	Reachability      string     `bun:"reachability"`
	EmailStatus       string     `bun:"email_status"`
	LastError         *string    `bun:"last_error"`
	SentAt            *time.Time `bun:"sent_at"`
	Attempts          int        `bun:"attempts"`
	OutboxStatus      *string    `bun:"outbox_status"`
	DeliveryState     *string    `bun:"delivery_state"`
	DeliveryStateAt   *time.Time `bun:"delivery_state_at"`
	AddressMark       *string    `bun:"address_mark"`
	Stopped           bool       `bun:"stopped"`
	StopShared        bool       `bun:"stop_shared"`
	MailedTo          *string    `bun:"mailed_to"`
	AddressRefused    bool       `bun:"current_address_refused"`
}

// heldMailSQL is a cancelled mail that stayed back for a hold, whether its own event
// arrived or not. Any other cancelled mail was cancelled on purpose and stays so:
// the status does not call it failed and a claim does not take it.
const heldMailSQL = `(outbox.status = 'cancelled' AND (outbox.delivery_state = 'suppressed' OR outbox.last_error IN (?, ?)))`

func (s *Store) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return false, err
	}
	result, err := db.NewRaw(`
		UPDATE platform.delivery_email_deliveries AS delivery
		SET outbox_id = NULL, updated_at = NOW()
		FROM platform.email_outbox AS outbox
		WHERE delivery.tenant_id = ? AND delivery.id = ?
			AND delivery.outbox_id = outbox.id
			AND outbox.tenant_id = delivery.tenant_id
			AND (outbox.status = 'dead_letter'
				OR `+heldMailSQL+`
				OR (outbox.status = 'sent' AND outbox.delivery_state IN ('failed', 'suppressed'))
				OR `+refusedElsewhereSQL+`
				OR (outbox.status = 'sent' AND outbox.delivery_state = 'delayed' AND outbox.delivery_state_at < ?))`,
		tenantID, deliveryID, deliveryContract.MailHeldByList, deliveryContract.MailHeldByStop,
		time.Now().Add(-deliveryContract.EmailDelayLimit)).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("delivery postgres: claim failed email delivery: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delivery postgres: count failed email delivery claim: %w", err)
	}
	return count == 1, nil
}

func (s *Store) EmailDeliveryStatuses(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]deliveryContract.EmailDeliveryStatus, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var rows []emailDeliveryStatusRow
	if err := db.NewRaw(emailDeliveryStatusesSQL, deliveryContract.MailHeldByList, deliveryContract.MailHeldByStop,
		tenantID, relatedType, relatedID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("delivery postgres: list email delivery statuses: %w", err)
	}
	statuses := make([]deliveryContract.EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		status := deliveryContract.EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, MailedTo: row.MailedTo,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		}
		status.Stopped, status.StopShared, status.CurrentAddressRefused = row.Stopped, row.StopShared, row.AddressRefused
		if row.AddressMark != nil {
			status.AddressMark = deliveryContract.EmailAddressMark(*row.AddressMark)
		}
		if row.OutboxStatus != nil {
			status.Outcome = deliveryContract.EmailOutcomeAt(deliveryContract.EmailOutcomeOf(*row.OutboxStatus, row.DeliveryState, row.LastError, status.AddressMark), row.DeliveryStateAt, time.Now())
			// The wait has ended: the status says a new mail can go out (#5934).
			if status.Outcome == deliveryContract.EmailOverdue {
				status.EmailStatus = "failed"
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}
