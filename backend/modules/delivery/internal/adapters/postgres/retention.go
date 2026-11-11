package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/uptrace/bun"
)

// The retention statements of one school (#3955). An intent ends when it is
// sent, finally failed or cancelled; rows from before the leased outbox
// (1.15.360) may lack the timestamp of their state, so their last update
// stands in.
// intentEndedBefore matches an intent of the queue that left it before ?1.
const intentEndedBefore = `status IN ('sent', 'dead_letter', 'cancelled')
	AND COALESCE(CASE status WHEN 'sent' THEN sent_at WHEN 'dead_letter' THEN dead_letter_at ELSE cancelled_at END, updated_at) < ?1`

const (
	deleteExpiredEmailDeliveriesSQL = `DELETE FROM platform.delivery_email_deliveries
		WHERE tenant_id = ?0 AND (
			(outbox_id IS NULL AND created_at < ?1)
			OR outbox_id IN (
				SELECT id FROM platform.email_outbox
				WHERE tenant_id = ?0 AND ` + intentEndedBefore + `))`
	deleteExpiredEmailIntentsSQL = `DELETE FROM platform.email_outbox
		WHERE tenant_id = ?0 AND ` + intentEndedBefore + `
			AND kind NOT IN (?2)`
	clearExpiredKeyOnlyEmailIntentsSQL = `UPDATE platform.email_outbox
		SET related_entity_type = NULL, related_entity_id = NULL,
			recipient = '{}'::jsonb, payload = '{}'::jsonb, provider_result = NULL, last_error = NULL,
			message_id = NULL, provider_message_id = NULL, delivery_state = NULL, delivery_state_at = NULL,
			fallback_requested_at = NULL, delivery_checked_at = NULL,
			delivery_token = uuid_generate_v4()::text
		WHERE tenant_id = ?0 AND ` + intentEndedBefore + `
			AND kind IN (?2)
			AND (related_entity_type IS NOT NULL OR related_entity_id IS NOT NULL
				OR recipient <> '{}'::jsonb OR payload <> '{}'::jsonb
				OR provider_result IS NOT NULL OR last_error IS NOT NULL
				OR message_id IS NOT NULL OR provider_message_id IS NOT NULL OR delivery_state IS NOT NULL)`
	deleteExpiredPushIntentsSQL = `DELETE FROM platform.push_outbox
		WHERE tenant_id = ?0 AND ` + intentEndedBefore + ``
)

func (s *Store) DeleteExpiredEmailDeliveryLogs(ctx context.Context, tenantID int64, before time.Time) (int, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete expired email delivery logs")(db.ExecContext(ctx, deleteExpiredEmailDeliveriesSQL, tenantID, before))
}

func (s *Store) DeleteExpiredEmailIntents(ctx context.Context, tenantID int64, before time.Time, keyOnlyTemplates []string) (int, error) {
	if len(keyOnlyTemplates) == 0 {
		return 0, errors.New("delivery postgres: key-only templates are required")
	}
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete expired email intents")(db.ExecContext(ctx, deleteExpiredEmailIntentsSQL, tenantID, before, bun.List(keyOnlyTemplates)))
}

func (s *Store) ClearExpiredKeyOnlyEmailIntents(ctx context.Context, tenantID int64, before time.Time, keyOnlyTemplates []string) (int, error) {
	if len(keyOnlyTemplates) == 0 {
		return 0, errors.New("delivery postgres: key-only templates are required")
	}
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("clear expired key-only email intents")(db.ExecContext(ctx, clearExpiredKeyOnlyEmailIntentsSQL, tenantID, before, bun.List(keyOnlyTemplates)))
}

func (s *Store) DeleteExpiredPushIntents(ctx context.Context, tenantID int64, before time.Time) (int, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete expired push intents")(db.ExecContext(ctx, deleteExpiredPushIntentsSQL, tenantID, before))
}

// affected reports the rows a retention statement touched.
func affected(operation string) func(sql.Result, error) (int, error) {
	return func(result sql.Result, err error) (int, error) {
		if err != nil {
			return 0, fmt.Errorf("delivery postgres: %s: %w", operation, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("delivery postgres: %s: %w", operation, err)
		}
		return int(rows), nil
	}
}

// DeleteExpiredSystemEmails applies the same retention window to global
// identity mail and to the mails and pushes of a Träger (ADR 0110, ADR
// 0126), which cannot be visited by the tenant-scoped nightly sweep.
func (s *Store) DeleteExpiredSystemEmails(ctx context.Context, before time.Time, limit int) (count int, err error) {
	err = s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		emails, deleteErr := affected("delete expired system email intents")(db.ExecContext(txCtx, `DELETE FROM platform.email_outbox
   WHERE id IN (SELECT id FROM platform.email_outbox
    WHERE tenant_id IS NULL AND status IN ('sent', 'dead_letter', 'cancelled')
    AND NOT (kind = 'holiday_closure_notice' AND (status IN ('sent', 'dead_letter')
     OR (status = 'cancelled' AND last_error IN (?2, ?3))))
    AND COALESCE(CASE status WHEN 'sent' THEN sent_at WHEN 'dead_letter' THEN dead_letter_at ELSE cancelled_at END, updated_at) < ?0
    ORDER BY id LIMIT ?1)`, before, limit, deliveryContract.MailHeldByList, deliveryContract.MailHeldByStop))
		if deleteErr != nil {
			return deleteErr
		}
		// Failed and unknown closure mail remains recoverable. Accepted mail
		// keeps its key and outcome so a delayed reconciliation never retries it;
		// only registration links remain for the existing erasure contract.
		cleared, clearErr := affected("clear accepted holiday closure mail")(db.ExecContext(txCtx, `WITH expired AS MATERIALIZED (
    SELECT id FROM platform.email_outbox
    WHERE tenant_id IS NULL AND kind = 'holiday_closure_notice' AND status = 'sent' AND sent_at < ?
     AND (recipient <> '{}'::jsonb OR payload - 'registration_ids' <> '{}'::jsonb)
    ORDER BY id LIMIT ?
   ), removed_events AS (
    DELETE FROM platform.email_delivery_events WHERE outbox_id IN (SELECT id FROM expired)
   )
   UPDATE platform.email_outbox
   SET recipient = '{}'::jsonb, payload = jsonb_build_object('registration_ids', payload->'registration_ids'),
    provider_result = NULL, last_error = NULL, message_id = NULL, provider_message_id = NULL,
    delivery_state = NULL, delivery_state_at = NULL, fallback_requested_at = NULL, delivery_checked_at = NULL,
    delivery_token = uuid_generate_v4()::text
   WHERE id IN (SELECT id FROM expired)`, before, limit))
		if clearErr != nil {
			return clearErr
		}
		pushes, deleteErr := affected("delete expired Träger push intents")(db.ExecContext(txCtx, `DELETE FROM platform.push_outbox
   WHERE id IN (SELECT id FROM platform.push_outbox
    WHERE tenant_id IS NULL AND status IN ('sent', 'dead_letter', 'cancelled')
    AND COALESCE(CASE status WHEN 'sent' THEN sent_at WHEN 'dead_letter' THEN dead_letter_at ELSE cancelled_at END, updated_at) < ?
    ORDER BY id LIMIT ?)`, before, limit))
		count = emails + cleared + pushes
		return deleteErr
	})
	return count, err
}
