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
const emailEventMailSQL = `SELECT id, tenant_id, recipient->>'address' AS address, delivery_state
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

// A complaint is the stronger reason and stays once it is recorded.
const emailSuppressionUpsertSQL = `INSERT INTO platform.email_suppressions (tenant_id, address, reason)
	VALUES (?, ?, ?)
	ON CONFLICT (tenant_id, address) DO UPDATE SET
		reason = CASE WHEN platform.email_suppressions.reason = 'complained' THEN 'complained' ELSE EXCLUDED.reason END,
		updated_at = NOW()`

type emailEventMailRow struct {
	ID            int64   `bun:"id"`
	TenantID      *int64  `bun:"tenant_id"`
	Address       *string `bun:"address"`
	DeliveryState *string `bun:"delivery_state"`
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
	next := domain.NextEmailDeliveryState(result.State, event.State)
	if next != result.State {
		if _, err := db.ExecContext(ctx, emailEventStateSQL, string(next), event.OccurredAt, time.Now(), mail.ID); err != nil {
			return err
		}
		result.State = next
	}
	if !event.State.BlocksAddress() || mail.TenantID == nil || mail.Address == nil || *mail.Address == "" {
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
