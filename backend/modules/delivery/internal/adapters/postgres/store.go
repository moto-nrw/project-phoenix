package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/uptrace/bun"
)

type TenantDatabase func(context.Context, int64) (bun.IDB, error)

type AdminTransaction func(context.Context, func(context.Context, bun.IDB) error) error

type Store struct {
	tenantDB TenantDatabase
	adminTx  AdminTransaction
}

const emailEnqueueSQL = `
	WITH inserted AS (
		INSERT INTO platform.email_outbox (
			tenant_id, kind, idempotency_key, related_entity_type,
			related_entity_id, recipient, payload, status, next_retry_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)
		ON CONFLICT (tenant_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING id
	)
	SELECT id, FALSE AS duplicate, FALSE AS conflict FROM inserted
	UNION ALL
	SELECT id, TRUE AS duplicate,
		NOT (kind = ? AND recipient = ?::jsonb AND payload = ?::jsonb
		 AND COALESCE(related_entity_type, '') = COALESCE(?, '')
		 AND COALESCE(related_entity_id, 0) = COALESCE(?, 0)) AS conflict
	FROM platform.email_outbox
	WHERE ? IS NOT NULL AND tenant_id = ? AND idempotency_key = ?
		AND NOT EXISTS (SELECT 1 FROM inserted)
	LIMIT 1`

const pushEnqueueSQL = `
	WITH inserted AS (
		INSERT INTO platform.push_outbox (
			tenant_id, kind, idempotency_key, related_entity_type,
			related_entity_id, recipient, payload, status, next_retry_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)
		ON CONFLICT (tenant_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING id
	)
	SELECT id, FALSE AS duplicate, FALSE AS conflict FROM inserted
	UNION ALL
	SELECT id, TRUE AS duplicate,
		NOT (kind = ? AND recipient = ?::jsonb AND payload = ?::jsonb
		 AND COALESCE(related_entity_type, '') = COALESCE(?, '')
		 AND COALESCE(related_entity_id, 0) = COALESCE(?, 0)) AS conflict
	FROM platform.push_outbox
	WHERE ? IS NOT NULL AND tenant_id = ? AND idempotency_key = ?
		AND NOT EXISTS (SELECT 1 FROM inserted)
	LIMIT 1`

const emailExistingEnqueueSQL = `
	SELECT id, TRUE AS duplicate,
		NOT (kind = ? AND recipient = ?::jsonb AND payload = ?::jsonb
		 AND COALESCE(related_entity_type, '') = COALESCE(?, '')
		 AND COALESCE(related_entity_id, 0) = COALESCE(?, 0)) AS conflict
	FROM platform.email_outbox
	WHERE tenant_id = ? AND idempotency_key = ?
	LIMIT 1`

const pushExistingEnqueueSQL = `
	SELECT id, TRUE AS duplicate,
		NOT (kind = ? AND recipient = ?::jsonb AND payload = ?::jsonb
		 AND COALESCE(related_entity_type, '') = COALESCE(?, '')
		 AND COALESCE(related_entity_id, 0) = COALESCE(?, 0)) AS conflict
	FROM platform.push_outbox
	WHERE tenant_id = ? AND idempotency_key = ?
	LIMIT 1`

const emailClaimSQL = `
	UPDATE platform.email_outbox
	SET status = 'claimed', lease_token = uuid_generate_v4()::text,
		lease_expires_at = ?, updated_at = ?
	WHERE id IN (
		SELECT id FROM platform.email_outbox
		WHERE (status = 'pending' AND next_retry_at <= ?)
			OR (status = 'claimed' AND lease_expires_at <= ?)
		ORDER BY next_retry_at, id FOR UPDATE SKIP LOCKED LIMIT ?
	) RETURNING *`

const pushClaimSQL = `
	UPDATE platform.push_outbox
	SET status = 'claimed', lease_token = uuid_generate_v4()::text,
		lease_expires_at = ?, updated_at = ?
	WHERE id IN (
		SELECT id FROM platform.push_outbox
		WHERE (status = 'pending' AND next_retry_at <= ?)
			OR (status = 'claimed' AND lease_expires_at <= ?)
		ORDER BY next_retry_at, id FOR UPDATE SKIP LOCKED LIMIT ?
	) RETURNING *`

const emailRenewLeaseSQL = `UPDATE platform.email_outbox
	SET lease_expires_at = ?, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const pushRenewLeaseSQL = `UPDATE platform.push_outbox
	SET lease_expires_at = ?, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const emailFinalizeSentSQL = `UPDATE platform.email_outbox
	SET status = 'sent', provider_result = ?, sent_at = ?, last_error = NULL,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const pushFinalizeSentSQL = `UPDATE platform.push_outbox
	SET status = 'sent', provider_result = ?, sent_at = ?, last_error = NULL,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const emailFinalizeCancelledSQL = `UPDATE platform.email_outbox
	SET status = 'cancelled', last_error = ?, cancelled_at = ?,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const pushFinalizeCancelledSQL = `UPDATE platform.push_outbox
	SET status = 'cancelled', last_error = ?, cancelled_at = ?,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const emailFinalizeFailureSQL = `UPDATE platform.email_outbox
	SET status = ?, attempts = ?, last_error = ?, next_retry_at = ?,
		dead_letter_at = CASE WHEN ? = 'dead_letter' THEN ?::timestamptz ELSE NULL END,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const pushFinalizeFailureSQL = `UPDATE platform.push_outbox
	SET status = ?, attempts = ?, last_error = ?, next_retry_at = ?,
		dead_letter_at = CASE WHEN ? = 'dead_letter' THEN ?::timestamptz ELSE NULL END,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE id = ? AND status = 'claimed' AND lease_token = ?`

const emailCancelSQL = `UPDATE platform.email_outbox
	SET status = 'cancelled', last_error = ?, cancelled_at = ?,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE tenant_id = ? AND related_entity_type = ? AND related_entity_id = ?
	AND status = 'pending'`

const pushCancelSQL = `UPDATE platform.push_outbox
	SET status = 'cancelled', last_error = ?, cancelled_at = ?,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
	WHERE tenant_id = ? AND related_entity_type = ? AND related_entity_id = ?
	AND status = 'pending'`

const emailStatusesSQL = `SELECT * FROM platform.email_outbox
	WHERE tenant_id = ? AND related_entity_type = ? AND related_entity_id = ?
	ORDER BY created_at, id`

const pushStatusesSQL = `SELECT * FROM platform.push_outbox
	WHERE tenant_id = ? AND related_entity_type = ? AND related_entity_id = ?
	ORDER BY created_at, id`

type intentRow struct {
	bun.BaseModel     `bun:"-"`
	ID                int64           `bun:"id"`
	TenantID          int64           `bun:"tenant_id"`
	Kind              string          `bun:"kind"`
	IdempotencyKey    *string         `bun:"idempotency_key"`
	RelatedEntityType *string         `bun:"related_entity_type"`
	RelatedEntityID   *int64          `bun:"related_entity_id"`
	Recipient         json.RawMessage `bun:"recipient,type:jsonb"`
	Payload           json.RawMessage `bun:"payload,type:jsonb"`
	Status            string          `bun:"status"`
	Attempts          int             `bun:"attempts"`
	NextRetryAt       time.Time       `bun:"next_retry_at"`
	LeaseToken        *string         `bun:"lease_token"`
	LeaseExpiresAt    *time.Time      `bun:"lease_expires_at"`
	ProviderResult    json.RawMessage `bun:"provider_result,type:jsonb"`
	LastError         *string         `bun:"last_error"`
	SentAt            *time.Time      `bun:"sent_at"`
	DeadLetterAt      *time.Time      `bun:"dead_letter_at"`
	CancelledAt       *time.Time      `bun:"cancelled_at"`
	CreatedAt         time.Time       `bun:"created_at"`
	UpdatedAt         time.Time       `bun:"updated_at"`
}

type emailDeliveryRow struct {
	bun.BaseModel     `bun:"table:platform.delivery_email_deliveries"`
	ID                int64     `bun:"id,pk,autoincrement"`
	TenantID          int64     `bun:"tenant_id"`
	RelatedEntityType string    `bun:"related_entity_type"`
	RelatedEntityID   int64     `bun:"related_entity_id"`
	OutboxID          *int64    `bun:"outbox_id"`
	GuardianProfileID *int64    `bun:"guardian_profile_id"`
	AccountID         *int64    `bun:"account_id"`
	RecipientEmail    *string   `bun:"recipient_email"`
	Reachability      string    `bun:"reachability"`
	CreatedAt         time.Time `bun:"created_at"`
	UpdatedAt         time.Time `bun:"updated_at"`
}

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
}

func New(tenantDB TenantDatabase, adminTx AdminTransaction) *Store {
	if tenantDB == nil || adminTx == nil {
		panic("delivery postgres: tenant database and admin transaction are required")
	}
	return &Store{tenantDB: tenantDB, adminTx: adminTx}
}

func (s *Store) Enqueue(ctx context.Context, intent domain.Intent) (domain.Enqueued, error) {
	db, err := s.tenantDB(ctx, intent.TenantID)
	if err != nil {
		return domain.Enqueued{}, err
	}
	var result domain.Enqueued
	args := []any{
		intent.TenantID, intent.Template, intent.IdempotencyKey, intent.RelatedEntityType,
		intent.RelatedEntityID, intent.Recipient, intent.Payload, intent.NextRetryAt,
		intent.Template, intent.Recipient, intent.Payload, intent.RelatedEntityType, intent.RelatedEntityID,
		intent.IdempotencyKey, intent.TenantID, intent.IdempotencyKey,
	}
	switch intent.Transport {
	case domain.TransportEmail:
		err = db.NewRaw(emailEnqueueSQL, args...).Scan(ctx, &result)
	case domain.TransportPush:
		err = db.NewRaw(pushEnqueueSQL, args...).Scan(ctx, &result)
	default:
		return domain.Enqueued{}, unknownTransport(intent.Transport)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.Enqueued{}, fmt.Errorf("delivery postgres: enqueue %s: %w", intent.Transport, err)
	}
	if result.ID <= 0 && intent.IdempotencyKey != nil {
		existingArgs := []any{
			intent.Template, intent.Recipient, intent.Payload, intent.RelatedEntityType, intent.RelatedEntityID,
			intent.TenantID, intent.IdempotencyKey,
		}
		switch intent.Transport {
		case domain.TransportEmail:
			err = db.NewRaw(emailExistingEnqueueSQL, existingArgs...).Scan(ctx, &result)
		case domain.TransportPush:
			err = db.NewRaw(pushExistingEnqueueSQL, existingArgs...).Scan(ctx, &result)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Enqueued{}, fmt.Errorf("delivery postgres: read existing %s intent: %w", intent.Transport, err)
		}
	}
	if result.ID <= 0 {
		return domain.Enqueued{}, errors.New("delivery postgres: enqueue returned no intent")
	}
	if result.Conflict {
		return domain.Enqueued{}, fmt.Errorf("delivery postgres: %w", domain.ErrIdempotencyConflict)
	}
	return result, nil
}

func (s *Store) Claim(ctx context.Context, transport domain.Transport, limit int, now, leaseExpiresAt time.Time) ([]domain.Intent, error) {
	var rows []intentRow
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		switch transport {
		case domain.TransportEmail:
			return db.NewRaw(emailClaimSQL, leaseExpiresAt, now, now, now, limit).Scan(txCtx, &rows)
		case domain.TransportPush:
			return db.NewRaw(pushClaimSQL, leaseExpiresAt, now, now, now, limit).Scan(txCtx, &rows)
		default:
			return unknownTransport(transport)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("delivery postgres: claim %s: %w", transport, err)
	}
	return toDomainRows(transport, rows), nil
}

func (s *Store) RenewLease(ctx context.Context, transport domain.Transport, id int64, token string, leaseExpiresAt time.Time) (bool, error) {
	var renewed bool
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		var result interface{ RowsAffected() (int64, error) }
		var execErr error
		switch transport {
		case domain.TransportEmail:
			result, execErr = db.NewRaw(emailRenewLeaseSQL, leaseExpiresAt, time.Now(), id, token).Exec(txCtx)
		case domain.TransportPush:
			result, execErr = db.NewRaw(pushRenewLeaseSQL, leaseExpiresAt, time.Now(), id, token).Exec(txCtx)
		default:
			return unknownTransport(transport)
		}
		if execErr != nil {
			return execErr
		}
		rows, rowsErr := result.RowsAffected()
		renewed = rows == 1
		return rowsErr
	})
	if err != nil {
		return false, fmt.Errorf("delivery postgres: renew %s lease for intent %d: %w", transport, id, err)
	}
	return renewed, nil
}

func (s *Store) FinalizeSent(ctx context.Context, transport domain.Transport, id int64, token string, providerResult json.RawMessage, sentAt time.Time) (bool, error) {
	var finalized bool
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		var resultResult interface{ RowsAffected() (int64, error) }
		var execErr error
		switch transport {
		case domain.TransportEmail:
			resultResult, execErr = db.NewRaw(emailFinalizeSentSQL, providerResult, sentAt, sentAt, id, token).Exec(txCtx)
		case domain.TransportPush:
			resultResult, execErr = db.NewRaw(pushFinalizeSentSQL, providerResult, sentAt, sentAt, id, token).Exec(txCtx)
		default:
			return unknownTransport(transport)
		}
		if execErr != nil {
			return execErr
		}
		rows, rowsErr := resultResult.RowsAffected()
		finalized = rows == 1
		return rowsErr
	})
	if err != nil {
		return false, fmt.Errorf("delivery postgres: finalize sent %s intent %d: %w", transport, id, err)
	}
	return finalized, nil
}

func (s *Store) FinalizeCancelled(ctx context.Context, transport domain.Transport, id int64, token, reason string, cancelledAt time.Time) (bool, error) {
	var finalized bool
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		args := []any{reason, cancelledAt, cancelledAt, id, token}
		var result interface{ RowsAffected() (int64, error) }
		var execErr error
		switch transport {
		case domain.TransportEmail:
			result, execErr = db.NewRaw(emailFinalizeCancelledSQL, args...).Exec(txCtx)
		case domain.TransportPush:
			result, execErr = db.NewRaw(pushFinalizeCancelledSQL, args...).Exec(txCtx)
		default:
			return unknownTransport(transport)
		}
		if execErr != nil {
			return execErr
		}
		rows, rowsErr := result.RowsAffected()
		finalized = rows == 1
		return rowsErr
	})
	if err != nil {
		return false, fmt.Errorf("delivery postgres: finalize cancelled %s intent %d: %w", transport, id, err)
	}
	return finalized, nil
}

func (s *Store) FinalizeFailure(ctx context.Context, transport domain.Transport, id int64, token string, attempts int, lastError string, nextRetryAt time.Time, maxAttempts int) (domain.FinalizeResult, error) {
	state := "pending"
	if attempts >= maxAttempts {
		state = "dead_letter"
	}
	now := time.Now()
	var finalized bool
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		args := []any{state, attempts, lastError, nextRetryAt, state, now, now, id, token}
		var resultResult interface{ RowsAffected() (int64, error) }
		var execErr error
		switch transport {
		case domain.TransportEmail:
			resultResult, execErr = db.NewRaw(emailFinalizeFailureSQL, args...).Exec(txCtx)
		case domain.TransportPush:
			resultResult, execErr = db.NewRaw(pushFinalizeFailureSQL, args...).Exec(txCtx)
		default:
			return unknownTransport(transport)
		}
		if execErr != nil {
			return execErr
		}
		rows, rowsErr := resultResult.RowsAffected()
		finalized = rows == 1
		return rowsErr
	})
	if err != nil {
		return domain.FinalizeResult{}, fmt.Errorf("delivery postgres: finalize failed %s intent %d: %w", transport, id, err)
	}
	return domain.FinalizeResult{Finalized: finalized, State: state}, nil
}

func (s *Store) Cancel(ctx context.Context, tenantID int64, transport domain.Transport, relatedType string, relatedID int64, reason string, cancelledAt time.Time) (int64, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	args := []any{reason, cancelledAt, cancelledAt, tenantID, relatedType, relatedID}
	var resultResult interface{ RowsAffected() (int64, error) }
	switch transport {
	case domain.TransportEmail:
		resultResult, err = db.NewRaw(emailCancelSQL, args...).Exec(ctx)
	case domain.TransportPush:
		resultResult, err = db.NewRaw(pushCancelSQL, args...).Exec(ctx)
	default:
		return 0, unknownTransport(transport)
	}
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: cancel %s intents: %w", transport, err)
	}
	count, err := resultResult.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: count cancelled %s intents: %w", transport, err)
	}
	return count, nil
}

func (s *Store) Statuses(ctx context.Context, tenantID int64, transport domain.Transport, relatedType string, relatedID int64) ([]domain.Intent, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var rows []intentRow
	switch transport {
	case domain.TransportEmail:
		err = db.NewRaw(emailStatusesSQL, tenantID, relatedType, relatedID).Scan(ctx, &rows)
	case domain.TransportPush:
		err = db.NewRaw(pushStatusesSQL, tenantID, relatedType, relatedID).Scan(ctx, &rows)
	default:
		return nil, unknownTransport(transport)
	}
	if err != nil {
		return nil, fmt.Errorf("delivery postgres: list %s statuses: %w", transport, err)
	}
	return toDomainRows(transport, rows), nil
}

func (s *Store) Backlog(ctx context.Context) (int, error) {
	var count int
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(`
			SELECT
				(SELECT COUNT(*) FROM platform.email_outbox WHERE status IN ('pending', 'claimed'))
				+ (SELECT COUNT(*) FROM platform.push_outbox WHERE status IN ('pending', 'claimed'))
		`).Scan(txCtx, &count)
	})
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: count backlog: %w", err)
	}
	return count, nil
}

func (s *Store) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, error) {
	var oldest *time.Time
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		return db.NewRaw(`
			SELECT MIN(created_at) FROM (
				SELECT created_at FROM platform.email_outbox WHERE status IN ('pending', 'claimed')
				UNION ALL
				SELECT created_at FROM platform.push_outbox WHERE status IN ('pending', 'claimed')
			) AS pending`).Scan(txCtx, &oldest)
	})
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: find oldest pending intent: %w", err)
	}
	if oldest == nil || now.Before(*oldest) {
		return 0, nil
	}
	return now.Sub(*oldest), nil
}

func (s *Store) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []domain.EmailDelivery) error {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*emailDeliveryRow)(nil)).
		ModelTableExpr(`platform.delivery_email_deliveries AS "email_delivery"`).
		Where(`"email_delivery".tenant_id = ?`, tenantID).
		Where(`"email_delivery".related_entity_type = ?`, relatedType).
		Where(`"email_delivery".related_entity_id = ?`, relatedID).Exec(ctx); err != nil {
		return fmt.Errorf("delivery postgres: replace email deliveries delete: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	values := make([]emailDeliveryRow, 0, len(rows))
	for _, row := range rows {
		values = append(values, emailDeliveryRow{
			TenantID: tenantID, RelatedEntityType: relatedType, RelatedEntityID: relatedID,
			OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	if _, err := db.NewInsert().Model(&values).ExcludeColumn("created_at", "updated_at").
		ModelTableExpr(`platform.delivery_email_deliveries AS "email_delivery"`).Exec(ctx); err != nil {
		return fmt.Errorf("delivery postgres: replace email deliveries insert: %w", err)
	}
	return nil
}

func (s *Store) DeleteEmailDeliveries(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	result, err := db.NewDelete().Model((*emailDeliveryRow)(nil)).
		ModelTableExpr(`platform.delivery_email_deliveries AS "email_delivery"`).
		Where(`"email_delivery".tenant_id = ?`, tenantID).
		Where(`"email_delivery".related_entity_type = ?`, relatedType).
		Where(`"email_delivery".related_entity_id = ?`, relatedID).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: delete email deliveries: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: count deleted email deliveries: %w", err)
	}
	return count, nil
}

func (s *Store) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().Model((*emailDeliveryRow)(nil)).
		ModelTableExpr(`platform.delivery_email_deliveries AS "email_delivery"`).
		Set("outbox_id = ?", outboxID).Set("updated_at = NOW()").
		Where(`"email_delivery".tenant_id = ?`, tenantID).
		Where(`"email_delivery".id = ?`, deliveryID).
		Where(`EXISTS (SELECT 1 FROM platform.email_outbox AS outbox WHERE outbox.id = ? AND outbox.tenant_id = "email_delivery".tenant_id)`, outboxID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delivery postgres: attach email outbox: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delivery postgres: count attached email outbox: %w", err)
	}
	if count != 1 {
		return errors.New("delivery postgres: email delivery not found")
	}
	return nil
}

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
			AND outbox.status = 'dead_letter'`, tenantID, deliveryID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("delivery postgres: claim failed email delivery: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delivery postgres: count failed email delivery claim: %w", err)
	}
	return count == 1, nil
}

func (s *Store) EmailDeliveryStatuses(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]domain.EmailDeliveryStatus, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var rows []emailDeliveryStatusRow
	if err := db.NewRaw(`
		SELECT delivery.id AS delivery_id, delivery.guardian_profile_id, delivery.account_id,
			delivery.recipient_email, delivery.reachability,
			CASE
				WHEN delivery.outbox_id IS NULL THEN 'not_sent'
				WHEN outbox.status = 'sent' THEN 'sent'
				WHEN outbox.status IN ('dead_letter', 'cancelled') THEN 'failed'
				ELSE 'pending'
			END AS email_status,
			outbox.last_error, outbox.sent_at, COALESCE(outbox.attempts, 0) AS attempts
		FROM platform.delivery_email_deliveries AS delivery
		LEFT JOIN platform.email_outbox AS outbox ON outbox.id = delivery.outbox_id
		WHERE delivery.tenant_id = ? AND delivery.related_entity_type = ?
			AND delivery.related_entity_id = ?
		ORDER BY delivery.id`, tenantID, relatedType, relatedID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("delivery postgres: list email delivery statuses: %w", err)
	}
	statuses := make([]domain.EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, domain.EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail,
			Reachability:   row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return statuses, nil
}

func unknownTransport(transport domain.Transport) error {
	return fmt.Errorf("delivery postgres: unknown transport %q", transport)
}

func toDomainRows(transport domain.Transport, rows []intentRow) []domain.Intent {
	result := make([]domain.Intent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Intent{
			ID: row.ID, TenantID: row.TenantID, Transport: transport, Template: row.Kind,
			IdempotencyKey: row.IdempotencyKey, RelatedEntityType: row.RelatedEntityType,
			RelatedEntityID: row.RelatedEntityID, Recipient: row.Recipient, Payload: row.Payload,
			Status: row.Status, Attempts: row.Attempts, NextRetryAt: row.NextRetryAt,
			LeaseToken: row.LeaseToken, LeaseExpiresAt: row.LeaseExpiresAt,
			ProviderResult: row.ProviderResult, LastError: row.LastError, SentAt: row.SentAt,
			DeadLetterAt: row.DeadLetterAt, CancelledAt: row.CancelledAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return result
}
