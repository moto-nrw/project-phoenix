package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

// The mails of a Träger (ADR 0110) as the Träger reads them back (#4670):
// in its own Träger transaction under the Träger policy of the outbox.

// traegerEmailStatusesSQL reads the state of each mail without recipient
// and payload: a letter mail carries its PDF there, and the state is read
// again while mails wait.
const traegerEmailStatusesSQL = `SELECT id, tenant_id, organization_id, kind, idempotency_key, related_entity_type,
		related_entity_id, status, attempts, next_retry_at, provider_result, last_error, sent_at, dead_letter_at,
		cancelled_at, created_at, updated_at, delivery_state, delivery_state_at
	FROM platform.email_outbox
	WHERE tenant_id IS NULL AND organization_id = ? AND related_entity_type = ? AND related_entity_id = ?
	ORDER BY created_at, id`

// TraegerEmailStatuses lists the mails of a Träger for one related entity
// in the caller's Träger transaction of that Träger (ADR 0110, #4670).
func (s *Store) TraegerEmailStatuses(ctx context.Context, organizationID int64, relatedType string, relatedID int64) ([]domain.Intent, error) {
	db, err := RequireTraegerDatabase(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []intentRow
	if err := db.NewRaw(traegerEmailStatusesSQL, organizationID, relatedType, relatedID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("delivery postgres: list Träger email statuses: %w", err)
	}
	return toDomainRows(domain.TransportEmail, rows), nil
}

// traegerEmailStatusesByKeySQL reads the mails of a Träger by idempotency
// key (ADR 0196), without recipient and payload.
const traegerEmailStatusesByKeySQL = `SELECT id, tenant_id, organization_id, kind, idempotency_key, related_entity_type,
		related_entity_id, status, attempts, next_retry_at, provider_result, last_error, sent_at, dead_letter_at,
		cancelled_at, created_at, updated_at, delivery_state, delivery_state_at
	FROM platform.email_outbox
	WHERE tenant_id IS NULL AND organization_id = ? AND idempotency_key = ANY(?::text[])
	ORDER BY created_at, id`

// retryFailedTraegerEmailsSQL makes a Träger's dead letters with a known
// failure due again (ADR 0196 section 5). A mail sent, waiting or whose
// worker stopped during the provider call stays as it is: its outcome is
// known good, pending or unknown, and a repetition would be blind.
const retryFailedTraegerEmailsSQL = `UPDATE platform.email_outbox
	SET status = 'pending', next_retry_at = NOW(), attempts = 0, last_error = NULL, dead_letter_at = NULL,
		lease_token = NULL, lease_expires_at = NULL, updated_at = NOW()
	WHERE tenant_id IS NULL AND organization_id = ? AND idempotency_key = ANY(?::text[])
		AND status = 'dead_letter' AND last_error IS DISTINCT FROM ?`

// TraegerEmailStatusesByKey lists the mails of a Träger by idempotency key
// in the caller's Träger transaction of that Träger.
func (s *Store) TraegerEmailStatusesByKey(ctx context.Context, organizationID int64, keys []string) ([]domain.Intent, error) {
	db, err := RequireTraegerDatabase(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []intentRow
	if err := db.NewRaw(traegerEmailStatusesByKeySQL, organizationID, pgdialect.Array(keys)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("delivery postgres: list Träger email statuses by key: %w", err)
	}
	return toDomainRows(domain.TransportEmail, rows), nil
}

// RetryFailedTraegerEmails makes the Träger's failed mails of keys due
// again in the caller's Träger transaction of that Träger.
func (s *Store) RetryFailedTraegerEmails(ctx context.Context, organizationID int64, keys []string) (int64, error) {
	db, err := RequireTraegerDatabase(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(retryFailedTraegerEmailsSQL, organizationID, pgdialect.Array(keys), dispatchOutcomeUnknown).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: retry failed Träger emails: %w", err)
	}
	return result.RowsAffected()
}
