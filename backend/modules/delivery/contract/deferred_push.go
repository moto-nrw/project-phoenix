// Package contract publishes Delivery's native intent and provider values.
package contract

import (
	"encoding/json"
) // DeferredPushIntent persists a feature-owned push request before its audience
// is revalidated at dispatch time. Delivery owns the same lease and terminal
// states as device-snapshot intents; the root binds the feature dispatcher.
type DeferredPushIntent struct {
	TenantID       int64
	Template       string
	IdempotencyKey string
	Related        RelatedEntity
	Payload        json.RawMessage
}
