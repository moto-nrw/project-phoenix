package notifications

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
)

func NewPushAuthorizationChecker(execute func(context.Context, int64, func(context.Context) error) error, subscriptions PushSubscriptionAuthorizer, contains func([]*deliveryModels.PushSubscription, deliveryContract.PushRecipient) bool) func(context.Context, deliveryContract.ClaimedIntent) (bool, error) {
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) (bool, error) {
		var eligible []*deliveryModels.PushSubscription
		err := execute(ctx, intent.TenantID, func(txCtx context.Context) error {
			var lookupErr error
			eligible, lookupErr = EligiblePushSubscriptions(txCtx, subscriptions, intent.PushRecipient)
			return lookupErr
		})
		if err != nil {
			return false, err
		}
		return contains(eligible, intent.PushRecipient), nil
	}
}
func NewExpiredPushSubscriptionCleaner(execute func(context.Context, int64, func(context.Context) error) error, subscriptions interface {
	DeleteExpiredIfUnchanged(context.Context, *deliveryModels.PushSubscription) (bool, error)
}) func(context.Context, deliveryContract.ClaimedIntent) error {
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
		return execute(ctx, intent.TenantID, func(txCtx context.Context) error {
			subscription := &deliveryModels.PushSubscription{
				AccountID: intent.PushRecipient.AccountID, Portal: intent.PushRecipient.Portal,
				Endpoint: intent.PushRecipient.Endpoint, P256dh: intent.PushRecipient.P256DH,
				Auth: intent.PushRecipient.Auth,
			}
			subscription.ID = intent.PushRecipient.SubscriptionID
			subscription.UpdatedAt = intent.PushRecipient.UpdatedAt
			_, err := subscriptions.DeleteExpiredIfUnchanged(txCtx, subscription)
			return err
		})
	}
}
