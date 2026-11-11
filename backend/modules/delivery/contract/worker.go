// Package contract publishes Delivery's native intent and provider values.
package contract

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type WebPushConfig struct {
	Subscriber string
	PublicKey  string
	PrivateKey string
}

func (c WebPushConfig) Configured() bool {
	return c.Subscriber != "" && c.PublicKey != "" && c.PrivateKey != ""
}

// WebPushProviderSubscriber adapts the RFC 8292 contact URI to webpush-go,
// which adds the mailto scheme itself unless the value starts with https.
// Passed unchanged, a mailto contact becomes "mailto:mailto:…", a JWT that
// Apple's push service rejects with 403.
func WebPushProviderSubscriber(contact string) string {
	return strings.TrimPrefix(contact, "mailto:")
}

// WebPushRecordSize is the aes128gcm record size of every Web Push request.
// webpush-go pads each message to the record size, so the request body is
// exactly this long. Mozilla delivers to Firefox on Android through FCM and
// rejects with 413 when the base64-encoded body, inside its FCM data map,
// exceeds 4096 bytes. The library default of 4096 failed every such push.
// One fixed size keeps all messages the same length on the wire.
const WebPushRecordSize uint32 = 2048

// WebPushMaxPayloadBytes is the largest payload one record holds: the record
// size minus the 86-byte header, the padding delimiter and the 16-byte tag.
const WebPushMaxPayloadBytes = int(WebPushRecordSize) - 103

type PushSender interface {
	SendPush(context.Context, ClaimedIntent) (ProviderResult, error)
}

type ProviderResult struct {
	MessageID string `json:"message_id,omitempty"`
	// ProviderMessageID is the ID the mail provider reported for an accepted mail.
	ProviderMessageID string          `json:"provider_message_id,omitempty"`
	UsedFallback      bool            `json:"used_fallback,omitempty"`
	StatusCode        int             `json:"status_code,omitempty"`
	Details           json.RawMessage `json:"details,omitempty"`
}

const ProviderAcceptedStatusCode = 202

type ClaimedIntent struct {
	ID       int64
	TenantID int64
	// OrganizationID is the Träger of an intent without a school (ADR 0110,
	// ADR 0126); zero for a school or system intent.
	OrganizationID    int64
	Transport         Transport
	Template          string
	DeliveryToken     string
	Security          bool
	UseFallback       bool
	RelatedEntityType *string
	RelatedEntityID   *int64
	EmailRecipient    EmailRecipient
	PushRecipient     PushRecipient
	EmailPayload      json.RawMessage
	PushPayload       PushPayload
	DeferredPayload   json.RawMessage
	Attempts          int
	LeaseToken        string
	LeaseExpiresAt    time.Time
}

type WorkerStats struct {
	Claimed      int
	Sent         int
	Cancelled    int
	Retried      int
	DeadLettered int
	LeaseLost    int
	// DeadLetters says why the deliveries counted in DeadLettered failed for
	// good: transport, template, delivery id and last error, or the expired
	// dispatches of a transport. ErrDeadLettered carries them to Sentry.
	DeadLetters []string
}

// ErrDeadLettered fails a run in which deliveries failed for good after all
// their attempts. That is the end of a delivery's retries, so the worker's
// caller reports it like any failed run (#3640).
var ErrDeadLettered = errors.New("delivery worker: deliveries failed for good after all attempts")
