package notifications

import (
	"context"
	"fmt"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// DurablePushOutbox serves the notification channels' durable push outbox
// over the Delivery module.
type DurablePushOutbox struct{ Module *delivery.Module }

// CancelPendingByRelatedEntity cancels the pending pushes of an entity of
// the school in context.
func (a DurablePushOutbox) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("cancel push outbox rows: tenant is required: %w", err)
	}
	return a.Module.Cancel(ctx, tenantID.Int64(), deliveryContract.TransportPush, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

// EnqueuePush stores a push intent and reports whether it was a duplicate.
func (a DurablePushOutbox) EnqueuePush(ctx context.Context, input PushIntent) (bool, error) {
	stored, err := a.Module.EnqueuePush(ctx, deliveryContract.PushIntent{
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
