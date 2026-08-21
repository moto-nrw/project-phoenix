package platform

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const deliveryTableExprAlias = `platform.email_delivery AS "email_delivery"`

// EmailDeliveryRepository implements platform.EmailDeliveryRepository with bun.
type EmailDeliveryRepository struct {
	db *bun.DB
}

// NewEmailDeliveryRepository wires a new repository.
func NewEmailDeliveryRepository(db *bun.DB) platform.EmailDeliveryRepository {
	return &EmailDeliveryRepository{db: db}
}

// ReplaceForEntity swaps an entity's whole recipient set. Delete-then-insert
// rather than upsert: publishing resolves the audience live, so a guardian who
// was unlinked since the previous attempt must disappear from the matrix
// instead of lingering with a stale status. Both statements run in the caller's
// tenant transaction, so the swap is atomic.
func (r *EmailDeliveryRepository) ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []*platform.EmailDelivery) error {
	if _, err := r.DeleteForEntity(ctx, tenantID, relatedType, relatedID); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		row.TenantID = tenantID
		row.RelatedEntityType = relatedType
		row.RelatedEntityID = relatedID
		if row.Reachability == "" {
			row.Reachability = platform.ReachabilityOK
		}
	}
	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&rows).
		ModelTableExpr(deliveryTableExprAlias).
		Returning("id").
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert email delivery rows: %w", err)
	}
	return nil
}

// DeleteForEntity removes every delivery row of an entity and reports how many
// were dropped.
func (r *EmailDeliveryRepository) DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.EmailDelivery)(nil)).
		ModelTableExpr(deliveryTableExprAlias).
		Where(`"email_delivery".tenant_id = ?`, tenantID).
		Where(`"email_delivery".related_entity_type = ?`, relatedType).
		Where(`"email_delivery".related_entity_id = ?`, relatedID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete email delivery rows: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil //nolint:nilerr // RowsAffected is advisory here; the delete succeeded
	}
	return n, nil
}

// AttachOutbox links a delivery row to the outbox row queued for it.
func (r *EmailDeliveryRepository) AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	if _, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailDelivery)(nil)).
		ModelTableExpr(deliveryTableExprAlias).
		Set("outbox_id = ?", outboxID).
		Set("updated_at = NOW()").
		Where(`"email_delivery".tenant_id = ?`, tenantID).
		Where(`"email_delivery".id = ?`, deliveryID).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to attach outbox row to delivery: %w", err)
	}
	return nil
}

// ListForEntity returns the recipients joined with the outbox state of their
// mail.
//
// The CASE is the honest-status rule of #2384: a row we handed to the mail
// server reports "sent", never "delivered" — without provider webhooks (#1937)
// we do not know whether it reached the mailbox, and the UI must not show a
// green tick for a letter that bounced afterwards. A row with no outbox link
// reports why nothing was queued instead of pretending a send happened.
func (r *EmailDeliveryRepository) ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]*platform.EmailDeliveryStatus, error) {
	var rows []*platform.EmailDeliveryStatus
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT d.id AS delivery_id,
			d.guardian_profile_id,
			d.account_id,
			COALESCE(gp.first_name, '') AS first_name,
			COALESCE(gp.last_name, '')  AS last_name,
			d.recipient_email,
			d.reachability,
			CASE
				-- No outbox row means no mail was queued. The REASON lives in
				-- reachability; leaking it into email_status would collapse the two
				-- columns this view exists to keep apart.
				WHEN d.outbox_id IS NULL THEN 'not_sent'
				WHEN o.status = 'sent'   THEN 'sent'
				WHEN o.status = 'failed' THEN 'failed'
				ELSE 'pending'
			END AS email_status,
			o.last_error,
			o.sent_at,
			COALESCE(o.attempts, 0) AS attempts
		FROM platform.email_delivery d
		LEFT JOIN platform.email_outbox o ON o.id = d.outbox_id
		LEFT JOIN users.guardian_profiles gp ON gp.id = d.guardian_profile_id
		WHERE d.tenant_id = ?
			AND d.related_entity_type = ?
			AND d.related_entity_id = ?
		ORDER BY LOWER(COALESCE(gp.last_name, '')), LOWER(COALESCE(gp.first_name, '')), d.id`,
		tenantID, relatedType, relatedID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to list email deliveries: %w", err)
	}
	return rows, nil
}
