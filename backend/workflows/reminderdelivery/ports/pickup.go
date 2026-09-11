// Package ports defines the facts consumed by Reminder Delivery.
package ports

import "time"

// EffectivePickupTime is the resolved pickup clock for one student and day.
// Reminder evaluation does not consume schedule notes or override metadata.
type EffectivePickupTime struct {
	PickupTime *time.Time
}
