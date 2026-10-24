package notifications

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
)

// StaffDevices exposes the existing staff/school operations without exposing
// the application contract or changing the errors those operations return.
func (s *pushSubscriptionService) StaffDevices() devices.StaffPushSubscriptions {
	return staffPushSubscriptions{source: s}
}

type staffPushSubscriptions struct{ source PushSubscriptionService }

func (s staffPushSubscriptions) PublicKey() (string, error) { return s.source.PublicKey() }
func (s staffPushSubscriptions) Subscribe(ctx context.Context, accountID int64, input devices.PushSubscriptionInput) error {
	return s.source.Subscribe(ctx, accountID, PushSubscriptionInput(input))
}
func (s staffPushSubscriptions) SubscribeSchool(ctx context.Context, accountID int64, input devices.PushSubscriptionInput) error {
	return s.source.SubscribeSchool(ctx, accountID, PushSubscriptionInput(input))
}
func (s staffPushSubscriptions) Unsubscribe(ctx context.Context, accountID int64, endpoint string) error {
	return s.source.Unsubscribe(ctx, accountID, endpoint)
}
func (s staffPushSubscriptions) UnsubscribeSchool(ctx context.Context, accountID int64, endpoint string) error {
	return s.source.UnsubscribeSchool(ctx, accountID, endpoint)
}
func (staffPushSubscriptions) IsNotConfigured(err error) bool {
	return errors.Is(err, ErrWebPushNotConfigured)
}
func (staffPushSubscriptions) IsInvalidSubscription(err error) bool {
	return errors.Is(err, ErrInvalidPushSubscription)
}

// StaffPreferences exposes the same account-scoped decisions and raw errors.
func (s *preferenceService) StaffPreferences() devices.StaffNotificationPreferences {
	return staffPreferences{source: s}
}

type staffPreferences struct{ source PreferenceService }

func (s staffPreferences) GetForAccount(ctx context.Context, accountID int64, portal string) (*devices.PreferenceOverview, error) {
	row, err := s.source.GetForAccount(ctx, accountID, portal)
	if err != nil || row == nil {
		return nil, err
	}
	out := &devices.PreferenceOverview{TenantEnabled: row.TenantEnabled}
	if row.Types != nil {
		out.Types = make([]devices.PreferenceState, len(row.Types))
		for i, value := range row.Types {
			out.Types[i] = devices.PreferenceState{Key: value.Key, Label: value.Label, Description: value.Description, Group: value.Group, Enabled: value.Enabled, Available: value.Available}
		}
	}
	return out, nil
}
func (s staffPreferences) SetForPortalAccount(ctx context.Context, accountID int64, portal, notificationType string, enabled bool) error {
	return s.source.SetForPortalAccount(ctx, accountID, portal, notificationType, enabled)
}
func (s staffPreferences) DisableAllForPortalAccount(ctx context.Context, accountID int64, portal string) error {
	return s.source.DisableAllForPortalAccount(ctx, accountID, portal)
}
func (s staffPreferences) EmailSubscribed(ctx context.Context, accountID int64, permissions []string, notificationType string) (bool, error) {
	return s.source.EmailSubscribed(ctx, accountID, permissions, notificationType)
}
func (s staffPreferences) SetEmailSubscribed(ctx context.Context, accountID int64, permissions []string, notificationType string, enabled bool) error {
	return s.source.SetEmailSubscribed(ctx, accountID, permissions, notificationType, enabled)
}
func (staffPreferences) IsUnknownType(err error) bool {
	return errors.Is(err, ErrUnknownNotificationType)
}
func (staffPreferences) IsPermissionRequired(err error) bool {
	return errors.Is(err, ErrEmailPermissionRequired)
}
