package application

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
)

type PushSubscriptionCleaner interface {
	DeleteExpiredIfUnchanged(context.Context, *deliveryModels.PushSubscription) (bool, error)
}

func ContainsPushSubscription(subscriptions []*deliveryModels.PushSubscription, recipient deliveryContract.PushRecipient) bool {
	for _, subscription := range subscriptions {
		if subscription.ID == recipient.SubscriptionID && subscription.AccountID == recipient.AccountID &&
			subscription.Endpoint == recipient.Endpoint && subscription.P256dh == recipient.P256DH &&
			subscription.Auth == recipient.Auth && subscription.Portal == recipient.Portal &&
			subscription.UpdatedAt.Equal(recipient.UpdatedAt) {
			return true
		}
	}
	return false
}
