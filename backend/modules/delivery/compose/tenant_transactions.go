package compose

import (
	"context"
	"fmt"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewTenantTransactions binds provider steps to the intent's explicit scope.
func NewTenantTransactions(db *bun.DB) delivery.IntentTransactions {
	return delivery.NewIntentTransactions(func(ctx context.Context, id int64, fn func(context.Context) error) error {
		return tenant.WithTenantTx(ctx, db, id, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
	}, tenant.WithinAdmin)
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
	return a.module.Cancel(ctx, tenantID.Int64(), deliveryContract.TransportPush, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

// EnqueuePush stores a push intent and reports whether it was a duplicate.
func (a DurablePushOutbox) EnqueuePush(ctx context.Context, input notifications.PushIntent) (bool, error) {
	stored, err := a.module.EnqueuePush(ctx, deliveryContract.PushIntent{
		TenantID: input.TenantID, Template: input.Template,
		Recipient: deliveryContract.PushRecipient{
			SubscriptionID: input.SubscriptionID, AccountID: input.AccountID, Endpoint: input.Endpoint, P256DH: input.P256DH,
			Auth: input.Auth, Portal: input.Portal, UpdatedAt: input.UpdatedAt,
			AudienceScope: string(input.AudienceScope), StudentIDs: input.StudentIDs,
		},
		Payload: deliveryContract.PushPayload{
			Title: input.Title, Body: input.Body, DeepLink: input.DeepLink, Type: input.Type, Priority: input.Priority,
		},
		IdempotencyKey: input.IdempotencyKey,
		Related:        deliveryContract.RelatedEntity{Type: input.RelatedType, ID: input.RelatedID},
	})
	return stored.Duplicate, err
}
