package devices

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/locales"
)

// ParentDeliveryPolicy exposes the canonical language, branding and notification
// rules without constructing another router or copying its policy.
type ParentDeliveryPolicy struct{}

func (ParentDeliveryPolicy) DefaultLocale() string               { return locales.Default() }
func (ParentDeliveryPolicy) NormalizeLocale(value string) string { return locales.Normalize(value) }
func (ParentDeliveryPolicy) MotoLogoURL(base string) string      { return emailbranding.MotoLogoURL(base) }
func (ParentDeliveryPolicy) SchoolLogoURL(base, value string) string {
	return emailbranding.SchoolLogoURL(base, value)
}
func (ParentDeliveryPolicy) ParentMessageCopy(locale string) (string, string) {
	return locales.ParentMessageCopy(locale)
}
func (ParentDeliveryPolicy) ParentAnnouncementCopy(locale, kind string) (string, string) {
	return locales.ParentAnnouncementCopy(locale, kind)
}
func (ParentDeliveryPolicy) IsNotificationsDisabled(err error) bool {
	return errors.Is(err, ErrNotificationsDisabled)
}
func (ParentDeliveryPolicy) IsOutsideActiveWindow(err error) bool {
	return errors.Is(err, ErrOutsideNotificationWindow)
}

// Canonical dispatch errors retain the identities used by the legacy router.
var (
	ErrNotificationsDisabled     = errors.New("notifications are disabled for this tenant")
	ErrOutsideNotificationWindow = errors.New("outside the tenant's notification window")
)
