package notifications

import (
	"context"
	"errors"

	parentAPI "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
)

// ParentDevices exposes the existing parent operations through Delivery's public contract.
func (s *pushSubscriptionService) ParentDevices() parentAPI.PushSubscriptions {
	return parentPushSubscriptions{s}
}

type parentPushSubscriptions struct {
	PushSubscriptionService
}

func (s parentPushSubscriptions) PublicKey() (string, error) {
	key, err := s.PushSubscriptionService.PublicKey()
	return key, parentDeliveryError(err)
}
func (s parentPushSubscriptions) SubscribeParent(ctx context.Context, accountID int64, input parentAPI.PushSubscriptionInput) error {
	return parentDeliveryError(s.PushSubscriptionService.SubscribeParent(ctx, accountID, PushSubscriptionInput(input)))
}

// ParentPreferences exposes cross-school consent through Delivery's public contract.
func (s *preferenceService) ParentPreferences() parentAPI.NotificationPreferences {
	return parentPreferences{s}
}

type parentPreferences struct {
	PreferenceService
}

func (s parentPreferences) GetForParent(ctx context.Context, accountID int64) (*parentAPI.PreferenceOverview, error) {
	row, err := s.PreferenceService.GetForParent(ctx, accountID)
	if row == nil {
		return nil, err
	}
	out := &parentAPI.PreferenceOverview{TenantEnabled: row.TenantEnabled}
	if row.Types != nil {
		out.Types = make([]parentAPI.PreferenceState, len(row.Types))
		for i, item := range row.Types {
			out.Types[i] = parentAPI.PreferenceState(item)
		}
	}
	return out, err
}
func (s parentPreferences) SetForParent(ctx context.Context, accountID int64, key string, enabled bool) error {
	return parentDeliveryError(s.PreferenceService.SetForParent(ctx, accountID, key, enabled))
}

// Keep diagnostics and the original cause while adding the consumer's identity.
func parentDeliveryError(err error) error {
	for _, pair := range [][2]error{
		{ErrWebPushNotConfigured, parentAPI.ErrWebPushNotConfigured},
		{ErrInvalidPushSubscription, parentAPI.ErrInvalidPushSubscription},
		{ErrUnknownNotificationType, parentAPI.ErrUnknownNotificationType},
	} {
		if errors.Is(err, pair[0]) {
			return parentDeliveryFailure{error: err, kind: pair[1]}
		}
	}
	return err
}

type parentDeliveryFailure struct {
	error
	kind error
}

func (e parentDeliveryFailure) Is(target error) bool { return target == e.kind }
func (e parentDeliveryFailure) Unwrap() error        { return e.error }

// SupportedLocale uses the same language catalog as the notification copy.
func (*preferenceService) SupportedLocale(locale string) bool { return IsSupportedLocale(locale) }
