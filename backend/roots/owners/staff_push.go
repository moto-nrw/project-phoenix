package owners

import (
	"context"
	"errors"
	"fmt"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
)

// StaffPush is one device notification to named staff accounts of the
// request's school. Its type must be a staff type of Delivery's catalogue
// that is on by default (DefaultOn): recipients who switched it off are
// dropped here, everyone else keeps it.
type StaffPush struct {
	TenantID       int64
	Type           string
	IdempotencyKey string
	RelatedType    string
	RelatedID      int64
	Title          string
	Body           string
	DeepLink       string
	AccountIDs     []int64
}

// StaffPushNotifier sends a StaffPush through the existing Delivery router
// in the caller's transaction (#6854): the push outbox entry commits or rolls
// back with the caller's write, and the worker delivers it with the outbox's
// retry and dead-letter rules. A school without notifications, or a time
// outside its notification window, is no error: the producer's in-app record
// is what carries the information, the push only points to it.
//
// It is the shared building block for staff notices produced in a request,
// such as a shared child note (#6854) or a changed pickup time (#6855).
type StaffPushNotifier struct {
	preferences staffPushPreferences
	router      staffPushRouter
}

// staffPushPreferences drops the recipients who switched a type off.
type staffPushPreferences interface {
	FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
}

// staffPushRouter is Delivery's notification router.
type staffPushRouter interface {
	Notify(ctx context.Context, event deliveryCompose.NotificationEvent) error
}

// NewStaffPushNotifier binds the notifier over the root's notifications.
func NewStaffPushNotifier(notifications DeliveryNotifications) StaffPushNotifier {
	notifier := StaffPushNotifier{}
	if notifications.Preferences != nil {
		notifier.preferences = notifications.Preferences
	}
	if notifications.Notifier != nil {
		notifier.router = notifications.Notifier
	}
	return notifier
}

// Notify filters out explicit declines and enqueues the push.
func (n StaffPushNotifier) Notify(ctx context.Context, push StaffPush) error {
	if n.router == nil || n.preferences == nil {
		return errors.New("staff push: notifications are not configured")
	}
	if len(push.AccountIDs) == 0 {
		return nil
	}
	recipients, err := n.preferences.FilterNotOptedOut(ctx, push.Type, push.AccountIDs)
	if err != nil {
		return fmt.Errorf("staff push: filter recipients: %w", err)
	}
	if len(recipients) == 0 {
		return nil
	}
	err = n.router.Notify(ctx, deliveryCompose.NotificationEvent{
		Type: push.Type, IdempotencyKey: push.IdempotencyKey, RelatedType: push.RelatedType, RelatedID: push.RelatedID,
		Title: push.Title, Body: push.Body, DeepLink: push.DeepLink,
		Audience: deliveryCompose.NotificationAudience{
			TenantID: push.TenantID, Scope: deliveryCompose.NotificationAudienceScope("staff"), StaffAccountIDs: recipients,
		},
	})
	if errors.Is(err, deliveryCompose.ErrNotificationsDisabled) || errors.Is(err, deliveryCompose.ErrOutsideNotificationWindow) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("staff push: %w", err)
	}
	return nil
}
