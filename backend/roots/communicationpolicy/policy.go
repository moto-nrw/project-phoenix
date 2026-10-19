// Package communicationpolicy binds real platform policy values for Communication's decision suites.
// It deliberately has no owner composition dependency, so an internal suite can use the typed root without a cycle.
package communicationpolicy

import (
	"io"

	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

func NewParentDeliveryPolicy() devices.ParentDeliveryPolicy { return devices.ParentDeliveryPolicy{} }
func NotificationsDisabled() error                          { return devices.ErrNotificationsDisabled }
func OutsideNotificationWindow() error                      { return devices.ErrOutsideNotificationWindow }
func Fingerprint(content []byte) string                     { return securityruntime.Fingerprint(content) }
func ReaderFingerprint(r io.Reader) (string, int64, error) {
	return securityruntime.ReaderFingerprint(r)
}
