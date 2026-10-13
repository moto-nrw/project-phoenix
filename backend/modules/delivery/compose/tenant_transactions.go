package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantTransactions runs the provider steps of a claimed intent — the
// template render, the push authorization and the expired-subscription
// cleanup — in the tenant transaction of the intent's school, which the
// worker's own context does not carry (#2747).
type TenantTransactions struct{ db *bun.DB }

// NewTenantTransactions binds the provider steps to the root database.
func NewTenantTransactions(db *bun.DB) TenantTransactions {
	return TenantTransactions{db: db}
}

// WithinTenant runs fn in the tenant transaction of tenantID.
func (t TenantTransactions) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, t.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// DurablePushOutbox serves the notification channels' durable push outbox
// over the Delivery module.
type DurablePushOutbox struct{ module *delivery.Module }

// NewDurablePushOutbox binds the push outbox to module.
func NewDurablePushOutbox(module *delivery.Module) DurablePushOutbox {
	return DurablePushOutbox{module: module}
}

// CancelPendingByRelatedEntity cancels the pending pushes of an entity of
// the school in context.
func (a DurablePushOutbox) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("cancel push outbox rows: tenant is required: %w", err)
	}
	return a.module.Cancel(ctx, tenantID.Int64(), delivery.TransportPush, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

// EnqueuePush stores a push intent and reports whether it was a duplicate.
func (a DurablePushOutbox) EnqueuePush(ctx context.Context, input notifications.PushIntent) (bool, error) {
	stored, err := a.module.EnqueuePush(ctx, delivery.PushIntent{
		TenantID: input.TenantID, Template: input.Template,
		Recipient: delivery.PushRecipient{
			SubscriptionID: input.SubscriptionID, AccountID: input.AccountID, Endpoint: input.Endpoint, P256DH: input.P256DH,
			Auth: input.Auth, Portal: input.Portal, UpdatedAt: input.UpdatedAt,
			AudienceScope: string(input.AudienceScope), StudentIDs: input.StudentIDs,
		},
		Payload: delivery.PushPayload{
			Title: input.Title, Body: input.Body, DeepLink: input.DeepLink, Type: input.Type, Priority: input.Priority,
		},
		IdempotencyKey: input.IdempotencyKey,
		Related:        delivery.RelatedEntity{Type: input.RelatedType, ID: input.RelatedID},
	})
	return stored.Duplicate, err
}
