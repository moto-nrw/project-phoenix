package platform

import (
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// deliveryRank orders provider outcomes on a single monotone scale. A stored
// status is only ever replaced by one of strictly higher rank, or by the same
// rank with a newer provider timestamp. That rule is what makes out-of-order
// webhook delivery safe without any ordering guarantee from the provider.
//
//	 0  unknown       no feedback yet
//	10  queued        the provider accepted our submission
//	20  deferred      transient 4xx; the provider is still retrying. Never
//	                  overwrites a terminal outcome that arrived first.
//	30  delivered     the recipient MTA accepted it. Terminal for "did it
//	                  arrive" — but not for the mailbox's health.
//	40  soft_bounced  the provider gave up after transient failures. Outranks
//	                  delivered because a later soft bounce for the same
//	                  message can only mean the delivered event described an
//	                  earlier hop.
//	50  suppressed    the provider never attempted it. Outranks soft_bounced:
//	                  a standing decision beats one failed attempt.
//	60  complained    the recipient marked it as spam. Outranks every
//	                  delivery-side outcome — it arrived AND was rejected, and
//	                  that is the fact operations must act on.
//	70  bounced       hard bounce. Highest rank because it is the only outcome
//	                  that invalidates the address itself.
var deliveryRank = map[string]int{
	platformModels.EmailDeliveryStatusUnknown:     0,
	platformModels.EmailDeliveryStatusQueued:      10,
	platformModels.EmailDeliveryStatusDeferred:    20,
	platformModels.EmailDeliveryStatusDelivered:   30,
	platformModels.EmailDeliveryStatusSoftBounced: 40,
	platformModels.EmailDeliveryStatusSuppressed:  50,
	platformModels.EmailDeliveryStatusComplained:  60,
	platformModels.EmailDeliveryStatusBounced:     70,
}

// DeliveryRank exposes the lattice position of a status. Unknown values rank
// lowest so a status we cannot interpret can never displace a real outcome.
func DeliveryRank(status string) int {
	return deliveryRank[status]
}

// statusForEvent maps a provider event onto the delivery-status vocabulary.
// A bounce without an explicit class is treated as hard: presenting a dead
// address as merely "vorübergehend" would leave staff waiting forever for a
// retry that will never succeed.
func statusForEvent(eventType string, bounceClass *string) string {
	switch eventType {
	case platformModels.DeliveryEventQueued:
		return platformModels.EmailDeliveryStatusQueued
	case platformModels.DeliveryEventDeferred:
		return platformModels.EmailDeliveryStatusDeferred
	case platformModels.DeliveryEventDelivered:
		return platformModels.EmailDeliveryStatusDelivered
	case platformModels.DeliveryEventComplained:
		return platformModels.EmailDeliveryStatusComplained
	case platformModels.DeliveryEventSuppressed:
		return platformModels.EmailDeliveryStatusSuppressed
	case platformModels.DeliveryEventBounced:
		if bounceClass != nil && *bounceClass == platformModels.BounceClassSoft {
			return platformModels.EmailDeliveryStatusSoftBounced
		}
		return platformModels.EmailDeliveryStatusBounced
	default:
		return ""
	}
}

// IsTerminalFailure reports whether a delivery status means the message will
// never reach this recipient without human intervention. Drives the ops
// notification and the "Adresse korrigieren" action in the UI.
func IsTerminalFailure(status string) bool {
	switch status {
	case platformModels.EmailDeliveryStatusBounced,
		platformModels.EmailDeliveryStatusComplained,
		platformModels.EmailDeliveryStatusSuppressed:
		return true
	default:
		return false
	}
}
