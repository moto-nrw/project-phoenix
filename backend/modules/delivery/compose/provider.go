package compose

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
)

// Worker transports render mail and re-check push subscriptions (#4013).

// NewDurableEmails binds the e-mail outbox's store to module.
func NewDurableEmails(module delivery.Capability) DurableEmails {
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

func (p *MailPushProvider) PreparePushBatch(ctx context.Context, intents []deliveryContract.ClaimedIntent) context.Context {
	return (*emailoutbox.MailPushProvider)(p).PreparePushBatch(ctx, intents)
}

// NewPushAuthorizationChecker re-checks the subscription and audience.
func NewPushAuthorizationChecker(tenants IntentTenantTransactions, subscriptions notifications.PushSubscriptionAuthorizer) func(context.Context, deliveryContract.ClaimedIntent) (bool, error) {
	return notifications.NewPushAuthorizationChecker(func(ctx context.Context, id int64, fn func(context.Context) error) error {
		return tenants.WithinTenant(ctx, id, fn)
	}, subscriptions, applicationBindings.ContainsPushSubscription)
}

// NewExpiredPushSubscriptionCleaner retires only an unchanged expired subscription.
func NewExpiredPushSubscriptionCleaner(tenants IntentTenantTransactions, subscriptions applicationBindings.PushSubscriptionCleaner) func(context.Context, deliveryContract.ClaimedIntent) error {
	return notifications.NewExpiredPushSubscriptionCleaner(func(ctx context.Context, id int64, fn func(context.Context) error) error {
		return tenants.WithinTenant(ctx, id, fn)
	}, subscriptions)
}
