// Package delivery is the public durable Delivery capability.
package delivery

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

// Capability enqueues, cancels and reports durable mail and push intents and
// keeps the per-recipient e-mail delivery log. Every write joins the caller's
// tenant transaction (or, for system mail, its administrative transaction).
type Capability interface {
	EnqueueEmail(context.Context, deliveryContract.EmailIntent) (deliveryContract.Enqueued, error)
	// EnqueueSystemEmail persists a global identity mail in the caller's admin
	// transaction. Global mail is not assigned to a placeholder school.
	EnqueueSystemEmail(context.Context, deliveryContract.EmailIntent) (deliveryContract.Enqueued, error)
	EnqueuePush(context.Context, deliveryContract.PushIntent) (deliveryContract.Enqueued, error)
	EnqueueDeferredPush(context.Context, deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error)
	Cancel(context.Context, int64, deliveryContract.Transport, deliveryContract.RelatedEntity, string) (int64, error)
	Statuses(context.Context, int64, deliveryContract.Transport, deliveryContract.RelatedEntity) ([]deliveryContract.Status, error)
	ReplaceEmailDeliveries(context.Context, int64, deliveryContract.RelatedEntity, []deliveryContract.EmailDelivery) error
	DeleteEmailDeliveries(context.Context, int64, deliveryContract.RelatedEntity) (int64, error)
	EmailDeliveryStatuses(context.Context, int64, deliveryContract.RelatedEntity) ([]deliveryContract.EmailDeliveryStatus, error)
	AttachEmailOutbox(context.Context, int64, int64, int64) error
	ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error)
}
