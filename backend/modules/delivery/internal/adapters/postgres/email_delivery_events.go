package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/uptrace/bun"
)

// The mail is locked while its event is recorded: two events of one mail must
// not both read the old state.
const emailEventMailSQL = `SELECT id, tenant_id, recipient->>'address' AS address, delivery_state, security, fallback_requested_at
	FROM platform.email_outbox
	WHERE delivery_token = NULLIF(?, '')
		OR message_id = NULLIF(?, '')
		OR provider_message_id = NULLIF(?, '')
	ORDER BY id LIMIT 1 FOR UPDATE`

const emailEventInsertSQL = `INSERT INTO platform.email_delivery_events
		(tenant_id, outbox_id, provider, provider_event_id, state, occurred_at, diagnostic)
	VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''))
	ON CONFLICT (outbox_id, provider, provider_event_id) DO NOTHING`

const emailEventStateSQL = `UPDATE platform.email_outbox
	SET delivery_state = ?, delivery_state_at = ?, updated_at = ?
	WHERE id = ?`

// The fallback's outcome is unknown, so the provider's state of the mail ends
// here. A mail still in the worker's hands stays there: finalizing it puts it
// back in the queue.
const emailEventRerouteSQL = `UPDATE platform.email_outbox
	SET fallback_requested_at = ?0, delivery_state = NULL, delivery_state_at = NULL,
		status = CASE WHEN status = 'sent' THEN 'pending' ELSE status END,
		next_retry_at = CASE WHEN status = 'sent' THEN ?0 ELSE next_retry_at END,
		updated_at = ?0
	WHERE id = ?1`

// A complaint is the stronger reason and stays once it is recorded.
const emailSuppressionUpsertSQL = `INSERT INTO platform.email_suppressions (tenant_id, address, reason)
	VALUES (?, ?, ?)
	ON CONFLICT (tenant_id, address) DO UPDATE SET
		reason = CASE WHEN platform.email_suppressions.reason = 'complained' THEN 'complained' ELSE EXCLUDED.reason END,
		updated_at = NOW()`

type emailEventMailRow struct {
	ID            int64      `bun:"id"`
	TenantID      *int64     `bun:"tenant_id"`
	Address       *string    `bun:"address"`
	DeliveryState *string    `bun:"delivery_state"`
	Security      bool       `bun:"security"`
	Fallback      *time.Time `bun:"fallback_requested_at"`
}

// RecordEmailDeliveryEvent implements ports.EmailDeliveryEventStore.
func (s *Store) RecordEmailDeliveryEvent(ctx context.Context, event domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error) {
	var result domain.EmailDeliveryEventResult
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		var mail emailEventMailRow
		err := db.NewRaw(emailEventMailSQL, event.Token, event.MessageID, event.ProviderMessageID).Scan(txCtx, &mail)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		result.Matched = true
		result.State = emailEventCurrentState(mail)
		inserted, err := db.ExecContext(txCtx, emailEventInsertSQL, mail.TenantID, mail.ID, event.Provider, event.ProviderEventID,
			string(event.State), event.OccurredAt, event.Diagnostic)
		if err != nil {
			return err
		}
		rows, err := inserted.RowsAffected()
		if err != nil || rows == 0 {
			return err
		}
		result.Recorded = true
		return applyEmailEvent(txCtx, db, mail, event, &result)
	})
	if err != nil {
		return domain.EmailDeliveryEventResult{}, fmt.Errorf("delivery postgres: record email delivery event: %w", err)
	}
	return result, nil
}

func emailEventCurrentState(mail emailEventMailRow) domain.EmailDeliveryState {
	if mail.DeliveryState == nil {
		return ""
	}
	return domain.EmailDeliveryState(*mail.DeliveryState)
}

func applyEmailEvent(ctx context.Context, db bun.IDB, mail emailEventMailRow, event domain.EmailDeliveryEvent, result *domain.EmailDeliveryEventResult) error {
	if mail.Fallback != nil {
		// The mail left over the fallback: the event stays in the history, and
		// the provider no longer speaks for it.
		return nil
	}
	if event.Reroute && mail.Security {
		if _, err := db.ExecContext(ctx, emailEventRerouteSQL, time.Now(), mail.ID); err != nil {
			return err
		}
		result.State, result.Rerouted = "", true
		return nil
	}
	next := domain.NextEmailDeliveryState(result.State, event.State)
	if next != result.State {
		if _, err := db.ExecContext(ctx, emailEventStateSQL, string(next), event.OccurredAt, time.Now(), mail.ID); err != nil {
			return err
		}
		result.State = next
	}
	if !event.State.BlocksAddress() || event.KeepsAddress || mail.TenantID == nil || mail.Address == nil || *mail.Address == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, emailSuppressionUpsertSQL, *mail.TenantID, strings.ToLower(*mail.Address), string(event.State))
	return err
}

// EmailAddressSuppressed implements ports.EmailDeliveryEventStore.
func (s *Store) EmailAddressSuppressed(ctx context.Context, tenantID int64, address string) (bool, error) {
	var suppressed bool
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.email_suppressions WHERE tenant_id = ? AND address = ?)`,
			tenantID, strings.ToLower(strings.TrimSpace(address))).Scan(txCtx, &suppressed)
	})
	if err != nil {
		return false, fmt.Errorf("delivery postgres: read email suppression: %w", err)
	}
	return suppressed, nil
}

// DeleteExpiredEmailDeliveryEvents implements ports.RetentionStore.
func (s *Store) DeleteExpiredEmailDeliveryEvents(ctx context.Context, tenantID int64, before time.Time) (int, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete expired email delivery events")(db.ExecContext(ctx, `DELETE FROM platform.email_delivery_events
		WHERE tenant_id = ?0 AND outbox_id IN (
			SELECT id FROM platform.email_outbox WHERE tenant_id = ?0 AND `+intentEndedBefore+`)`, tenantID, before))
}

// DeleteExpiredEmailSuppressions implements ports.RetentionStore.
func (s *Store) DeleteExpiredEmailSuppressions(ctx context.Context, tenantID int64, before time.Time) (int, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete expired email suppressions")(db.ExecContext(ctx, `DELETE FROM platform.email_suppressions
		WHERE tenant_id = ? AND updated_at < ?`, tenantID, before))
}

const openProviderMailsSQL = `SELECT id, provider_message_id, delivery_state, sent_at
	FROM platform.email_outbox
	WHERE status = 'sent' AND provider_message_id IS NOT NULL AND fallback_requested_at IS NULL
		AND (delivery_state IS NULL OR delivery_state = 'delayed')
		AND sent_at >= ? AND sent_at < ?
		AND (delivery_checked_at IS NULL OR delivery_checked_at < ?)
	ORDER BY delivery_checked_at NULLS FIRST, id LIMIT ?`

type providerMailRow struct {
	ID                int64     `bun:"id"`
	ProviderMessageID string    `bun:"provider_message_id"`
	DeliveryState     *string   `bun:"delivery_state"`
	SentAt            time.Time `bun:"sent_at"`
}

// OpenProviderMails implements ports.EmailReconciliationStore.
func (s *Store) OpenProviderMails(ctx context.Context, query domain.ProviderMailQuery) ([]domain.ProviderMail, error) {
	var rows []providerMailRow
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(openProviderMailsSQL, query.SentAfter, query.SentBefore, query.CheckedBefore, query.Limit).Scan(txCtx, &rows)
	})
	if err != nil {
		return nil, fmt.Errorf("delivery postgres: read open provider mails: %w", err)
	}
	mails := make([]domain.ProviderMail, 0, len(rows))
	for _, row := range rows {
		mail := domain.ProviderMail{ID: row.ID, ProviderMessageID: row.ProviderMessageID, SentAt: row.SentAt}
		if row.DeliveryState != nil {
			mail.State = domain.EmailDeliveryState(*row.DeliveryState)
		}
		mails = append(mails, mail)
	}
	return mails, nil
}

// MarkEmailDeliveryChecked implements ports.EmailReconciliationStore.
func (s *Store) MarkEmailDeliveryChecked(ctx context.Context, id int64, at time.Time) error {
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		_, err := db.ExecContext(txCtx, `UPDATE platform.email_outbox SET delivery_checked_at = ? WHERE id = ?`, at, id)
		return err
	})
	if err != nil {
		return fmt.Errorf("delivery postgres: mark email delivery checked: %w", err)
	}
	return nil
}

// EmailSuppressionReason implements ports.EmailReconciliationStore.
func (s *Store) EmailSuppressionReason(ctx context.Context, tenantID int64, address string) (domain.EmailDeliveryState, bool, error) {
	var reason string
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(`SELECT reason FROM platform.email_suppressions WHERE tenant_id = ? AND address = ?`,
			tenantID, normalizedAddress(address)).Scan(txCtx, &reason)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("delivery postgres: read email suppression reason: %w", err)
	}
	return domain.EmailDeliveryState(reason), true, nil
}

// DeleteEmailSuppression implements ports.EmailReconciliationStore.
func (s *Store) DeleteEmailSuppression(ctx context.Context, tenantID int64, address string) error {
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		_, err := db.ExecContext(txCtx, `DELETE FROM platform.email_suppressions WHERE tenant_id = ? AND address = ?`,
			tenantID, normalizedAddress(address))
		return err
	})
	if err != nil {
		return fmt.Errorf("delivery postgres: delete email suppression: %w", err)
	}
	return nil
}

func normalizedAddress(address string) string { return strings.ToLower(strings.TrimSpace(address)) }
