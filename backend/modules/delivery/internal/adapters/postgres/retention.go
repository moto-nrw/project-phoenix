package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
			recipient = '{}'::jsonb, payload = '{}'::jsonb, provider_result = NULL, last_error = NULL
		WHERE tenant_id = ?0 AND ` + intentEndedBefore + `
			AND kind IN (?2)
			AND (related_entity_type IS NOT NULL OR related_entity_id IS NOT NULL
				OR recipient <> '{}'::jsonb OR payload <> '{}'::jsonb
				OR provider_result IS NOT NULL OR last_error IS NOT NULL)`
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
    AND COALESCE(CASE status WHEN 'sent' THEN sent_at WHEN 'dead_letter' THEN dead_letter_at ELSE cancelled_at END, updated_at) < ?
    ORDER BY id LIMIT ?)`, before, limit))
		if deleteErr != nil {
			return deleteErr
		}
		pushes, deleteErr := affected("delete expired Träger push intents")(db.ExecContext(txCtx, `DELETE FROM platform.push_outbox
   WHERE id IN (SELECT id FROM platform.push_outbox
    WHERE tenant_id IS NULL AND status IN ('sent', 'dead_letter', 'cancelled')
    AND COALESCE(CASE status WHEN 'sent' THEN sent_at WHEN 'dead_letter' THEN dead_letter_at ELSE cancelled_at END, updated_at) < ?
    ORDER BY id LIMIT ?)`, before, limit))
		count = emails + pushes
		return deleteErr
	})
	return count, err
}
