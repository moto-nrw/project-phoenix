package api

import (
	"context"
	"errors"

	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

func bindParentPushSubscriptions(source interface {
	ParentDevices() deliveryModule.PushSubscriptions
}) parentAPI.PushSubscriptions {
	if source == nil {
		return nil
	}
	return parentPushSubscriptions{source.ParentDevices()}
}

type parentPushSubscriptions struct {
	source deliveryModule.PushSubscriptions
}

func (s parentPushSubscriptions) PublicKey() (string, error) {
	key, err := s.source.PublicKey()
	return key, parentDeliveryError(err)
}
func (s parentPushSubscriptions) SubscribeParent(ctx context.Context, accountID int64, input parentAPI.PushSubscriptionInput) error {
	return parentDeliveryError(s.source.SubscribeParent(ctx, accountID, deliveryModule.PushSubscriptionInput(input)))
}
func (s parentPushSubscriptions) UnsubscribeParent(ctx context.Context, accountID int64, endpoint string) error {
	return s.source.UnsubscribeParent(ctx, accountID, endpoint)
}

func bindParentPreferences(source interface {
	ParentPreferences() deliveryModule.NotificationPreferences
}) parentAPI.NotificationPreferences {
	if source == nil {
		return nil
	}
	return parentPreferences{source.ParentPreferences()}
}

type parentPreferences struct {
	source deliveryModule.NotificationPreferences
}

func (s parentPreferences) GetForParent(ctx context.Context, accountID int64) (*parentAPI.PreferenceOverview, error) {
	row, err := s.source.GetForParent(ctx, accountID)
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
	return parentDeliveryError(s.source.SetForParent(ctx, accountID, key, enabled))
}
func (s parentPreferences) DisableAllForParent(ctx context.Context, accountID int64) error {
	return s.source.DisableAllForParent(ctx, accountID)
}

// Keep diagnostics and the original cause while adding the consumer's identity.
func parentDeliveryError(err error) error {
	for _, pair := range [][2]error{
		{deliveryModule.ErrWebPushNotConfigured, parentAPI.ErrWebPushNotConfigured},
		{deliveryModule.ErrInvalidPushSubscription, parentAPI.ErrInvalidPushSubscription},
		{deliveryModule.ErrUnknownNotificationType, parentAPI.ErrUnknownNotificationType},
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

func bindParentLocale(source interface {
	ParentPreferences() deliveryModule.NotificationPreferences
}) func(string) bool {
	if source == nil {
		return nil
	}
	return source.ParentPreferences().SupportedLocale
}
