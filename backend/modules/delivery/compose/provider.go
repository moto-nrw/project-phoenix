package compose

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
)

// The transports behind the Delivery worker: the e-mail provider renders a
// claimed intent through the template registry and sends it, the push
// provider re-checks the subscription before it sends (#4013).

// NewDurableEmails binds the e-mail outbox's store to module.
func NewDurableEmails(module *delivery.Module) DurableEmails {
	return DurableEmails{DurableEmails: emailoutbox.DurableEmails{Module: module}}
}

type MailPushProvider emailoutbox.MailPushProvider
type DurableEmails struct {
	emailoutbox.DurableEmails
}
type IntentTenantTransactions interface {
	emailoutbox.IntentTenantTransactions
}

func (p *MailPushProvider) SendEmail(ctx context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return (*emailoutbox.MailPushProvider)(p).SendEmail(ctx, intent)
}
func (p *MailPushProvider) SendPush(ctx context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return (*emailoutbox.MailPushProvider)(p).SendPush(ctx, intent)
}

// NewPushAuthorizationChecker answers whether the claimed push still goes to
// a subscription its audience may receive.
func NewPushAuthorizationChecker(tenants IntentTenantTransactions, subscriptions notifications.PushSubscriptionAuthorizer) func(context.Context, deliveryContract.ClaimedIntent) (bool, error) {
	return notifications.NewPushAuthorizationChecker(func(ctx context.Context, id int64, fn func(context.Context) error) error {
		return tenants.WithinTenant(ctx, id, fn)
	}, subscriptions, applicationBindings.ContainsPushSubscription)
}

// NewExpiredPushSubscriptionCleaner removes the subscription a push service
// reported gone, unless it changed since the intent was claimed.
func NewExpiredPushSubscriptionCleaner(tenants IntentTenantTransactions, subscriptions applicationBindings.PushSubscriptionCleaner) func(context.Context, deliveryContract.ClaimedIntent) error {
	return notifications.NewExpiredPushSubscriptionCleaner(func(ctx context.Context, id int64, fn func(context.Context) error) error {
		return tenants.WithinTenant(ctx, id, fn)
	}, subscriptions)
}
