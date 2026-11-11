package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
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
const traegerEmailStatusesByKeySQL = `SELECT candidate.id, candidate.tenant_id, candidate.organization_id, candidate.kind,
 candidate.idempotency_key, candidate.related_entity_type, candidate.related_entity_id,
 CASE WHEN candidate.status = 'pending' AND blocker.id IS NOT NULL THEN 'dead_letter' ELSE candidate.status END AS status,
 candidate.attempts, candidate.next_retry_at, candidate.provider_result,
 CASE WHEN candidate.status = 'pending' AND blocker.id IS NOT NULL THEN blocker.last_error ELSE candidate.last_error END AS last_error,
 candidate.sent_at, candidate.dead_letter_at, candidate.cancelled_at, candidate.created_at, candidate.updated_at,
 candidate.delivery_state, candidate.delivery_state_at
 FROM platform.email_outbox AS candidate
 LEFT JOIN LATERAL (
  SELECT predecessor.id, predecessor.last_error FROM platform.email_outbox AS predecessor
  WHERE candidate.kind = 'holiday_closure_notice' AND candidate.status = 'pending'
   AND predecessor.tenant_id IS NULL AND predecessor.organization_id = candidate.organization_id
   AND predecessor.kind = candidate.kind
   AND (predecessor.status = 'dead_letter' OR ` + holidayClosureRecoverableCancellationSQL + `)
   AND lower(trim(predecessor.recipient->>'address')) = lower(trim(candidate.recipient->>'address'))
   AND predecessor.id < candidate.id
  ORDER BY (predecessor.last_error = '` + dispatchOutcomeUnknown + `' ) DESC NULLS LAST, predecessor.id LIMIT 1
 ) AS blocker ON true
 WHERE candidate.tenant_id IS NULL AND candidate.organization_id = ? AND candidate.idempotency_key = ANY(?::text[])
 ORDER BY candidate.created_at, candidate.id`

// retryFailedTraegerEmailsSQL makes a Träger's dead letters with a known
// failure due again (ADR 0196 section 5). A mail sent, waiting or whose
// worker stopped during the provider call stays as it is: its outcome is
// known good, pending or unknown, and a repetition would be blind.
const retryFailedTraegerEmailsSQL = `WITH requested AS MATERIALIZED (
 SELECT id, kind, recipient FROM platform.email_outbox
 WHERE tenant_id IS NULL AND organization_id = ?0 AND idempotency_key = ANY(?1::text[])
), recoverable AS (
 SELECT DISTINCT mail.id FROM platform.email_outbox AS mail
 JOIN requested ON mail.id = requested.id OR (
  requested.kind = 'holiday_closure_notice' AND mail.kind = requested.kind AND mail.id < requested.id
  AND lower(trim(mail.recipient->>'address')) = lower(trim(requested.recipient->>'address')))
 WHERE mail.tenant_id IS NULL AND mail.organization_id = ?0
  AND ((mail.status = 'dead_letter' AND mail.last_error IS DISTINCT FROM ?2)
   OR (mail.status = 'cancelled' AND mail.last_error IN (?3, ?4)))
)
UPDATE platform.email_outbox AS mail
 SET status = 'pending', next_retry_at = NOW(), attempts = 0, last_error = NULL, dead_letter_at = NULL, cancelled_at = NULL,
  lease_token = NULL, lease_expires_at = NULL, updated_at = NOW()
 WHERE mail.tenant_id IS NULL AND mail.organization_id = ?0 AND mail.id IN (SELECT id FROM recoverable)`

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
	result, err := db.NewRaw(retryFailedTraegerEmailsSQL, organizationID, pgdialect.Array(keys), dispatchOutcomeUnknown, deliveryContract.MailHeldByList, deliveryContract.MailHeldByStop).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delivery postgres: retry failed Träger emails: %w", err)
	}
	return result.RowsAffected()
}
