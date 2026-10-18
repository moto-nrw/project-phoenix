package notifications

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
)

// PushSubscriptionAuthorizer reads the currently eligible subscriptions.
type PushSubscriptionAuthorizer interface {
	FindForTenantStaff(context.Context) ([]*deliveryModels.PushSubscription, error)
	FindForTenantAdmins(context.Context) ([]*deliveryModels.PushSubscription, error)
	FindForGuardians(context.Context, []int64, []int64) ([]*deliveryModels.PushSubscription, error)
	FindForStaffAccounts(context.Context, []int64) ([]*deliveryModels.PushSubscription, error)
	FindForSchoolAccounts(context.Context, []int64) ([]*deliveryModels.PushSubscription, error)
}

// EligiblePushSubscriptions selects the owner read for the original audience.
func EligiblePushSubscriptions(ctx context.Context, subscriptions PushSubscriptionAuthorizer, recipient deliveryContract.PushRecipient) ([]*deliveryModels.PushSubscription, error) {
	switch recipient.AudienceScope {
	case string(ScopeTenant):
		return subscriptions.FindForTenantStaff(ctx)
	case string(ScopeAdmin):
		return subscriptions.FindForTenantAdmins(ctx)
	case string(ScopeGuardian):
		return subscriptions.FindForGuardians(ctx, []int64{recipient.AccountID}, recipient.StudentIDs)
	case string(ScopeStaff):
		if recipient.Portal == PortalSchool {
			return subscriptions.FindForSchoolAccounts(ctx, []int64{recipient.AccountID})
		}
		return subscriptions.FindForStaffAccounts(ctx, []int64{recipient.AccountID})
	default:
		return nil, nil
	}
}
