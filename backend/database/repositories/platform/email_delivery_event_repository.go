package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const deliveryEventTableExprAlias = `platform.email_delivery_events AS "email_delivery_event"`

// EmailDeliveryEventRepository implements platform.EmailDeliveryEventCleanupRepository.
type EmailDeliveryEventRepository struct {
	db *bun.DB
}

// NewEmailDeliveryEventRepository wires a new repository.
func NewEmailDeliveryEventRepository(db *bun.DB) platform.EmailDeliveryEventCleanupRepository {
	return &EmailDeliveryEventRepository{db: db}
}

// CreateIfNew inserts the event, reporting false when the provider already
// delivered this event id. The unique index on (provider, provider_event_id)
// is what makes webhook replays a no-op rather than a duplicated state change.
// Runs under phoenix_admin — the tenant comes from the resolved outbox row,
// never from the unauthenticated request.
func (r *EmailDeliveryEventRepository) CreateIfNew(ctx context.Context, event *platform.EmailDeliveryEvent) (bool, error) {
	if err := event.Validate(); err != nil {
		return false, fmt.Errorf("validation failed: %w", err)
	}

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(deliveryEventTableExprAlias).
		On("CONFLICT (provider, provider_event_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create delivery event: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// ListByOutboxIDs returns the events for the given outbox rows, newest first.
// Tenant-scoped through RLS; phoenix_tenant holds SELECT and nothing else.
func (r *EmailDeliveryEventRepository) ListByOutboxIDs(ctx context.Context, outboxIDs []int64) ([]*platform.EmailDeliveryEvent, error) {
	if len(outboxIDs) == 0 {
		return nil, nil
	}
	var rows []*platform.EmailDeliveryEvent
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(deliveryEventTableExprAlias).
		Where(`"email_delivery_event".outbox_id IN (?)`, bun.List(outboxIDs)).
		OrderExpr(`"email_delivery_event".occurred_at DESC, "email_delivery_event".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list delivery events: %w", err)
	}
	return rows, nil
}

// DeleteOlderThan removes one tenant's events received before cutoff. The
// retention sweep runs as phoenix_admin because phoenix_tenant has SELECT-only
// access, so tenant_id must be an explicit predicate rather than relying on RLS.
func (r *EmailDeliveryEventRepository) DeleteOlderThan(ctx context.Context, tenantID int64, cutoff time.Time) (int64, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.EmailDeliveryEvent)(nil)).
		ModelTableExpr(deliveryEventTableExprAlias).
		Where(`"email_delivery_event".tenant_id = ?`, tenantID).
		Where(`"email_delivery_event".received_at < ?`, cutoff).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired delivery events: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}
