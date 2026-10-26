package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

// The mails of a Träger (ADR 0110) as the Träger reads them back (#4670):
// in its own Träger transaction under the Träger policy of the outbox.

// traegerEmailStatusesSQL reads the state of each mail without recipient
// and payload: a letter mail carries its PDF there, and the state is read
// again while mails wait.
const traegerEmailStatusesSQL = `SELECT id, tenant_id, organization_id, kind, idempotency_key, related_entity_type,
		related_entity_id, status, attempts, next_retry_at, provider_result, last_error, sent_at, dead_letter_at,
		cancelled_at, created_at, updated_at
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
