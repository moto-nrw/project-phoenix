// Package subscriptiontest constructs native Delivery subscriptions for owner suites.
package subscriptiontest

import "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"

// NewSubscription returns a writable fixture with the supplied native fields.
func NewSubscription(value subscriptions.PushSubscription) *subscriptions.PushSubscription {
	return &value
}
