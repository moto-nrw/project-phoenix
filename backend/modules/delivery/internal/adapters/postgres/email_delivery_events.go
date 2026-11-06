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
// not both read the old state. Identities that name two mails are a contradiction:
// the school of the event would be a guess, so such an event matches nothing.
const emailEventMailSQL = `SELECT id, tenant_id, recipient->>'address' AS address, delivery_state, security, fallback_requested_at
	FROM platform.email_outbox
	WHERE delivery_token = NULLIF(?, '')
		OR message_id = NULLIF(?, '')
		OR provider_message_id = NULLIF(?, '')
	ORDER BY id LIMIT 2 FOR UPDATE`

const emailEventInsertSQL = `INSERT INTO platform.email_delivery_events
		(tenant_id, outbox_id, provider, provider_event_id, state, occurred_at, diagnostic)
	VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''))
	ON CONFLICT (outbox_id, provider, provider_event_id) DO NOTHING`

const emailEventStateSQL = `UPDATE platform.email_outbox
	SET delivery_state = ?, delivery_state_at = ?, updated_at = ?
	WHERE id = ?`

// The fallback's outcome is unknown, so the provider's state of the mail ends
// here. It gets attempts of its own: those spent on the provider say nothing about
// it. A mail still in the worker's hands stays there: finalizing it puts it back
// in the queue, and does the same with its attempts.
const emailEventRerouteSQL = `UPDATE platform.email_outbox
	SET fallback_requested_at = ?0, delivery_state = NULL, delivery_state_at = NULL,
		status = CASE WHEN status = 'sent' THEN 'pending' ELSE status END,
		next_retry_at = CASE WHEN status = 'sent' THEN ?0 ELSE next_retry_at END,
		attempts = CASE WHEN status = 'sent' THEN 0 ELSE attempts END,
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
		var mails []emailEventMailRow
		err := db.NewRaw(emailEventMailSQL, event.Token, event.MessageID, event.ProviderMessageID).Scan(txCtx, &mails)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if len(mails) != 1 {
			return nil
		}
		mail := mails[0]
		result.Matched = true
		if mail.Address != nil && *mail.Address != "" {
			// The server may echo the mailbox in a form the general filter misses.
			event.Diagnostic = strings.ReplaceAll(event.Diagnostic, *mail.Address, "[address]")
		}
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
	if mail.Fallback != nil && event.Provider != "moto" {
		// The mail left over the fallback: the event stays in the history, and
		// the original provider no longer speaks for it. Local SMTP rejections
		// still describe the fallback's outcome.
		if event.State == domain.EmailDeliveryComplained {
			return suppressEmailEventAddress(ctx, db, mail, event)
		}
		return nil
	}
	// What arrived or drew a complaint is not sent again. Every other state leaves
	// the outcome for the recipient open: a bounce the reconciliation read without
	// its reason can still turn out as a blocklist. fallback_requested_at makes
	// this happen once per mail.
	if event.Reroute && mail.Fallback == nil && mail.Security && result.State != domain.EmailDeliveryDelivered && result.State != domain.EmailDeliveryComplained {
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
	return suppressEmailEventAddress(ctx, db, mail, event)
}

func suppressEmailEventAddress(ctx context.Context, db bun.IDB, mail emailEventMailRow, event domain.EmailDeliveryEvent) error {
	if !event.State.BlocksAddress() || event.KeepsAddress || mail.TenantID == nil || mail.Address == nil || *mail.Address == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, emailSuppressionUpsertSQL, *mail.TenantID, normalizedAddress(*mail.Address), string(event.State.SuppressionReason()))
	return err
}

// A stop holds a mail back only if every person who holds the address carries it,
// so a shared address still reaches the person nobody stopped.
const (
	schoolEmailHoldsSQL = `SELECT
		EXISTS (SELECT 1 FROM platform.email_suppressions WHERE tenant_id = ?0 AND address = ?1) AS listed,
		EXISTS (SELECT 1 FROM platform.delivery_guardian_contacts WHERE tenant_id = ?0 AND address = ?1 AND emails_stopped_at IS NOT NULL)
		AND NOT EXISTS (SELECT 1 FROM platform.delivery_guardian_contacts WHERE tenant_id = ?0 AND address = ?1 AND emails_stopped_at IS NULL) AS stopped`
	traegerEmailHoldsSQL = `SELECT
		EXISTS (SELECT 1 FROM platform.email_suppressions AS list JOIN platform.delivery_traeger_schools AS school ON school.tenant_id = list.tenant_id
			WHERE school.organization_id = ?0 AND list.address = ?1) AS listed,
		EXISTS (SELECT 1 FROM platform.delivery_guardian_contacts AS contact JOIN platform.delivery_traeger_schools AS school ON school.tenant_id = contact.tenant_id
			WHERE school.organization_id = ?0 AND contact.address = ?1 AND contact.emails_stopped_at IS NOT NULL)
		AND NOT EXISTS (SELECT 1 FROM platform.delivery_guardian_contacts AS contact JOIN platform.delivery_traeger_schools AS school ON school.tenant_id = contact.tenant_id
			WHERE school.organization_id = ?0 AND contact.address = ?1 AND contact.emails_stopped_at IS NULL) AS stopped`
)

type emailHoldsRow struct {
	Listed  bool `bun:"listed"`
	Stopped bool `bun:"stopped"`
}

// SchoolEmailHolds implements ports.EmailDeliveryEventStore.
func (s *Store) SchoolEmailHolds(ctx context.Context, tenantID int64, address string) (bool, bool, error) {
	var row emailHoldsRow
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(schoolEmailHoldsSQL, tenantID, normalizedAddress(address)).Scan(txCtx, &row)
	})
	if err != nil {
		return false, false, fmt.Errorf("delivery postgres: read the holds of a school's mail: %w", err)
	}
	return row.Listed, row.Stopped, nil
}

// TraegerEmailHolds implements ports.EmailDeliveryEventStore.
func (s *Store) TraegerEmailHolds(ctx context.Context, organizationID int64, address string) (bool, bool, error) {
	var row emailHoldsRow
	err := s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(traegerEmailHoldsSQL, organizationID, normalizedAddress(address)).Scan(txCtx, &row)
	})
	if err != nil {
		return false, false, fmt.Errorf("delivery postgres: read the holds of a Träger's mail: %w", err)
	}
	return row.Listed, row.Stopped, nil
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

// DeleteUnheldEmailSuppressions implements ports.RetentionStore. An address stays
// marked until someone at the school checked it, however old the mark is (#5934).
// An address nobody at the school holds any more goes with the first run: this
// covers every path that deletes an account or a guardian, without each of them
// knowing the list. A new bounce would put it back.
func (s *Store) DeleteUnheldEmailSuppressions(ctx context.Context, tenantID int64) (int, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete unheld email suppressions")(db.ExecContext(ctx, `DELETE FROM platform.email_suppressions AS suppression
		WHERE suppression.tenant_id = ?0 AND NOT EXISTS (
			SELECT 1 FROM platform.delivery_known_email_addresses AS known
			WHERE known.tenant_id = ?0 AND known.address = suppression.address)`, tenantID))
}

const openProviderMailsSQL = `SELECT id, provider_message_id, delivery_state, delivery_state_at, sent_at, delivery_checked_at
	FROM platform.email_outbox
	WHERE status = 'sent' AND provider_message_id IS NOT NULL AND fallback_requested_at IS NULL
		AND (delivery_state IS NULL OR delivery_state = 'delayed')
		AND sent_at >= ? AND sent_at < ?
		AND (delivery_checked_at IS NULL OR delivery_checked_at < ?)
	ORDER BY delivery_checked_at NULLS FIRST, id LIMIT ?`

type providerMailRow struct {
	ID                int64      `bun:"id"`
	ProviderMessageID string     `bun:"provider_message_id"`
	DeliveryState     *string    `bun:"delivery_state"`
	DeliveryStateAt   *time.Time `bun:"delivery_state_at"`
	SentAt            time.Time  `bun:"sent_at"`
	CheckedAt         *time.Time `bun:"delivery_checked_at"`
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
		mail := domain.ProviderMail{ID: row.ID, ProviderMessageID: row.ProviderMessageID, StateAt: row.DeliveryStateAt, SentAt: row.SentAt, CheckedAt: row.CheckedAt}
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

// EmailSuppressionReason implements ports.EmailReconciliationStore. It runs in
// the caller's tenant transaction.
func (s *Store) EmailSuppressionReason(ctx context.Context, tenantID int64, address string) (domain.EmailSuppressionReason, bool, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return "", false, err
	}
	var reason string
	err = db.NewRaw(`SELECT reason FROM platform.email_suppressions WHERE tenant_id = ? AND address = ?`,
		tenantID, normalizedAddress(address)).Scan(ctx, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("delivery postgres: read email suppression reason: %w", err)
	}
	return domain.EmailSuppressionReason(reason), true, nil
}

// DeleteEmailSuppression implements ports.EmailReconciliationStore. It runs in
// the caller's tenant transaction. The DELETE
// decides by its own condition: it waits for a complaint committing at this moment
// and then rereads the row. A complaint may also have replaced the bounce during
// the provider call, so a row left behind is reported.
func (s *Store) DeleteEmailSuppression(ctx context.Context, tenantID int64, address string) error {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM platform.email_suppressions WHERE tenant_id = ? AND address = ? AND reason <> 'complained'`,
		tenantID, normalizedAddress(address)); err != nil {
		return fmt.Errorf("delivery postgres: delete email suppression: %w", err)
	}
	reason, listed, err := s.EmailSuppressionReason(ctx, tenantID, address)
	if err != nil {
		return err
	}
	if listed && reason == domain.EmailSuppressionComplained {
		return domain.ErrSuppressionNeedsReview
	}
	return nil
}

func normalizedAddress(address string) string { return strings.ToLower(strings.TrimSpace(address)) }
