package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

// Push intents of a school, or deferred pushes of a Träger (ADR 0126), with
// their idempotency key per scope.

const pushEnqueueSQL = `
	WITH inserted AS (
		INSERT INTO platform.push_outbox (
			tenant_id, organization_id, kind, idempotency_key, related_entity_type,
			related_entity_id, recipient, payload, status, next_retry_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)
		ON CONFLICT (tenant_id, organization_id, idempotency_key)
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
	WHERE ? IS NOT NULL AND COALESCE(tenant_id, 0) = COALESCE(?, 0)
		AND COALESCE(organization_id, 0) = COALESCE(?, 0) AND idempotency_key = ?
		AND NOT EXISTS (SELECT 1 FROM inserted)
	LIMIT 1`

const pushExistingEnqueueSQL = `
	SELECT id, TRUE AS duplicate,
		NOT (kind = ? AND recipient = ?::jsonb AND payload = ?::jsonb
		 AND COALESCE(related_entity_type, '') = COALESCE(?, '')
		 AND COALESCE(related_entity_id, 0) = COALESCE(?, 0)) AS conflict
	FROM platform.push_outbox
	WHERE COALESCE(tenant_id, 0) = COALESCE(?, 0) AND COALESCE(organization_id, 0) = COALESCE(?, 0)
		AND idempotency_key = ?
	LIMIT 1`

// enqueuePushIntent inserts a push intent of a school or, with a Träger, a
// deferred push of that Träger (ADR 0126), or reads the one with its
// idempotency key.
func enqueuePushIntent(ctx context.Context, db bun.IDB, intent domain.Intent) (domain.Enqueued, error) {
	var scope, organization any
	if intent.TenantID != 0 {
		scope = intent.TenantID
	}
	if intent.OrganizationID > 0 {
		organization = intent.OrganizationID
	}
	var result domain.Enqueued
	err := db.NewRaw(pushEnqueueSQL,
		scope, organization, intent.Template, intent.IdempotencyKey, intent.RelatedEntityType,
		intent.RelatedEntityID, intent.Recipient, intent.Payload, intent.NextRetryAt,
		intent.Template, intent.Recipient, intent.Payload, intent.RelatedEntityType, intent.RelatedEntityID,
		intent.IdempotencyKey, scope, organization, intent.IdempotencyKey,
	).Scan(ctx, &result)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.Enqueued{}, fmt.Errorf("delivery postgres: enqueue %s: %w", intent.Transport, err)
	}
	if result.ID > 0 || intent.IdempotencyKey == nil {
		return result, nil
	}
	err = db.NewRaw(pushExistingEnqueueSQL,
		intent.Template, intent.Recipient, intent.Payload, intent.RelatedEntityType, intent.RelatedEntityID,
		scope, organization, intent.IdempotencyKey,
	).Scan(ctx, &result)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.Enqueued{}, fmt.Errorf("delivery postgres: read existing %s intent: %w", intent.Transport, err)
	}
	return result, nil
}
